use super::*;
use crate::{db::Db, migrator::Migrator};
use sea_orm::{Database, TransactionTrait};
use sea_orm_migration::{MigratorTrait, SchemaManager};

async fn insert_user<C: ConnectionTrait>(db: &C, name: &str) -> i32 {
    db.execute(stmt(
        db,
        "INSERT INTO users(username,password) VALUES (?, 'unusable-test-hash')",
        vec![name.into()],
    ))
    .await
    .unwrap();
    db.query_one(stmt(
        db,
        "SELECT id FROM users WHERE username=?",
        vec![name.into()],
    ))
    .await
    .unwrap()
    .unwrap()
    .try_get("", "id")
    .unwrap()
}

#[tokio::test]
async fn user_management_upgrade_preserves_legacy_records() {
    let db = Database::connect("sqlite::memory:").await.unwrap();
    let migrations = Migrator::migrations();
    let manager = SchemaManager::new(&db);
    for migration in &migrations[..migrations.len() - 1] {
        migration.up(&manager).await.unwrap();
    }
    let id = insert_user(&db, "legacy-storage-test").await;
    db.execute(stmt(
        &db,
        "INSERT INTO users_groups(user_id,group_id) SELECT ?,id FROM groups WHERE name='admins'",
        vec![id.into()],
    ))
    .await
    .unwrap();
    db.execute(stmt(&db, "INSERT INTO user_running_network_configs(user_id,device_id,network_instance_id,network_config,disabled,create_time,update_time,source) VALUES (?, 'test-device', 'test-instance', 'test-config', FALSE, '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z', 'web')", vec![id.into()])).await.unwrap();
    let before = users::Entity::find_by_id(id)
        .one(&db)
        .await
        .unwrap()
        .unwrap();
    let groups_before = count(
        &db,
        "SELECT COUNT(*) AS count FROM users_groups WHERE user_id=?",
        vec![id.into()],
    )
    .await
    .unwrap();
    migrations.last().unwrap().up(&manager).await.unwrap();
    let record = get_user_record(&db, id).await.unwrap().unwrap();
    assert_eq!(record.user, before);
    assert_eq!(record.access, UserAccess::legacy(id));
    assert_eq!(record.role, "admin");
    let resource = db.query_one(stmt(&db,"SELECT user_id,network_config FROM user_running_network_configs WHERE device_id='test-device'",vec![])).await.unwrap().unwrap();
    assert_eq!(resource.try_get::<i32>("", "user_id").unwrap(), id);
    assert_eq!(
        resource.try_get::<String>("", "network_config").unwrap(),
        "test-config"
    );
    assert!(has_manage_permission(&db, id).await.unwrap());
    assert_eq!(
        count(
            &db,
            "SELECT COUNT(*) AS count FROM users_groups WHERE user_id=?",
            vec![id.into()]
        )
        .await
        .unwrap(),
        groups_before
    );
    // A rollback removes only new metadata, preserving legacy accounts and groups.
    migrations.last().unwrap().down(&manager).await.unwrap();
    assert_eq!(
        users::Entity::find_by_id(id)
            .one(&db)
            .await
            .unwrap()
            .unwrap(),
        before
    );
    migrations.last().unwrap().up(&manager).await.unwrap();
    assert_eq!(
        get_user_record(&db, id).await.unwrap().unwrap().access,
        UserAccess::legacy(id)
    );
}

