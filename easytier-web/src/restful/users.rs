use std::collections::HashSet;

use axum_login::{AuthUser, AuthnBackend, AuthzBackend, UserId};
use password_auth::verify_password;
use sea_orm::{
    ColumnTrait, EntityTrait, FromQueryResult, IntoActiveModel, JoinType, QueryFilter,
    QuerySelect as _, RelationTrait, Set, TransactionTrait,
};
use serde::{Deserialize, Serialize};
use tokio::task;

use crate::db::{self, entity, user_management as accounts};
use chrono::Utc;
use sha2::{Digest, Sha256};

#[derive(Clone)]
pub struct User {
    pub(crate) db_user: entity::users::Model,
    pub tokens: Vec<String>,
    pub(crate) access: accounts::UserAccess,
    session_digest: Vec<u8>,
}

// Here we've implemented `Debug` manually to avoid accidentally logging the
// password hash.
impl std::fmt::Debug for User {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("User")
            .field("id", &self.db_user.id)
            .field("username", &self.db_user.username)
            .field("password", &"[redacted]")
            .finish()
    }
}

impl AuthUser for User {
    type Id = i32;

    fn id(&self) -> Self::Id {
        self.db_user.id
    }

    fn session_auth_hash(&self) -> &[u8] {
        &self.session_digest
    }
}

impl User {
    pub(crate) fn from_record(record: accounts::UserRecord) -> Self {
        let mut hash = Sha256::new();
        hash.update(record.user.password.as_bytes());
        hash.update([0]);
        hash.update(record.access.auth_version.to_be_bytes());
        Self {
            tokens: vec![record.user.username.clone()],
            db_user: record.user,
            access: record.access,
            session_digest: hash.finalize().to_vec(),
        }
    }

    pub(crate) fn is_restricted(&self) -> bool {
        self.access.must_change_password
    }
}

fn access_allowed(access: &accounts::UserAccess) -> bool {
    access.web_login_enabled
        && (!access.must_change_password
            || access
                .temporary_password_expires_at
                .is_some_and(|expiry| expiry > Utc::now()))
}

fn valid_digest(value: &str) -> bool {
    value.len() == 32 && value.bytes().all(|b| b.is_ascii_hexdigit())
}

// This allows us to extract the authentication fields from forms. We use this
// to authenticate requests with the backend.
#[derive(Clone, Deserialize, Serialize)]
pub struct Credentials {
    pub username: String,
    pub password: String,
}

#[derive(Clone, Deserialize, Serialize)]
pub struct RegisterNewUser {
    pub credentials: Credentials,
    pub captcha: String,
}

#[derive(Clone, Deserialize, Serialize)]
pub struct ChangePassword {
    pub new_password: String,
}

#[derive(Debug, Clone)]
pub struct Backend {
    db: db::Db,
}

impl Backend {
    pub fn new(db: db::Db) -> Self {
        Self { db }
    }

    pub fn db(&self) -> &db::Db {
        &self.db
    }

    pub async fn register_new_user(&self, new_user: &RegisterNewUser) -> Result<(), Error> {
        if !valid_digest(&new_user.credentials.password) {
            return Err(Error::InvalidRequest);
        }
        let password = new_user.credentials.password.clone();
        let hashed = task::spawn_blocking(move || password_auth::generate_hash(password)).await?;
        let tx = self.db.orm_db().begin().await?;
        accounts::lock_user_management(&tx).await?;
        let result = entity::users::Entity::insert(entity::users::ActiveModel {
            username: Set(new_user.credentials.username.clone()),
            password: Set(hashed),
            ..Default::default()
        })
        .exec(&tx)
        .await?;
        accounts::set_user_role(&tx, result.last_insert_id, "user").await?;
        let mut access = accounts::ensure_user_access(&tx, result.last_insert_id).await?;
        access.source = "local".into();
        access.local_password_enabled = Some(true);
        access.created_at = Some(Utc::now());
        accounts::save_user_access(&tx, &access).await?;
        tx.commit().await?;
        Ok(())
    }

