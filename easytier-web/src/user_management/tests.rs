use super::*;

const TEST_PASSWORD: &str = "fictional-test-password-123";
async fn fixture() -> (UserManagementService, i32) {
    let service = UserManagementService::new(Db::memory_db().await);
    let admin = service
        .recover_admin("wum-operator", TEST_PASSWORD)
        .await
        .unwrap();
    (service, admin.id)
}
async fn record(service: &UserManagementService, id: i32) -> store::UserRecord {
    store::get_user_record(service.db.orm_db(), id)
        .await
        .unwrap()
        .unwrap()
}
#[tokio::test]
async fn account_service_lifecycle_and_audit() {
    let (service, actor) = fixture().await;
    let created = service.create_user(actor, "wum-new-user").await.unwrap();
    let id = created.user.id;
    assert_eq!(created.temporary_password.len(), 32);
    assert!(
        created
            .temporary_password
            .bytes()
            .all(|b| b.is_ascii_hexdigit())
    );
    assert!((created.expires_at - Utc::now()).num_seconds() > 86_300);
    assert!((created.expires_at - Utc::now()).num_seconds() <= 86_400);
    assert!(created.user.must_change_password);
    assert_eq!(created.user.role, "user");
    assert!(
        password_auth::verify_password(
            format!("{:x}", md5::compute(&created.temporary_password)),
            &record(&service, id).await.user.password
        )
        .is_ok()
    );
    assert!(matches!(
        service.create_user(actor, "wum-new-user").await,
        Err(ManagementError::UsernameConflict)
    ));
    let first = record(&service, id).await;
    service.revoke_sessions(actor, id).await.unwrap();
    let revoked = record(&service, id).await;
    assert_eq!(first.user, revoked.user);
    assert_eq!(first.access.auth_version + 1, revoked.access.auth_version);
    assert_eq!(
        first.access.web_login_enabled,
        revoked.access.web_login_enabled
    );
    assert_eq!(first.role, revoked.role);
    service.set_access(actor, id, false).await.unwrap();
    let disabled = record(&service, id).await;
    assert!(!disabled.access.web_login_enabled);
    service.set_access(actor, id, false).await.unwrap();
    assert_eq!(
        record(&service, id).await.access.auth_version,
        disabled.access.auth_version
    );
    service.set_access(actor, id, true).await.unwrap();
    assert!(record(&service, id).await.access.auth_version > disabled.access.auth_version);
    let reset = service.reset_password(actor, id).await.unwrap();
    assert_ne!(reset.temporary_password, created.temporary_password);
    let reset_hash = record(&service, id).await.user.password;
    assert!(
        password_auth::verify_password(
            format!("{:x}", md5::compute(&reset.temporary_password)),
            &reset_hash
        )
        .is_ok()
    );
    assert!(
        !serde_json::to_string(&reset.user)
            .unwrap()
            .contains(&reset_hash)
    );
    assert!(
        password_auth::verify_password(
            format!("{:x}", md5::compute(created.temporary_password)),
            &record(&service, id).await.user.password
        )
        .is_err()
    );
    service.set_role(actor, id, "admin").await.unwrap();
    assert_eq!(record(&service, id).await.role, "admin");
    assert!(matches!(
        service.set_access(actor, actor, false).await,
        Err(ManagementError::SelfActionForbidden)
    ));
    assert!(matches!(
        service.set_role(actor, actor, "user").await,
        Err(ManagementError::SelfActionForbidden)
    ));
    assert!(matches!(
        service.reset_password(actor, actor).await,
        Err(ManagementError::SelfActionForbidden)
    ));
    let events = service.list_events(actor, Some(id), 1, 100).await.unwrap();
    assert!(events.items.iter().any(|e| e.action == "password_reset"));
    let json = serde_json::to_string(&events).unwrap();
    assert!(!json.contains(&reset.temporary_password));
    assert!(!json.contains("password_hash"));
    assert!(!json.contains(&reset_hash));
    assert_eq!(
        events.items.iter().filter(|e| e.action == "access").count(),
        2
    );
}
#[tokio::test]
async fn account_service_authorization_unknown_identity_and_rollback() {
    let (service, actor) = fixture().await;
    let user = users::ActiveModel {
        username: Set("historical-user".into()),
        password: Set("unusable-test-hash".into()),
        ..Default::default()
    }
    .insert(service.db.orm_db())
    .await
    .unwrap();
    assert!(matches!(
        service.reset_password(actor, user.id).await,
        Err(ManagementError::LocalPasswordUnavailable)
    ));
    assert!(matches!(
        service.create_user(user.id, "denied-user").await,
        Err(ManagementError::Forbidden)
    ));
    let fresh = service.create_user(actor, "limited-user").await.unwrap();
    assert!(matches!(
        service
            .list_users(fresh.user.id, UserListFilter::default())
            .await,
        Err(ManagementError::PasswordChangeRequired)
    ));
    let before = record(&service, user.id).await;
    // Force success-audit insertion to fail: mutation must roll back before failure audit.
    service.db.orm_db().execute(sea_orm::Statement::from_string(sea_orm::DatabaseBackend::Sqlite,"CREATE TRIGGER test_audit_failure BEFORE INSERT ON user_events WHEN NEW.result='success' BEGIN SELECT RAISE(ABORT, 'test failure'); END".to_owned())).await.unwrap();
    assert!(matches!(
        service.set_access(actor, user.id, false).await,
        Err(ManagementError::InternalError)
    ));
    assert_eq!(record(&service, user.id).await.access, before.access);
    let events = service
        .list_events(actor, Some(user.id), 1, 20)
        .await
        .unwrap();
    assert!(events.items.iter().any(|e| e.result == "failure"));
    assert!(
        users::Entity::find()
            .filter(users::Column::Username.eq("denied-user"))
            .one(service.db.orm_db())
            .await
            .unwrap()
            .is_none()
    );
}
#[tokio::test]
async fn account_service_concurrent_admin_changes_preserve_one_and_duplicate_create() {
    let (service, actor) = fixture().await;
    let other = service
        .recover_admin("second-operator", TEST_PASSWORD)
        .await
        .unwrap()
        .id;
    // Remove initial seeded admin membership in the isolated fixture only.
    let tx = service.db.orm_db().begin().await.unwrap();
    store::lock_user_management(&tx).await.unwrap();
    tx.execute(sea_orm::Statement::from_sql_and_values(sea_orm::DatabaseBackend::Sqlite,"DELETE FROM users_groups WHERE user_id NOT IN (?,?) AND group_id IN (SELECT id FROM groups WHERE name='admins')",[actor.into(),other.into()])).await.unwrap();
    tx.commit().await.unwrap();
    let (a, b) = tokio::join!(
        service.set_role(actor, other, "user"),
        service.set_role(other, actor, "user")
    );
    assert_eq!(usize::from(a.is_ok()) + usize::from(b.is_ok()), 1);
    assert!(
        matches!(a, Err(ManagementError::Forbidden))
            || matches!(b, Err(ManagementError::Forbidden))
    );
    assert_eq!(
        store::count_enabled_admins(service.db.orm_db())
            .await
            .unwrap(),
        1
    );
    let remaining = if record(&service, actor).await.role == "admin" {
        actor
    } else {
        other
    };
    let (a, b) = tokio::join!(
        service.create_user(remaining, "duplicate-user"),
        service.create_user(remaining, "duplicate-user")
    );
    assert_eq!(usize::from(a.is_ok()) + usize::from(b.is_ok()), 1);
    assert!(
        matches!(a, Err(ManagementError::UsernameConflict))
            || matches!(b, Err(ManagementError::UsernameConflict))
    );
}
#[tokio::test]
async fn account_service_offline_recovery_and_binding_preserve_identity() {
    let (service, actor) = fixture().await;
    let before = record(&service, actor).await;
    let recovered = service
        .recover_admin("wum-operator", "another-fictional-password")
        .await
        .unwrap();
    assert_eq!(recovered.id, actor);
    assert_eq!(
        record(&service, actor).await.access.auth_version,
        before.access.auth_version + 1
    );
    let target = service
        .create_user(actor, "oidc-binding-target")
        .await
        .unwrap()
        .user
        .id;
    service.set_access(actor, target, false).await.unwrap();
    let before = record(&service, target).await;
    service
        .bind_oidc_user(
            "oidc-binding-target",
            "https://idp.example.invalid",
            "test-subject",
        )
        .await
        .unwrap();
    let after = record(&service, target).await;
    assert_eq!(before.access, after.access);
    assert_eq!(before.user, after.user);
    assert_eq!(before.role, after.role);
    assert!(matches!(
        service
            .bind_oidc_user(
                "wum-operator",
                "https://idp.example.invalid",
                "test-subject"
            )
            .await,
        Err(ManagementError::IdentityConflict)
    ));
}

