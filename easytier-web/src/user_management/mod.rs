//! Account policy owner. Writes serialize in the database and include their audit event.
use crate::db::{Db, entity::users, user_management as store};
use chrono::{DateTime, Duration, Utc};
use sea_orm::{
    ActiveModelTrait, ColumnTrait, ConnectionTrait, DatabaseTransaction, DbErr, EntityTrait,
    QueryFilter, Set, TransactionTrait,
};
use serde::Serialize;
pub use store::UserListFilter;

#[derive(Clone, Debug, Serialize)]
pub struct UserSummary {
    pub id: i32,
    pub username: String,
    pub role: String,
    pub web_login_enabled: bool,
    pub source: String,
    pub local_password_enabled: Option<bool>,
    pub must_change_password: bool,
    pub created_at: Option<DateTime<Utc>>,
    pub last_login_at: Option<DateTime<Utc>>,
}
impl From<store::UserRecord> for UserSummary {
    fn from(r: store::UserRecord) -> Self {
        Self {
            id: r.user.id,
            username: r.user.username,
            role: r.role,
            web_login_enabled: r.access.web_login_enabled,
            source: r.access.source,
            local_password_enabled: r.access.local_password_enabled,
            must_change_password: r.access.must_change_password,
            created_at: r.access.created_at,
            last_login_at: r.access.last_login_at,
        }
    }
}
#[derive(Serialize)]
pub struct Page<T> {
    pub items: Vec<T>,
    pub total: u64,
    pub page: u64,
    pub page_size: u64,
}
// Deliberately no Debug: delivery contains a one-time plaintext credential.
#[derive(Serialize)]
pub struct CredentialDelivery {
    pub user: UserSummary,
    pub temporary_password: String,
    pub expires_at: DateTime<Utc>,
}
#[derive(Serialize)]
pub struct UserEvent {
    pub id: i64,
    pub actor_id: Option<i32>,
    pub target_id: Option<i32>,
    pub action: String,
    pub occurred_at: DateTime<Utc>,
    pub result: String,
}
impl From<store::UserEvent> for UserEvent {
    fn from(e: store::UserEvent) -> Self {
        Self {
            id: e.id,
            actor_id: e.actor_id,
            target_id: e.target_id,
            action: e.action,
            occurred_at: e.occurred_at,
            result: e.result,
        }
    }
}
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
pub enum ManagementError {
    #[error("invalid_request")]
    InvalidRequest,
    #[error("unauthenticated")]
    Unauthenticated,
    #[error("forbidden")]
    Forbidden,
    #[error("password_change_required")]
    PasswordChangeRequired,
    #[error("self_action_forbidden")]
    SelfActionForbidden,
    #[error("local_password_unavailable")]
    LocalPasswordUnavailable,
    #[error("user_not_found")]
    UserNotFound,
    #[error("username_conflict")]
    UsernameConflict,
    #[error("last_admin")]
    LastAdmin,
    #[error("identity_conflict")]
    IdentityConflict,
    #[error("internal_error")]
    InternalError,
}
impl ManagementError {
    pub fn code(self) -> String {
        self.to_string()
    }
}
impl From<DbErr> for ManagementError {
    fn from(_: DbErr) -> Self {
        Self::InternalError
    }
}
type Result<T> = std::result::Result<T, ManagementError>;
#[derive(Clone)]
pub struct UserManagementService {
    db: Db,
}
enum Mutation<'a> {
    Create(&'a str),
    Access(bool),
    Role(&'a str),
    Reset,
    Revoke,
}
impl Mutation<'_> {
    fn action(&self) -> &'static str {
        match self {
            Self::Create(_) => "create",
            Self::Access(_) => "access",
            Self::Role(_) => "role",
            Self::Reset => "password_reset",
            Self::Revoke => "revoke_sessions",
        }
    }
}
struct Outcome {
    user: UserSummary,
    credential: Option<(String, DateTime<Utc>)>,
}
fn valid_username(value: &str) -> bool {
    (3..=64).contains(&value.len())
        && value
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || b"_.@-".contains(&b))
}
fn validate_page(page: u64, size: u64) -> Result<()> {
    if page == 0
        || !(1..=100).contains(&size)
        || page
            .checked_sub(1)
            .and_then(|p| p.checked_mul(size))
            .is_none_or(|v| v > i64::MAX as u64)
    {
        Err(ManagementError::InvalidRequest)
    } else {
        Ok(())
    }
}
fn increment(access: &mut store::UserAccess) -> Result<()> {
    access.auth_version = access
        .auth_version
        .checked_add(1)
        .ok_or(ManagementError::InternalError)?;
    Ok(())
}
fn temporary_credential() -> (String, String, DateTime<Utc>) {
    let bytes = rand::random::<[u8; 16]>();
    let password = bytes.iter().map(|b| format!("{b:02x}")).collect::<String>();
    let digest = format!("{:x}", md5::compute(password.as_bytes()));
    let hash = password_auth::generate_hash(digest);
    (password, hash, Utc::now() + Duration::hours(24))
}
async fn audit<C: ConnectionTrait>(
    conn: &C,
    actor: Option<i32>,
    target: Option<i32>,
    action: &str,
    result: &str,
) -> Result<()> {
    store::append_user_event(
        conn,
        store::NewUserEvent {
            actor_id: actor,
            target_id: target,
            action: action.into(),
            occurred_at: Utc::now(),
            result: result.into(),
        },
    )
    .await?;
    Ok(())
}
async fn authorize<C: ConnectionTrait>(conn: &C, actor: i32) -> Result<()> {
    let record = store::get_user_record(conn, actor)
        .await?
        .ok_or(ManagementError::Unauthenticated)?;
    if !record.access.web_login_enabled {
        return Err(ManagementError::Unauthenticated);
    }
    if record.access.must_change_password {
        return Err(ManagementError::PasswordChangeRequired);
    }
    if !store::has_manage_permission(conn, actor).await? {
        return Err(ManagementError::Forbidden);
    }
    Ok(())
}
impl UserManagementService {
    pub fn new(db: Db) -> Self {
        Self { db }
    }
    pub async fn list_users(
        &self,
        actor: i32,
        filter: UserListFilter,
    ) -> Result<Page<UserSummary>> {
        validate_page(filter.page, filter.page_size)?;
        if filter
            .role
            .as_deref()
            .is_some_and(|r| !matches!(r, "admin" | "user"))
        {
            return Err(ManagementError::InvalidRequest);
        }
        let tx = self.db.orm_db().begin().await?;
        authorize(&tx, actor).await?;
        let (page, page_size) = (filter.page, filter.page_size);
        let (items, total) = store::list_user_records(&tx, filter).await?;
        tx.commit().await?;
        Ok(Page {
            items: items.into_iter().map(Into::into).collect(),
            total,
            page,
            page_size,
        })
    }
    pub async fn get_user(&self, actor: i32, target: i32) -> Result<UserSummary> {
        let tx = self.db.orm_db().begin().await?;
        authorize(&tx, actor).await?;
        let user = store::get_user_record(&tx, target)
            .await?
            .ok_or(ManagementError::UserNotFound)?;
        tx.commit().await?;
        Ok(user.into())
    }
    pub async fn list_events(
        &self,
        actor: i32,
        target: Option<i32>,
        page: u64,
        page_size: u64,
    ) -> Result<Page<UserEvent>> {
        validate_page(page, page_size)?;
        let tx = self.db.orm_db().begin().await?;
        authorize(&tx, actor).await?;
        let (items, total) = store::list_user_events(&tx, target, page, page_size).await?;
        tx.commit().await?;
        Ok(Page {
            items: items.into_iter().map(Into::into).collect(),
            total,
            page,
            page_size,
        })
    }
    pub async fn create_user(&self, actor: i32, username: &str) -> Result<CredentialDelivery> {
        Self::delivery(self.mutate(actor, None, Mutation::Create(username)).await?)
    }
    pub async fn set_access(&self, actor: i32, target: i32, enabled: bool) -> Result<UserSummary> {
        Ok(self
            .mutate(actor, Some(target), Mutation::Access(enabled))
            .await?
            .user)
    }
    pub async fn set_role(&self, actor: i32, target: i32, role: &str) -> Result<UserSummary> {
        Ok(self
            .mutate(actor, Some(target), Mutation::Role(role))
            .await?
            .user)
    }
    pub async fn reset_password(&self, actor: i32, target: i32) -> Result<CredentialDelivery> {
        Self::delivery(self.mutate(actor, Some(target), Mutation::Reset).await?)
    }
    pub async fn revoke_sessions(&self, actor: i32, target: i32) -> Result<UserSummary> {
        Ok(self
            .mutate(actor, Some(target), Mutation::Revoke)
            .await?
            .user)
    }
    fn delivery(outcome: Outcome) -> Result<CredentialDelivery> {
        let (temporary_password, expires_at) =
            outcome.credential.ok_or(ManagementError::InternalError)?;
        Ok(CredentialDelivery {
            user: outcome.user,
            temporary_password,
            expires_at,
        })
    }
    async fn mutate(
        &self,
        actor: i32,
        target: Option<i32>,
        mutation: Mutation<'_>,
    ) -> Result<Outcome> {
        let action = mutation.action();
        let tx = self.db.orm_db().begin().await?;
        let result = async {
            store::lock_user_management(&tx).await?;
            authorize(&tx, actor).await?;
            self.apply(&tx, actor, target, mutation).await
        }
        .await;
        match result {
            Ok(value) => {
                tx.commit().await?;
                Ok(value)
            }
            Err(error) => {
                tx.rollback().await?;
                // A separate transaction records only verified IDs and a fixed action/result.
                let audit_tx = self.db.orm_db().begin().await?;
                store::lock_user_management(&audit_tx).await?;
                if store::get_user_record(&audit_tx, actor).await?.is_some() {
                    let verified_target = match target {
                        Some(id) if store::get_user_record(&audit_tx, id).await?.is_some() => {
                            Some(id)
                        }
                        _ => None,
                    };
                    audit(
                        &audit_tx,
                        Some(actor),
                        verified_target,
                        action,
                        if error == ManagementError::InternalError {
                            "failure"
                        } else {
                            "denied"
                        },
                    )
                    .await?;
                }
                audit_tx.commit().await?;
                Err(error)
            }
        }
    }
    async fn apply(
        &self,
        tx: &DatabaseTransaction,
        actor: i32,
        target: Option<i32>,
        mutation: Mutation<'_>,
    ) -> Result<Outcome> {
        let action = mutation.action();
        if let Mutation::Create(username) = mutation {
            if !valid_username(username) {
                return Err(ManagementError::InvalidRequest);
            }
            if users::Entity::find()
                .filter(users::Column::Username.eq(username))
                .one(tx)
                .await?
                .is_some()
            {
                return Err(ManagementError::UsernameConflict);
            }
            let (password, hash, expires) = temporary_credential();
            let user = users::ActiveModel {
                username: Set(username.into()),
                password: Set(hash),
                ..Default::default()
            }
            .insert(tx)
            .await
            .map_err(|e| {
                if e.sql_err()
                    .is_some_and(|e| matches!(e, sea_orm::SqlErr::UniqueConstraintViolation(_)))
                {
                    ManagementError::UsernameConflict
                } else {
                    ManagementError::InternalError
                }
            })?;
            store::set_user_role(tx, user.id, "user").await?;
            let mut access = store::UserAccess::legacy(user.id);
            access.source = "local".into();
            access.local_password_enabled = Some(true);
            access.created_at = Some(Utc::now());
            access.must_change_password = true;
            access.temporary_password_expires_at = Some(expires);
            store::save_user_access(tx, &access).await?;
            audit(tx, Some(actor), Some(user.id), action, "success").await?;
            return Ok(Outcome {
                user: store::get_user_record(tx, user.id)
                    .await?
                    .ok_or(ManagementError::InternalError)?
                    .into(),
                credential: Some((password, expires)),
            });
        }
        let target = target.ok_or(ManagementError::InvalidRequest)?;
        let mut record = store::get_user_record(tx, target)
            .await?
            .ok_or(ManagementError::UserNotFound)?;
        let mut credential = None;
        let changed = match mutation {
            Mutation::Access(enabled) => {
                if actor == target && !enabled {
                    return Err(ManagementError::SelfActionForbidden);
                }
                if record.access.web_login_enabled == enabled {
                    false
                } else {
                    if !enabled
                        && record.role == "admin"
                        && store::count_enabled_admins(tx).await? <= 1
                    {
                        return Err(ManagementError::LastAdmin);
                    }
                    record.access.web_login_enabled = enabled;
                    true
                }
            }
            Mutation::Role(role) => {
                if !matches!(role, "admin" | "user") {
                    return Err(ManagementError::InvalidRequest);
                }
                if actor == target && role == "user" {
                    return Err(ManagementError::SelfActionForbidden);
                }
                if record.role == role {
                    false
                } else {
                    if role == "user"
                        && record.access.web_login_enabled
                        && record.role == "admin"
                        && store::count_enabled_admins(tx).await? <= 1
                    {
                        return Err(ManagementError::LastAdmin);
                    }
                    store::set_user_role(tx, target, role).await?;
                    true
                }
            }
            Mutation::Reset => {
                if actor == target {
                    return Err(ManagementError::SelfActionForbidden);
                }
                if record.access.local_password_enabled != Some(true) {
                    return Err(ManagementError::LocalPasswordUnavailable);
                }
                let (password, hash, expires) = temporary_credential();
                let update = users::ActiveModel {
                    id: Set(target),
                    password: Set(hash),
                    ..Default::default()
                };
                update.update(tx).await?;
                record.access.must_change_password = true;
                record.access.temporary_password_expires_at = Some(expires);
                credential = Some((password, expires));
                true
            }
            Mutation::Revoke => true,
            Mutation::Create(_) => unreachable!(),
        };
        if changed {
            increment(&mut record.access)?;
            store::save_user_access(tx, &record.access).await?;
            audit(tx, Some(actor), Some(target), action, "success").await?;
        }
        Ok(Outcome {
            user: store::get_user_record(tx, target)
                .await?
                .ok_or(ManagementError::InternalError)?
                .into(),
            credential,
        })
    }
    /// Offline database-owner operation. Never exposed through the HTTP service.
    pub async fn recover_admin(&self, username: &str, password: &str) -> Result<UserSummary> {
        if !(12..=128).contains(&password.chars().count()) {
            return Err(ManagementError::InvalidRequest);
        }
        let tx = self.db.orm_db().begin().await?;
        store::lock_user_management(&tx).await?;
        let existing = users::Entity::find()
            .filter(users::Column::Username.eq(username))
            .one(&tx)
            .await?;
        let hash = password_auth::generate_hash(format!("{:x}", md5::compute(password.as_bytes())));
        let (user, action) = if let Some(user) = existing {
            let update = users::ActiveModel {
                id: Set(user.id),
                password: Set(hash),
                ..Default::default()
            };
            (update.update(&tx).await?, "recover")
        } else {
            if !valid_username(username) {
                return Err(ManagementError::InvalidRequest);
            }
            (
                users::ActiveModel {
                    username: Set(username.into()),
                    password: Set(hash),
                    ..Default::default()
                }
                .insert(&tx)
                .await?,
                "bootstrap",
            )
        };
        let mut access = store::ensure_user_access(&tx, user.id).await?;
        if action == "bootstrap" {
            access.source = "local".into();
            access.created_at = Some(Utc::now());
        }
        access.local_password_enabled = Some(true);
        access.web_login_enabled = true;
        access.must_change_password = false;
        access.temporary_password_expires_at = None;
        increment(&mut access)?;
        store::save_user_access(&tx, &access).await?;
        store::set_user_role(&tx, user.id, "admin").await?;
        audit(&tx, None, Some(user.id), action, "success").await?;
        let summary = store::get_user_record(&tx, user.id)
            .await?
            .ok_or(ManagementError::InternalError)?
            .into();
        tx.commit().await?;
        Ok(summary)
    }
    pub async fn bind_oidc_user(&self, username: &str, issuer: &str, subject: &str) -> Result<()> {
        if issuer.is_empty() || subject.is_empty() || url::Url::parse(issuer).is_err() {
            return Err(ManagementError::InvalidRequest);
        }
        let tx = self.db.orm_db().begin().await?;
        store::lock_user_management(&tx).await?;
        let user = users::Entity::find()
            .filter(users::Column::Username.eq(username))
            .one(&tx)
            .await?
            .ok_or(ManagementError::UserNotFound)?;
        if store::find_identity(&tx, issuer, subject)
            .await?
            .is_some_and(|id| id != user.id)
        {
            return Err(ManagementError::IdentityConflict);
        }
        store::bind_identity(&tx, user.id, issuer, subject).await?;
        audit(&tx, None, Some(user.id), "identity_bind", "success").await?;
        tx.commit().await?;
        Ok(())
    }
}
#[cfg(test)]
mod tests;

#[cfg(test)]
mod upgrade_tests;
