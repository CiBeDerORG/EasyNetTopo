use sea_orm_migration::prelude::*;

#[derive(DeriveMigrationName)]
pub struct Migration;

#[async_trait::async_trait]
impl MigrationTrait for Migration {
    async fn up(&self, manager: &SchemaManager) -> Result<(), DbErr> {
        // Account metadata is separate from the legacy users model. Do not guess
        // historical identity provenance or change existing credentials/access.
        manager.get_connection().execute_unprepared(r#"
            CREATE TABLE user_access (
                user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
                web_login_enabled BOOLEAN NOT NULL DEFAULT TRUE,
                auth_version BIGINT NOT NULL DEFAULT 0 CHECK(auth_version >= 0),
                must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
                source TEXT NOT NULL DEFAULT 'unknown' CHECK(source IN ('unknown','local','oidc','device')),
                local_password_enabled BOOLEAN,
                created_at TEXT, last_login_at TEXT, temporary_password_expires_at TEXT
            );
            INSERT INTO user_access(user_id) SELECT id FROM users;
            CREATE TABLE user_identities (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                issuer TEXT NOT NULL, subject TEXT NOT NULL,
                UNIQUE(issuer, subject)
            );
            CREATE INDEX idx_user_identities_user ON user_identities(user_id);
            CREATE TABLE user_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                actor_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
                target_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
                action TEXT NOT NULL CHECK(action IN ('create','access','role','password_reset','revoke_sessions','bootstrap','recover','identity_bind')),
                occurred_at TEXT NOT NULL,
                result TEXT NOT NULL CHECK(result IN ('success','denied','failure'))
            );
            CREATE INDEX idx_user_events_target ON user_events(target_id, id);
            CREATE TABLE user_management_lock (id INTEGER PRIMARY KEY CHECK(id = 1), revision BIGINT NOT NULL DEFAULT 0);
            INSERT INTO user_management_lock(id) VALUES (1);
            INSERT INTO permissions(name) SELECT 'users.manage'
                WHERE NOT EXISTS (SELECT 1 FROM permissions WHERE name = 'users.manage');
            INSERT INTO groups_permissions(group_id, permission_id)
                SELECT g.id, p.id FROM groups g, permissions p
                WHERE g.name = 'admins' AND p.name = 'users.manage'
                  AND NOT EXISTS (SELECT 1 FROM groups_permissions gp WHERE gp.group_id = g.id AND gp.permission_id = p.id);
        "#).await?;
        Ok(())
    }

    async fn down(&self, manager: &SchemaManager) -> Result<(), DbErr> {
        // Downgraded binaries cannot enforce this migration's account state.
        // Preserve existing group/permission data, including preexisting grants.
        manager
            .get_connection()
            .execute_unprepared(
                r#"
            DROP TABLE user_events;
            DROP TABLE user_identities;
            DROP TABLE user_access;
            DROP TABLE user_management_lock;
        "#,
            )
            .await?;
        Ok(())
    }
}