#[tokio::test]
async fn account_service_last_admin_guard_and_concurrent_disable() {
    let (service, actor) = fixture().await;
    let other = service
        .recover_admin("disable-operator", TEST_PASSWORD)
        .await
        .unwrap()
        .id;
    let tx = service.db.orm_db().begin().await.unwrap();
    store::lock_user_management(&tx).await.unwrap();
    tx.execute(sea_orm::Statement::from_sql_and_values(sea_orm::DatabaseBackend::Sqlite, "DELETE FROM users_groups WHERE user_id NOT IN (?,?) AND group_id IN (SELECT id FROM groups WHERE name='admins')", [actor.into(),other.into()])).await.unwrap();
    tx.commit().await.unwrap();
    let (a, b) = tokio::join!(
        service.set_access(actor, other, false),
        service.set_access(other, actor, false)
    );
    assert_eq!(usize::from(a.is_ok()) + usize::from(b.is_ok()), 1);
    assert!(
        matches!(a, Err(ManagementError::Unauthenticated))
            || matches!(b, Err(ManagementError::Unauthenticated))
    );
    assert_eq!(
        store::count_enabled_admins(service.db.orm_db())
            .await
            .unwrap(),
        1
    );
    let remaining = if record(&service, actor).await.access.web_login_enabled {
        actor
    } else {
        other
    };
    let manager = users::ActiveModel {
        username: Set("custom-manager-test".into()),
        password: Set("unusable-test-hash".into()),
        ..Default::default()
    }
    .insert(service.db.orm_db())
    .await
    .unwrap();
    // Isolated fixture exercises the last-admin guard independently of self-protection.
    service.db.orm_db().execute(sea_orm::Statement::from_string(sea_orm::DatabaseBackend::Sqlite,"INSERT INTO groups(name) VALUES ('test-managers'); INSERT INTO groups_permissions(group_id,permission_id) SELECT g.id,p.id FROM groups g,permissions p WHERE g.name='test-managers' AND p.name='users.manage'".to_owned())).await.unwrap();
    service.db.orm_db().execute(sea_orm::Statement::from_sql_and_values(sea_orm::DatabaseBackend::Sqlite,"INSERT INTO users_groups(user_id,group_id) SELECT ?,id FROM groups WHERE name='test-managers'",[manager.id.into()])).await.unwrap();
    let before = record(&service, remaining).await;
    assert!(matches!(
        service.set_access(manager.id, remaining, false).await,
        Err(ManagementError::LastAdmin)
    ));
    assert!(matches!(
        service.set_role(manager.id, remaining, "user").await,
        Err(ManagementError::LastAdmin)
    ));
    let after = record(&service, remaining).await;
    assert_eq!(before.access, after.access);
    assert_eq!(before.role, after.role);
}