    /// Only verified issuer/subject claims may enter this function. A display
    /// username never authorizes linking an existing account.
    pub async fn find_or_create_oidc_user(
        &self,
        username: &str,
        issuer: &str,
        subject: &str,
    ) -> Result<User, Error> {
        if username.is_empty() || issuer.is_empty() || subject.is_empty() {
            return Err(Error::InvalidRequest);
        }
        let tx = self.db.orm_db().begin().await?;
        accounts::lock_user_management(&tx).await?;
        let id = if let Some(id) = accounts::find_identity(&tx, issuer, subject).await? {
            id
        } else {
            if entity::users::Entity::find()
                .filter(entity::users::Column::Username.eq(username))
                .one(&tx)
                .await?
                .is_some()
            {
                return Err(Error::IdentityConflict);
            }
            // No usable local credential is provisioned for external identities.
            let result = entity::users::Entity::insert(entity::users::ActiveModel {
                username: Set(username.to_owned()),
                password: Set("!oidc-no-local-password".into()),
                ..Default::default()
            })
            .exec(&tx)
            .await?;
            let id = result.last_insert_id;
            accounts::set_user_role(&tx, id, "user").await?;
            let mut access = accounts::ensure_user_access(&tx, id).await?;
            access.source = "oidc".into();
            access.local_password_enabled = Some(false);
            access.created_at = Some(Utc::now());
            accounts::save_user_access(&tx, &access).await?;
            accounts::bind_identity(&tx, id, issuer, subject).await?;
            id
        };
        let mut record = accounts::get_user_record(&tx, id)
            .await?
            .ok_or(Error::Unavailable)?;
        if !access_allowed(&record.access) {
            return Err(Error::Unavailable);
        }
        record.access.last_login_at = Some(Utc::now());
        accounts::save_user_access(&tx, &record.access).await?;
        tx.commit().await?;
        Ok(User::from_record(record))
    }

