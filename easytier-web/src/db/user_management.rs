//! Persistence primitives for account management. Callers own authorization and
//! business transactions. Records containing legacy users must never be serialized.
use chrono::{DateTime, Utc};
use sea_orm::{
    ConnectionTrait, DatabaseTransaction, DbErr, EntityTrait, FromQueryResult, Statement, Value,
};

use super::entity::users;

#[derive(Clone, Debug, PartialEq, Eq, FromQueryResult)]
pub struct UserAccess {
    pub user_id: i32,
    pub web_login_enabled: bool,
    pub auth_version: i64,
    pub must_change_password: bool,
    pub source: String,
    pub local_password_enabled: Option<bool>,
    pub created_at: Option<DateTime<Utc>>,
    pub last_login_at: Option<DateTime<Utc>>,
    pub temporary_password_expires_at: Option<DateTime<Utc>>,
}

impl UserAccess {
    pub fn legacy(user_id: i32) -> Self {
        Self {
            user_id,
            web_login_enabled: true,
            auth_version: 0,
            must_change_password: false,
            source: "unknown".into(),
            local_password_enabled: None,
            created_at: None,
            last_login_at: None,
            temporary_password_expires_at: None,
        }
    }
}

// Intentionally neither Debug nor Serialize: users::Model contains a password hash.
#[derive(Clone)]
pub struct UserRecord {
    pub user: users::Model,
    pub access: UserAccess,
    pub role: String,
}