#[tokio::test]
async fn user_management_filters_identity_and_transaction_rollback() {
    let db = Db::memory_db().await;
    let conn = db.orm_db();
    let ordinary = insert_user(conn, "wum-Alice_100%").await;
    let admin = insert_user(conn, "wum-Bob").await;
    let no_metadata = get_user_record(conn, ordinary).await.unwrap().unwrap();
    assert_eq!(no_metadata.access, UserAccess::legacy(ordinary));
    assert!(!has_manage_permission(conn, ordinary).await.unwrap());
    let tx = conn.begin().await.unwrap();
    lock_user_management(&tx).await.unwrap();
    set_user_role(&tx, admin, "admin").await.unwrap();
    set_user_role(&tx, admin, "admin").await.unwrap();
    let mut access = ensure_user_access(&tx, ordinary).await.unwrap();
    access.web_login_enabled = false;
    access.auth_version = 2;
    access.created_at = Some(Utc::now());
    save_user_access(&tx, &access).await.unwrap();
    bind_identity(&tx, ordinary, "https://idp.example.invalid", "subject-test")
        .await
        .unwrap();
    bind_identity(&tx, ordinary, "https://idp.example.invalid", "subject-test")
        .await
        .unwrap();
    assert!(
        bind_identity(&tx, admin, "https://idp.example.invalid", "subject-test")
            .await
            .is_err()
    );
    append_user_event(
        &tx,
        NewUserEvent {
            actor_id: Some(admin),
            target_id: Some(ordinary),
            action: "access".into(),
            occurred_at: Utc::now(),
            result: "success".into(),
        },
    )
    .await
    .unwrap();
    tx.commit().await.unwrap();
    assert_eq!(
        get_user_record(conn, ordinary)
            .await
            .unwrap()
            .unwrap()
            .access,
        access
    );
    assert!(has_manage_permission(conn, admin).await.unwrap());
    assert_eq!(count(conn,"SELECT COUNT(*) AS count FROM users_groups ug JOIN groups g ON g.id=ug.group_id WHERE ug.user_id=? AND g.name='admins'",vec![admin.into()]).await.unwrap(),1);
    let filter = UserListFilter {
        search: Some("ALICE_100%".into()),
        role: Some("user".into()),
        web_login_enabled: Some(false),
        ..Default::default()
    };
    let (records, total) = list_user_records(conn, filter).await.unwrap();
    assert_eq!(total, 1);
    assert_eq!(records[0].user.id, ordinary);
    let (records, total) = list_user_records(
        conn,
        UserListFilter {
            search: Some("wum-".into()),
            page: 2,
            page_size: 1,
            ..Default::default()
        },
    )
    .await
    .unwrap();
    assert_eq!(total, 2);
    assert_eq!(records[0].user.id, admin);
    let (records, total) = list_user_records(
        conn,
        UserListFilter {
            search: Some("wum-".into()),
            page: 3,
            page_size: 1,
            ..Default::default()
        },
    )
    .await
    .unwrap();
    assert_eq!(total, 2);
    assert!(records.is_empty());
    assert!(
        list_user_records(
            conn,
            UserListFilter {
                page: 0,
                ..Default::default()
            }
        )
        .await
        .is_err()
    );
    assert!(
        list_user_records(
            conn,
            UserListFilter {
                page: u64::MAX,
                ..Default::default()
            }
        )
        .await
        .is_err()
    );
    let (events, total) = list_user_events(conn, Some(ordinary), 1, 20).await.unwrap();
    assert_eq!(total, 1);
    assert_eq!(events[0].action, "access");
    assert_eq!(
        find_identity(conn, "https://idp.example.invalid", "subject-test")
            .await
            .unwrap(),
        Some(ordinary)
    );
    let enabled_before = count_enabled_admins(conn).await.unwrap();
    let tx = conn.begin().await.unwrap();
    lock_user_management(&tx).await.unwrap();
    set_user_role(&tx, admin, "user").await.unwrap();
    access.auth_version += 1;
    save_user_access(&tx, &access).await.unwrap();
    // Constraint failure is propagated and callers can roll back the entire action.
    assert!(
        append_user_event(
            &tx,
            NewUserEvent {
                actor_id: Some(admin),
                target_id: Some(ordinary),
                action: "invalid-action".into(),
                occurred_at: Utc::now(),
                result: "success".into()
            }
        )
        .await
        .is_err()
    );
    tx.rollback().await.unwrap();
    assert_eq!(count_enabled_admins(conn).await.unwrap(), enabled_before);
    assert_eq!(
        get_user_record(conn, ordinary)
            .await
            .unwrap()
            .unwrap()
            .access
            .auth_version,
        2
    );
    assert_eq!(
        list_user_events(conn, Some(ordinary), 1, 20)
            .await
            .unwrap()
            .1,
        1
    );
}