    pub async fn change_password(&self, id: i32, req: &ChangePassword) -> Result<(), Error> {
        if !valid_digest(&req.new_password) {
            return Err(Error::InvalidRequest);
        }
        let password = req.new_password.clone();
        let hashed = task::spawn_blocking(move || password_auth::generate_hash(password)).await?;
        let tx = self.db.orm_db().begin().await?;
        accounts::lock_user_management(&tx).await?;
        let mut record = accounts::get_user_record(&tx, id)
            .await?
            .ok_or(Error::Unavailable)?;
        if !access_allowed(&record.access) {
            return Err(Error::Unavailable);
        }
        if record.access.local_password_enabled != Some(true) {
            return Err(Error::LocalPasswordUnavailable);
        }
        if record.access.must_change_password
            && verify_password(&req.new_password, &record.user.password).is_ok()
        {
            return Err(Error::InvalidRequest);
        }
        let mut user = record.user.into_active_model();
        user.password = Set(hashed);
        entity::users::Entity::update(user).exec(&tx).await?;
        record.access.auth_version = record
            .access
            .auth_version
            .checked_add(1)
            .ok_or(Error::Unavailable)?;
        record.access.must_change_password = false;
        record.access.temporary_password_expires_at = None;
        accounts::save_user_access(&tx, &record.access).await?;
        tx.commit().await?;
        Ok(())
    }
}

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("invalid_request")]
    InvalidRequest,
    #[error("unauthenticated")]
    Unavailable,
    #[error("identity_conflict")]
    IdentityConflict,
    #[error("local_password_unavailable")]
    LocalPasswordUnavailable,
    #[error(transparent)]
    Sqlx(#[from] sea_orm::DbErr),

    #[error(transparent)]
    TaskJoin(#[from] task::JoinError),
}

impl AuthnBackend for Backend {
    type User = User;
    type Credentials = Credentials;
    type Error = Error;

    async fn authenticate(
        &self,
        creds: Self::Credentials,
    ) -> Result<Option<Self::User>, Self::Error> {
        let Some(candidate) = entity::users::Entity::find()
            .filter(entity::users::Column::Username.eq(&creds.username))
            .one(self.db.orm_db())
            .await?
        else {
            return Ok(None);
        };
        let checked_hash = candidate.password.clone();
        let input = creds.password;
        let hash = checked_hash.clone();
        if !task::spawn_blocking(move || verify_password(input, &hash).is_ok()).await? {
            return Ok(None);
        }
        // Re-read under the same DB lock used by management writes, so a concurrent
        // reset/disable cannot be undone by last-login metadata updates.
        let tx = self.db.orm_db().begin().await?;
        accounts::lock_user_management(&tx).await?;
        let Some(mut record) = accounts::get_user_record(&tx, candidate.id).await? else {
            return Ok(None);
        };
        if record.user.password != checked_hash
            || !access_allowed(&record.access)
            || record.access.local_password_enabled == Some(false)
        {
            return Ok(None);
        }
        record.access.local_password_enabled = Some(true);
        record.access.last_login_at = Some(Utc::now());
        accounts::save_user_access(&tx, &record.access).await?;
        tx.commit().await?;
        Ok(Some(User::from_record(record)))
    }

    async fn get_user(&self, user_id: &UserId<Self>) -> Result<Option<Self::User>, Self::Error> {
        Ok(accounts::get_user_record(self.db.orm_db(), *user_id)
            .await?
            .filter(|record| access_allowed(&record.access))
            .map(User::from_record))
    }
}

#[derive(Debug, Clone, Eq, PartialEq, Hash, FromQueryResult)]
pub struct Permission {
    pub name: String,
}

impl From<&str> for Permission {
    fn from(name: &str) -> Self {
        Permission {
            name: name.to_string(),
        }
    }
}

impl AuthzBackend for Backend {
    type Permission = Permission;

    async fn get_group_permissions(
        &self,
        user: &Self::User,
    ) -> Result<HashSet<Self::Permission>, Self::Error> {
        let permissions = entity::users::Entity::find()
            .select_only()
            .filter(entity::users::Column::Id.eq(user.id()))
            .column_as(entity::permissions::Column::Name, "name")
            .join(
                JoinType::InnerJoin,
                entity::users::Relation::UsersGroups.def(),
            )
            .join(
                JoinType::InnerJoin,
                entity::users_groups::Relation::Groups.def(),
            )
            .join(
                JoinType::InnerJoin,
                entity::groups::Relation::GroupsPermissions.def(),
            )
            .join(
                JoinType::InnerJoin,
                entity::groups_permissions::Relation::Permissions.def(),
            )
            .into_model::<Self::Permission>()
            .all(self.db.orm_db())
            .await?;

        Ok(permissions.into_iter().collect())
    }
}

// We use a type alias for convenience.
//
// Note that we've supplied our concrete backend here.
pub type AuthSession = axum_login::AuthSession<Backend>;

#[cfg(test)]
mod tests {
    use super::*;

    /// Exercise the authentication backend against a real, migrated SQLite database.
    /// The fixture uses no usable passwords and never starts a network or VPN service.
    #[tokio::test]
    async fn group_permissions_are_scoped_to_user() {
        let db = db::Db::memory_db().await;
        let pool = db.inner();
        for name in ["wum-readers", "wum-admins", "wum-auditors", "wum-empty"] {
            sqlx::query("INSERT INTO groups (name) VALUES (?)")
                .bind(name)
                .execute(&pool)
                .await
                .unwrap();
        }
        for name in ["wum.read", "wum.manage", "wum.audit"] {
            sqlx::query("INSERT INTO permissions (name) VALUES (?)")
                .bind(name)
                .execute(&pool)
                .await
                .unwrap();
        }
        for (group, permission) in [
            ("wum-readers", "wum.read"),
            ("wum-admins", "wum.manage"),
            ("wum-auditors", "wum.audit"),
            // Shared permission must remain a set when a user belongs to both groups.
            ("wum-auditors", "wum.read"),
        ] {
            sqlx::query(
                "INSERT INTO groups_permissions (group_id, permission_id)
                 SELECT groups.id, permissions.id FROM groups, permissions
                 WHERE groups.name = ? AND permissions.name = ?",
            )
            .bind(group)
            .bind(permission)
            .execute(&pool)
            .await
            .unwrap();
        }
        let cases: &[(&str, &[&str], &[&str])] = &[
            ("wum-ordinary", &["wum-readers"], &["wum.read"]),
            ("wum-administrator", &["wum-admins"], &["wum.manage"]),
            ("wum-ungrouped", &[], &[]),
            ("wum-no-permissions", &["wum-empty"], &[]),
            (
                "wum-multiple",
                &["wum-readers", "wum-auditors", "wum-empty"],
                &["wum.read", "wum.audit"],
            ),
        ];
        for (username, groups, _) in cases {
            sqlx::query("INSERT INTO users (username, password) VALUES (?, ?)")
                .bind(*username)
                .bind("unusable-test-password")
                .execute(&pool)
                .await
                .unwrap();
            for group in *groups {
                sqlx::query(
                    "INSERT INTO users_groups (user_id, group_id)
                     SELECT users.id, groups.id FROM users, groups
                     WHERE users.username = ? AND groups.name = ?",
                )
                .bind(*username)
                .bind(group)
                .execute(&pool)
                .await
                .unwrap();
            }
        }
        let backend = Backend::new(db);
        for (username, _, expected) in cases {
            let db_user = entity::users::Entity::find()
                .filter(entity::users::Column::Username.eq(*username))
                .one(backend.db().orm_db())
                .await
                .unwrap()
                .unwrap();
            let user = User::from_record(
                accounts::get_user_record(backend.db().orm_db(), db_user.id)
                    .await
                    .unwrap()
                    .unwrap(),
            );
            let actual = backend.get_group_permissions(&user).await.unwrap();
            let expected: HashSet<Permission> = expected.iter().copied().map(Into::into).collect();
            assert_eq!(actual, expected, "permissions for {username}");
        }
    }
}

#[cfg(test)]
mod lifecycle_tests {
    use super::*;

    async fn local(backend: &Backend, name: &str) -> User {
        backend
            .register_new_user(&RegisterNewUser {
                credentials: Credentials {
                    username: name.into(),
                    password: format!("{:x}", md5::compute("fictional-initial-password")),
                },
                captcha: "test".into(),
            })
            .await
            .unwrap();
        backend
            .authenticate(Credentials {
                username: name.into(),
                password: format!("{:x}", md5::compute("fictional-initial-password")),
            })
            .await
            .unwrap()
            .unwrap()
    }

    #[tokio::test]
    async fn auth_lifecycle_versions_disabled_restore_and_temporary_credentials() {
        let backend = Backend::new(db::Db::memory_db().await);
        let user = local(&backend, "auth-lifecycle").await;
        let old_digest = user.session_auth_hash().to_vec();
        let tx = backend.db.orm_db().begin().await.unwrap();
        accounts::lock_user_management(&tx).await.unwrap();
        let mut access = accounts::ensure_user_access(&tx, user.id()).await.unwrap();
        access.web_login_enabled = false;
        access.auth_version += 1;
        accounts::save_user_access(&tx, &access).await.unwrap();
        tx.commit().await.unwrap();
        assert!(backend.get_user(&user.id()).await.unwrap().is_none());
        assert!(
            backend
                .authenticate(Credentials {
                    username: "auth-lifecycle".into(),
                    password: format!("{:x}", md5::compute("fictional-initial-password"))
                })
                .await
                .unwrap()
                .is_none()
        );
        access.web_login_enabled = true;
        access.auth_version += 1;
        let tx = backend.db.orm_db().begin().await.unwrap();
        accounts::lock_user_management(&tx).await.unwrap();
        accounts::save_user_access(&tx, &access).await.unwrap();
        tx.commit().await.unwrap();
        assert_ne!(
            backend
                .get_user(&user.id())
                .await
                .unwrap()
                .unwrap()
                .session_auth_hash(),
            old_digest
        );
        access.must_change_password = true;
        access.temporary_password_expires_at = Some(Utc::now() + chrono::Duration::hours(1));
        let tx = backend.db.orm_db().begin().await.unwrap();
        accounts::lock_user_management(&tx).await.unwrap();
        accounts::save_user_access(&tx, &access).await.unwrap();
        tx.commit().await.unwrap();
        assert!(
            backend
                .get_user(&user.id())
                .await
                .unwrap()
                .unwrap()
                .is_restricted()
        );
        assert!(matches!(
            backend
                .change_password(
                    user.id(),
                    &ChangePassword {
                        new_password: format!("{:x}", md5::compute("fictional-initial-password"))
                    }
                )
                .await,
            Err(Error::InvalidRequest)
        ));
        backend
            .change_password(
                user.id(),
                &ChangePassword {
                    new_password: format!("{:x}", md5::compute("fictional-replacement-password")),
                },
            )
            .await
            .unwrap();
        let reloaded = backend.get_user(&user.id()).await.unwrap().unwrap();
        assert!(!reloaded.is_restricted());
        assert!(reloaded.access.temporary_password_expires_at.is_none());
        assert_eq!(reloaded.access.auth_version, 3);
        assert!(
            backend
                .authenticate(Credentials {
                    username: "auth-lifecycle".into(),
                    password: format!("{:x}", md5::compute("fictional-initial-password"))
                })
                .await
                .unwrap()
                .is_none()
        );
        access = reloaded.access;
        access.must_change_password = true;
        access.temporary_password_expires_at = Some(Utc::now() - chrono::Duration::seconds(1));
        let tx = backend.db.orm_db().begin().await.unwrap();
        accounts::lock_user_management(&tx).await.unwrap();
        accounts::save_user_access(&tx, &access).await.unwrap();
        tx.commit().await.unwrap();
        assert!(backend.get_user(&user.id()).await.unwrap().is_none());
    }

    #[tokio::test]
    async fn auth_lifecycle_oidc_binding_conflict_and_disabled_identity() {
        let backend = Backend::new(db::Db::memory_db().await);
        let local_user = local(&backend, "oidc-name-conflict").await;
        assert!(matches!(
            backend
                .find_or_create_oidc_user("oidc-name-conflict", "https://test.invalid", "sub1")
                .await,
            Err(Error::IdentityConflict)
        ));
        let external = backend
            .find_or_create_oidc_user("oidc-new-test", "https://test.invalid", "sub2")
            .await
            .unwrap();
        assert_eq!(external.access.local_password_enabled, Some(false));
        assert_eq!(external.access.source, "oidc");
        assert!(matches!(
            backend
                .change_password(
                    external.id(),
                    &ChangePassword {
                        new_password: "a".repeat(32)
                    }
                )
                .await,
            Err(Error::LocalPasswordUnavailable)
        ));
        assert_eq!(
            backend
                .find_or_create_oidc_user("changed-display-name", "https://test.invalid", "sub2")
                .await
                .unwrap()
                .id(),
            external.id()
        );
        let tx = backend.db.orm_db().begin().await.unwrap();
        accounts::lock_user_management(&tx).await.unwrap();
        let mut access = external.access;
        access.web_login_enabled = false;
        access.auth_version += 1;
        accounts::save_user_access(&tx, &access).await.unwrap();
        tx.commit().await.unwrap();
        assert!(matches!(
            backend
                .find_or_create_oidc_user("oidc-new-test", "https://test.invalid", "sub2")
                .await,
            Err(Error::Unavailable)
        ));
        assert_ne!(local_user.id(), external.db_user.id);
    }

    #[tokio::test]
    async fn auth_lifecycle_legacy_login_confirms_capability_without_guessing_source() {
        let db = db::Db::memory_db().await;
        let row = db
            .create_user_and_join_users_group(
                "legacy-auth",
                password_auth::generate_hash("legacy-digest-input"),
            )
            .await
            .unwrap();
        let backend = Backend::new(db);
        assert_eq!(
            accounts::get_user_record(backend.db.orm_db(), row.id)
                .await
                .unwrap()
                .unwrap()
                .access
                .local_password_enabled,
            None
        );
        let user = backend
            .authenticate(Credentials {
                username: "legacy-auth".into(),
                password: "legacy-digest-input".into(),
            })
            .await
            .unwrap()
            .unwrap();
        assert_eq!(user.access.source, "unknown");
        assert_eq!(user.access.local_password_enabled, Some(true));
        assert!(user.access.created_at.is_none());
        assert!(user.access.last_login_at.is_some());
    }
}