#[derive(Clone, Debug)]
pub struct UserListFilter {
    pub search: Option<String>,
    pub role: Option<String>,
    pub web_login_enabled: Option<bool>,
    pub page: u64,
    pub page_size: u64,
}
impl Default for UserListFilter {
    fn default() -> Self {
        Self {
            search: None,
            role: None,
            web_login_enabled: None,
            page: 1,
            page_size: 20,
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, FromQueryResult)]
pub struct UserEvent {
    pub id: i64,
    pub actor_id: Option<i32>,
    pub target_id: Option<i32>,
    pub action: String,
    pub occurred_at: DateTime<Utc>,
    pub result: String,
}
#[derive(Clone, Debug)]
pub struct NewUserEvent {
    pub actor_id: Option<i32>,
    pub target_id: Option<i32>,
    pub action: String,
    pub occurred_at: DateTime<Utc>,
    pub result: String,
}

fn stmt<C: ConnectionTrait>(conn: &C, sql: &str, values: Vec<Value>) -> Statement {
    Statement::from_sql_and_values(conn.get_database_backend(), sql, values)
}

pub async fn get_user_record<C: ConnectionTrait>(
    conn: &C,
    user_id: i32,
) -> Result<Option<UserRecord>, DbErr> {
    let Some(user) = users::Entity::find_by_id(user_id).one(conn).await? else {
        return Ok(None);
    };
    let access = match conn
        .query_one(stmt(
            conn,
            "SELECT * FROM user_access WHERE user_id = ?",
            vec![user_id.into()],
        ))
        .await?
    {
        Some(row) => UserAccess::from_query_result(&row, "")?,
        None => UserAccess::legacy(user_id),
    };
    let row = conn.query_one(stmt(conn, "SELECT EXISTS(SELECT 1 FROM users_groups ug JOIN groups g ON g.id = ug.group_id WHERE ug.user_id = ? AND g.name = 'admins') AS is_admin", vec![user_id.into()])).await?.ok_or_else(|| DbErr::Custom("missing role result".into()))?;
    let role = if row.try_get::<bool>("", "is_admin")? {
        "admin"
    } else {
        "user"
    }
    .into();
    Ok(Some(UserRecord { user, access, role }))
}

pub async fn ensure_user_access<C: ConnectionTrait>(
    conn: &C,
    user_id: i32,
) -> Result<UserAccess, DbErr> {
    conn.execute(stmt(
        conn,
        "INSERT INTO user_access(user_id) VALUES (?) ON CONFLICT(user_id) DO NOTHING",
        vec![user_id.into()],
    ))
    .await?;
    let row = conn
        .query_one(stmt(
            conn,
            "SELECT * FROM user_access WHERE user_id = ?",
            vec![user_id.into()],
        ))
        .await?
        .ok_or_else(|| DbErr::Custom("missing account metadata".into()))?;
    UserAccess::from_query_result(&row, "")
}

/// Replace metadata inside the caller's locked transaction. Read the record
/// after acquiring the lock, including for login timestamps, so a concurrent
/// revocation/version update can never be overwritten by a stale snapshot.
pub async fn save_user_access<C: ConnectionTrait>(
    conn: &C,
    access: &UserAccess,
) -> Result<(), DbErr> {
    conn.execute(stmt(conn, r#"INSERT INTO user_access
        (user_id, web_login_enabled, auth_version, must_change_password, source, local_password_enabled, created_at, last_login_at, temporary_password_expires_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET
        web_login_enabled=excluded.web_login_enabled, auth_version=excluded.auth_version,
        must_change_password=excluded.must_change_password, source=excluded.source,
        local_password_enabled=excluded.local_password_enabled, created_at=excluded.created_at,
        last_login_at=excluded.last_login_at, temporary_password_expires_at=excluded.temporary_password_expires_at"#,
        vec![access.user_id.into(), access.web_login_enabled.into(), access.auth_version.into(),
            access.must_change_password.into(), access.source.clone().into(), access.local_password_enabled.into(),
            access.created_at.map(|v| v.to_rfc3339()).into(), access.last_login_at.map(|v| v.to_rfc3339()).into(),
            access.temporary_password_expires_at.map(|v| v.to_rfc3339()).into()])).await?;
    Ok(())
}

pub async fn set_user_role<C: ConnectionTrait>(
    conn: &C,
    user_id: i32,
    role: &str,
) -> Result<(), DbErr> {
    if !matches!(role, "admin" | "user") {
        return Err(DbErr::Custom("invalid role".into()));
    }
    // Membership changes must be part of the caller's locked write transaction.
    // Preserve custom groups. Every product account keeps ordinary user membership.
    let group = if role == "admin" { "admins" } else { "users" };
    conn.execute(stmt(conn, "INSERT INTO users_groups(user_id, group_id) SELECT ?, id FROM groups WHERE name = ? AND NOT EXISTS(SELECT 1 FROM users_groups ug WHERE ug.user_id = ? AND ug.group_id = groups.id)", vec![user_id.into(), group.into(), user_id.into()])).await?;
    if role == "admin" {
        conn.execute(stmt(conn, "INSERT INTO users_groups(user_id, group_id) SELECT ?, id FROM groups WHERE name = 'users' AND NOT EXISTS(SELECT 1 FROM users_groups ug WHERE ug.user_id = ? AND ug.group_id = groups.id)", vec![user_id.into(), user_id.into()])).await?;
    } else {
        conn.execute(stmt(conn, "DELETE FROM users_groups WHERE user_id = ? AND group_id IN (SELECT id FROM groups WHERE name = 'admins')", vec![user_id.into()])).await?;
    }
    Ok(())
}

pub async fn count_enabled_admins<C: ConnectionTrait>(conn: &C) -> Result<u64, DbErr> {
    count(conn, "SELECT COUNT(*) AS count FROM users u LEFT JOIN user_access a ON a.user_id=u.id WHERE COALESCE(a.web_login_enabled, TRUE) AND EXISTS(SELECT 1 FROM users_groups ug JOIN groups g ON g.id=ug.group_id WHERE ug.user_id=u.id AND g.name='admins')", vec![]).await
}

/// Must be the first statement after begin(), before any authorization reads.
/// SQLite serializes writers across processes; a deferred read upgraded later
/// can instead fail with SQLITE_BUSY_SNAPSHOT. Callers propagate/retry busy errors.
pub async fn lock_user_management(txn: &DatabaseTransaction) -> Result<(), DbErr> {
    let result = txn
        .execute(stmt(
            txn,
            "UPDATE user_management_lock SET revision = revision WHERE id = 1",
            vec![],
        ))
        .await?;
    if result.rows_affected() != 1 {
        return Err(DbErr::Custom("account write lock missing".into()));
    }
    Ok(())
}

pub async fn has_manage_permission<C: ConnectionTrait>(
    conn: &C,
    user_id: i32,
) -> Result<bool, DbErr> {
    Ok(count(conn, "SELECT COUNT(*) AS count FROM users_groups ug JOIN groups_permissions gp ON gp.group_id=ug.group_id JOIN permissions p ON p.id=gp.permission_id WHERE ug.user_id=? AND p.name='users.manage'", vec![user_id.into()]).await? > 0)
}

pub async fn append_user_event<C: ConnectionTrait>(
    conn: &C,
    event: NewUserEvent,
) -> Result<(), DbErr> {
    conn.execute(stmt(
        conn,
        "INSERT INTO user_events(actor_id,target_id,action,occurred_at,result) VALUES (?,?,?,?,?)",
        vec![
            event.actor_id.into(),
            event.target_id.into(),
            event.action.into(),
            event.occurred_at.to_rfc3339().into(),
            event.result.into(),
        ],
    ))
    .await?;
    Ok(())
}

pub async fn find_identity<C: ConnectionTrait>(
    conn: &C,
    issuer: &str,
    subject: &str,
) -> Result<Option<i32>, DbErr> {
    conn.query_one(stmt(
        conn,
        "SELECT user_id FROM user_identities WHERE issuer=? AND subject=?",
        vec![issuer.into(), subject.into()],
    ))
    .await?
    .map(|row| row.try_get("", "user_id"))
    .transpose()
}

pub async fn bind_identity<C: ConnectionTrait>(
    conn: &C,
    user_id: i32,
    issuer: &str,
    subject: &str,
) -> Result<(), DbErr> {
    // Repeat binding to the same account is idempotent; never reassign identity.
    conn.execute(stmt(conn, "INSERT INTO user_identities(user_id,issuer,subject) VALUES (?,?,?) ON CONFLICT(issuer,subject) DO NOTHING", vec![user_id.into(), issuer.into(), subject.into()])).await?;
    if find_identity(conn, issuer, subject).await? != Some(user_id) {
        return Err(DbErr::Custom("identity_conflict".into()));
    }
    Ok(())
}

fn pagination(page: u64, page_size: u64) -> Result<i64, DbErr> {
    if page == 0 || !(1..=100).contains(&page_size) {
        return Err(DbErr::Custom("invalid pagination".into()));
    }
    page.checked_sub(1)
        .and_then(|p| p.checked_mul(page_size))
        .and_then(|v| i64::try_from(v).ok())
        .ok_or_else(|| DbErr::Custom("pagination overflow".into()))
}
async fn count<C: ConnectionTrait>(conn: &C, sql: &str, values: Vec<Value>) -> Result<u64, DbErr> {
    let row = conn
        .query_one(stmt(conn, sql, values))
        .await?
        .ok_or_else(|| DbErr::Custom("missing count".into()))?;
    let count: i64 = row.try_get("", "count")?;
    u64::try_from(count).map_err(|_| DbErr::Custom("invalid count".into()))
}

pub async fn list_user_records<C: ConnectionTrait>(
    conn: &C,
    filter: UserListFilter,
) -> Result<(Vec<UserRecord>, u64), DbErr> {
    let offset = pagination(filter.page, filter.page_size)?;
    let mut condition =
        " FROM users u LEFT JOIN user_access a ON a.user_id=u.id WHERE 1=1".to_owned();
    let mut values = Vec::new();
    if let Some(search) = filter.search.filter(|s| !s.is_empty()) {
        condition.push_str(" AND u.username LIKE ? ESCAPE '\\'");
        values.push(
            format!(
                "%{}%",
                search
                    .replace('\\', "\\\\")
                    .replace('%', "\\%")
                    .replace('_', "\\_")
            )
            .into(),
        );
    }
    if let Some(role) = filter.role {
        if !matches!(role.as_str(), "admin" | "user") {
            return Err(DbErr::Custom("invalid role".into()));
        }
        condition.push_str(if role == "admin" {
            " AND EXISTS"
        } else {
            " AND NOT EXISTS"
        });
        condition.push_str("(SELECT 1 FROM users_groups ug JOIN groups g ON g.id=ug.group_id WHERE ug.user_id=u.id AND g.name='admins')");
    }
    if let Some(enabled) = filter.web_login_enabled {
        condition.push_str(" AND COALESCE(a.web_login_enabled, TRUE) = ?");
        values.push(enabled.into());
    }
    let total = count(
        conn,
        &format!("SELECT COUNT(*) AS count{condition}"),
        values.clone(),
    )
    .await?;
    values.push((filter.page_size as i64).into());
    values.push(offset.into());
    let rows = conn
        .query_all(stmt(
            conn,
            &format!("SELECT u.id{condition} ORDER BY u.id ASC LIMIT ? OFFSET ?"),
            values,
        ))
        .await?;
    let mut records = Vec::with_capacity(rows.len());
    for row in rows {
        if let Some(record) = get_user_record(conn, row.try_get("", "id")?).await? {
            records.push(record);
        }
    }
    Ok((records, total))
}

pub async fn list_user_events<C: ConnectionTrait>(
    conn: &C,
    target_id: Option<i32>,
    page: u64,
    page_size: u64,
) -> Result<(Vec<UserEvent>, u64), DbErr> {
    let offset = pagination(page, page_size)?;
    let condition = if target_id.is_some() {
        " WHERE target_id=?"
    } else {
        ""
    };
    let mut values: Vec<Value> = target_id.into_iter().map(Into::into).collect();
    let total = count(
        conn,
        &format!("SELECT COUNT(*) AS count FROM user_events{condition}"),
        values.clone(),
    )
    .await?;
    values.push((page_size as i64).into());
    values.push(offset.into());
    let rows = conn
        .query_all(stmt(
            conn,
            &format!("SELECT * FROM user_events{condition} ORDER BY id DESC LIMIT ? OFFSET ?"),
            values,
        ))
        .await?;
    Ok((
        rows.iter()
            .map(|row| UserEvent::from_query_result(row, ""))
            .collect::<Result<_, _>>()?,
        total,
    ))
}

#[cfg(test)]
mod tests;