#[tokio::test]
async fn user_management_write_lock_serializes_connections() {
    let path = std::env::temp_dir().join(format!("wum-storage-{}.db", uuid::Uuid::new_v4()));
    let db = Db::new(path.display().to_string()).await.unwrap();
    let second = Db::new(path.display().to_string()).await.unwrap();
    let first_tx = db.orm_db().begin().await.unwrap();
    lock_user_management(&first_tx).await.unwrap();
    let mut contender = tokio::spawn(async move {
        let tx = second.orm_db().begin().await.unwrap();
        lock_user_management(&tx).await.unwrap();
        tx.commit().await.unwrap();
        second.inner().close().await;
    });
    // The second independent pool cannot acquire the write reservation while held.
    assert!(
        tokio::time::timeout(std::time::Duration::from_millis(100), &mut contender)
            .await
            .is_err()
    );
    first_tx.commit().await.unwrap();
    tokio::time::timeout(std::time::Duration::from_secs(5), contender)
        .await
        .unwrap()
        .unwrap();
    db.inner().close().await;
    std::fs::remove_file(path).unwrap();
}

#[tokio::test]
async fn user_management_device_autocreate_preserves_existing_accounts() {
    let db = Db::memory_db().await;
    let created = db.auto_create_user("wum-device-created").await.unwrap();
    let record = get_user_record(db.orm_db(), created.id)
        .await
        .unwrap()
        .unwrap();
    assert_eq!(record.access.source, "device");
    assert_eq!(record.access.local_password_enabled, Some(false));
    assert!(record.access.created_at.is_some());
    assert_eq!(record.role, "user");
    assert!(
        !has_manage_permission(db.orm_db(), created.id)
            .await
            .unwrap()
    );

    let tx = db.orm_db().begin().await.unwrap();
    lock_user_management(&tx).await.unwrap();
    let mut access = record.access;
    access.web_login_enabled = false;
    access.auth_version = 7;
    access.source = "oidc".into();
    save_user_access(&tx, &access).await.unwrap();
    bind_identity(
        &tx,
        created.id,
        "https://idp.example.invalid",
        "device-autocreate-test",
    )
    .await
    .unwrap();
    tx.execute(stmt(&tx, "INSERT INTO user_running_network_configs(user_id,device_id,network_instance_id,network_config,disabled,create_time,update_time,source) VALUES (?, 'autocreate-device', 'autocreate-instance', 'isolated-config', FALSE, '2020-01-01T00:00:00Z', '2020-01-01T00:00:00Z', 'web')",vec![created.id.into()])).await.unwrap();
    tx.commit().await.unwrap();

    assert!(db.auto_create_user("wum-device-created").await.is_err());
    let after = get_user_record(db.orm_db(), created.id)
        .await
        .unwrap()
        .unwrap();
    assert_eq!(after.user, created);
    assert_eq!(after.access, access);
    assert_eq!(
        find_identity(
            db.orm_db(),
            "https://idp.example.invalid",
            "device-autocreate-test"
        )
        .await
        .unwrap(),
        Some(created.id)
    );
    let resource = db.orm_db().query_one(stmt(db.orm_db(), "SELECT user_id,network_config FROM user_running_network_configs WHERE device_id='autocreate-device'", vec![])).await.unwrap().unwrap();
    assert_eq!(resource.try_get::<i32>("", "user_id").unwrap(), created.id);
    assert_eq!(
        resource.try_get::<String>("", "network_config").unwrap(),
        "isolated-config"
    );
}
