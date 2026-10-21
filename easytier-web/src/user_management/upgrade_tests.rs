//! Upgrade acceptance uses disposable files and real CLI/HTTP, never Core/TUN.
use crate::{db::Db, migrator::Migrator};
use sea_orm::SqlxSqliteConnector;
use sea_orm_migration::MigratorTrait;
use serde_json::{Value, json};
use sqlx::{Row, SqlitePool};
use std::{
    io::Write,
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    time::Duration,
};

const NEW_PASSWORD: &str = "fictional-upgrade-password-2026";
const RECOVERED_PASSWORD: &str = "fictional-recovered-password-2026";
struct Fixture(PathBuf);
impl Fixture {
    fn new() -> Self {
        let p = std::env::temp_dir().join(format!("easytier-wum-upgrade-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir(&p).unwrap();
        Self(p)
    }
    fn db(&self) -> PathBuf {
        self.0.join("isolated.sqlite")
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}
fn binary() -> PathBuf {
    let p = std::env::var_os("WUM_TEST_WEB_BINARY")
        .map(PathBuf::from)
        .unwrap_or_else(|| {
            PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../target/debug/easytier-web")
        });
    assert!(
        p.is_file(),
        "Build the current easytier-web binary before upgrade acceptance"
    );
    p
}
fn command(db: &Path) -> Command {
    let mut c = Command::new(binary());
    c.env_clear().arg("--db").arg(db);
    c
}
fn offline(db: &Path, args: &[&str], password: Option<&str>, success: bool) {
    let mut child = command(db)
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    if let Some(password) = password {
        child
            .stdin
            .as_mut()
            .unwrap()
            .write_all(format!("{password}\n").as_bytes())
            .unwrap();
    }
    drop(child.stdin.take());
    let output = child.wait_with_output().unwrap();
    assert_eq!(output.status.success(), success, "offline operation status");
    let output = [output.stdout, output.stderr].concat();
    for password in [NEW_PASSWORD, RECOVERED_PASSWORD] {
        assert!(
            !String::from_utf8_lossy(&output).contains(password),
            "offline output excludes passwords"
        );
        assert!(
            !String::from_utf8_lossy(&output).contains(&digest(password)),
            "offline output excludes digests"
        );
    }
}
fn digest(password: &str) -> String {
    format!("{:x}", md5::compute(password))
}
async fn pool(path: &Path) -> SqlitePool {
    SqlitePool::connect(&format!("sqlite:{}?mode=rwc", path.display()))
        .await
        .unwrap()
}
async fn snapshot(pool: &SqlitePool) -> Vec<Value> {
    let mut result = Vec::new();
    for r in sqlx::query("SELECT id,username,password FROM users ORDER BY id")
        .fetch_all(pool)
        .await
        .unwrap()
    {
        result.push(json!([
            r.get::<i64, _>(0),
            r.get::<String, _>(1),
            r.get::<String, _>(2)
        ]));
    }
    for r in sqlx::query("SELECT user_id,group_id FROM users_groups ORDER BY user_id,group_id")
        .fetch_all(pool)
        .await
        .unwrap()
    {
        result.push(json!([r.get::<i64, _>(0), r.get::<i64, _>(1)]));
    }
    for r in sqlx::query("SELECT user_id,machine_id,hostname,first_seen_time,last_seen_time FROM devices ORDER BY user_id,machine_id").fetch_all(pool).await.unwrap() { result.push(json!([r.get::<i64,_>(0),r.get::<String,_>(1),r.get::<String,_>(2),r.get::<String,_>(3),r.get::<String,_>(4)])); }
    for r in sqlx::query("SELECT user_id,device_id,network_config FROM user_running_network_configs ORDER BY user_id,device_id").fetch_all(pool).await.unwrap() { result.push(json!([r.get::<i64,_>(0),r.get::<String,_>(1),r.get::<String,_>(2)])); }
    result
}
struct Server {
    child: Child,
    base: String,
    client: reqwest::Client,
}
impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}
impl Server {
    async fn start(db: &Path) -> Self {
        easytier::utils::init_crypto_provider();
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        drop(listener);
        let child = command(db)
            .args([
                "--api-server-addr",
                "127.0.0.1",
                "--api-server-port",
                &port.to_string(),
                "--config-server-protocol",
                "tcp",
                "--config-server-port",
                "0",
                "--console-log-level",
                "off",
            ])
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap();
        let mut s = Self {
            child,
            base: format!("http://127.0.0.1:{port}/api/v1"),
            client: reqwest::Client::builder().no_proxy().build().unwrap(),
        };
        for _ in 0..100 {
            if s.client
                .get(format!("{}/auth/policy", s.base))
                .send()
                .await
                .is_ok_and(|r| r.status().is_success())
            {
                return s;
            }
            assert!(
                s.child.try_wait().unwrap().is_none(),
                "isolated backend exited before readiness"
            );
            tokio::time::sleep(Duration::from_millis(100)).await;
        }
        panic!("isolated backend readiness timed out");
    }
    async fn call(
        &self,
        method: reqwest::Method,
        path: &str,
        cookie: Option<&str>,
        body: Value,
        expected: u16,
    ) -> Value {
        let mut request = self
            .client
            .request(method, format!("{}{path}", self.base))
            .header("X-Requested-With", "EasyTier")
            .json(&body);
        if let Some(cookie) = cookie {
            request = request.header("Cookie", cookie);
        }
        let response = request.send().await.unwrap();
        assert_eq!(
            response.status().as_u16(),
            expected,
            "HTTP status for {path}"
        );
        let bytes = response.bytes().await.unwrap();
        serde_json::from_slice(&bytes).unwrap_or(Value::Null)
    }
    async fn login(&self, username: &str, password: &str, expected: u16) -> Option<String> {
        let response = self
            .client
            .post(format!("{}/auth/login", self.base))
            .json(&json!({"username":username,"password":digest(password)}))
            .send()
            .await
            .unwrap();
        assert_eq!(response.status().as_u16(), expected, "login status");
        response
            .headers()
            .get("set-cookie")
            .map(|v| v.to_str().unwrap().split(';').next().unwrap().to_owned())
    }
    async fn me(&self, cookie: &str) -> Value {
        self.call(
            reqwest::Method::GET,
            "/auth/me",
            Some(cookie),
            json!({}),
            200,
        )
        .await
    }
}

#[tokio::test]
#[ignore = "requires a separately built current easytier-web binary; run explicit upgrade acceptance"]
async fn user_management_upgrade_old_database_http_and_backup() {
    let fixture = Fixture::new();
    let path = fixture.db();
    let db = pool(&path).await;
    let orm = SqlxSqliteConnector::from_sqlx_sqlite_pool(db.clone());
    Migrator::up(&orm, Some((Migrator::migrations().len() - 1) as u32))
        .await
        .unwrap();
    let hash = password_auth::generate_hash(digest(NEW_PASSWORD));
    sqlx::query("INSERT INTO users(username,password) VALUES ('fictional-legacy',?)")
        .bind(hash)
        .execute(&db)
        .await
        .unwrap();
    sqlx::query("INSERT INTO users_groups(user_id,group_id) SELECT u.id,g.id FROM users u,groups g WHERE u.username='fictional-legacy' AND g.name='users'").execute(&db).await.unwrap();
    sqlx::query("INSERT INTO devices(user_id,machine_id,hostname,first_seen_time,last_seen_time) SELECT id,'fixture-device','isolated-device','2020-01-01','2020-01-02' FROM users WHERE username='fictional-legacy'").execute(&db).await.unwrap();
    sqlx::query("INSERT INTO user_running_network_configs(user_id,device_id,network_instance_id,network_config,disabled,create_time,update_time,source) SELECT id,'fixture-device','fixture-instance','isolated-resource',FALSE,'2020-01-01','2020-01-01','web' FROM users WHERE username='fictional-legacy'").execute(&db).await.unwrap();
    let before = snapshot(&db).await;
    drop(orm);
    db.close().await;
    let backup = fixture.0.join("before-upgrade.sqlite");
    std::fs::copy(&path, &backup).unwrap();
    let server = Server::start(&path).await;
    let db = pool(&path).await;
    assert!(
        snapshot(&db).await == before,
        "migration preserves legacy IDs, hashes, groups and resources"
    );
    let unknown: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM user_access WHERE source='unknown' AND local_password_enabled IS NULL AND created_at IS NULL AND last_login_at IS NULL AND web_login_enabled=TRUE AND must_change_password=FALSE").fetch_one(&db).await.unwrap();
    assert_eq!(unknown, 3);
    let cookie = server.login("admin", "admin", 200).await.unwrap();
    let me = server.me(&cookie).await;
    assert_eq!(me["role"], "admin");
    assert_eq!(me["default_password_warning"], true);
    assert_eq!(me["must_change_password"], false);
    server
        .call(
            reqwest::Method::GET,
            "/admin/users",
            Some(&cookie),
            json!({}),
            200,
        )
        .await;
    server
        .call(
            reqwest::Method::PUT,
            "/auth/password",
            Some(&cookie),
            json!({"new_password":digest(RECOVERED_PASSWORD)}),
            200,
        )
        .await;
    server
        .call(
            reqwest::Method::GET,
            "/auth/me",
            Some(&cookie),
            json!({}),
            401,
        )
        .await;
    server.login("admin", "admin", 401).await;
    let updated = server
        .login("admin", RECOVERED_PASSWORD, 200)
        .await
        .unwrap();
    assert_eq!(server.me(&updated).await["default_password_warning"], false);
    let ordinary = server.login("user", "user", 200).await.unwrap();
    assert_eq!(server.me(&ordinary).await["role"], "user");
    assert_eq!(server.me(&ordinary).await["default_password_warning"], true);
    server
        .call(
            reqwest::Method::GET,
            "/admin/users",
            Some(&ordinary),
            json!({}),
            403,
        )
        .await;
    let legacy = server
        .login("fictional-legacy", NEW_PASSWORD, 200)
        .await
        .unwrap();
    assert_eq!(server.me(&legacy).await["source"], "unknown");
    let resource: String = sqlx::query_scalar(
        "SELECT network_config FROM user_running_network_configs WHERE device_id='fixture-device'",
    )
    .fetch_one(&db)
    .await
    .unwrap();
    assert_eq!(resource, "isolated-resource");
    drop(server);
    db.close().await;
    let restored = fixture.0.join("restored.sqlite");
    std::fs::copy(&backup, &restored).unwrap();
    let restored_pool = pool(&restored).await;
    assert!(
        snapshot(&restored_pool).await == before,
        "closed-file backup restores original data"
    );
    let metadata: i64 =
        sqlx::query_scalar("SELECT COUNT(*) FROM sqlite_master WHERE name='user_access'")
            .fetch_one(&restored_pool)
            .await
            .unwrap();
    assert_eq!(metadata, 0);
    restored_pool.close().await;
    let restored_server = Server::start(&restored).await;
    let restored_cookie = restored_server.login("admin", "admin", 200).await.unwrap();
    assert_eq!(
        restored_server.me(&restored_cookie).await["default_password_warning"],
        true
    );
    drop(restored_server);
    // Actual metadata downgrade demonstrates its boundary, not an old-binary run.
    let upgraded = Db::new(path.to_str().unwrap()).await.unwrap();
    sqlx::query("UPDATE user_access SET web_login_enabled=FALSE WHERE user_id=(SELECT id FROM users WHERE username='user')").execute(&upgraded.inner()).await.unwrap();
    Migrator::down(upgraded.orm_db(), Some(1)).await.unwrap();
    let metadata: i64 =
        sqlx::query_scalar("SELECT COUNT(*) FROM sqlite_master WHERE name='user_access'")
            .fetch_one(&upgraded.inner())
            .await
            .unwrap();
    assert_eq!(metadata, 0);
    let preserved_hash: String =
        sqlx::query_scalar("SELECT password FROM users WHERE username='user'")
            .fetch_one(&upgraded.inner())
            .await
            .unwrap();
    assert!(password_auth::verify_password(digest("user"), &preserved_hash).is_ok());
    upgraded.inner().close().await;
}

#[tokio::test]
#[ignore = "requires a separately built current easytier-web binary; run explicit upgrade acceptance"]
async fn user_management_upgrade_offline_initialization_recovery_and_binding() {
    let fixture = Fixture::new();
    let path = fixture.db();
    offline(
        &path,
        &["--recover-admin", "admin"],
        Some(NEW_PASSWORD),
        true,
    );
    let server = Server::start(&path).await;
    server.login("admin", "admin", 401).await;
    let cookie = server.login("admin", NEW_PASSWORD, 200).await.unwrap();
    assert_eq!(server.me(&cookie).await["default_password_warning"], false);
    let db = pool(&path).await;
    let admin_id: i64 = sqlx::query_scalar("SELECT id FROM users WHERE username='admin'")
        .fetch_one(&db)
        .await
        .unwrap();
    let ordinary_id: i64 = sqlx::query_scalar("SELECT id FROM users WHERE username='user'")
        .fetch_one(&db)
        .await
        .unwrap();
    sqlx::query("INSERT INTO devices(user_id,machine_id,hostname,first_seen_time,last_seen_time) VALUES (?,'recovery-device','isolated','2020-01-01','2020-01-01')").bind(admin_id).execute(&db).await.unwrap();
    sqlx::query("UPDATE user_access SET web_login_enabled=FALSE,auth_version=auth_version+1 WHERE user_id IN (?,?)").bind(admin_id).bind(ordinary_id).execute(&db).await.unwrap();
    sqlx::query("DELETE FROM users_groups WHERE user_id=?")
        .bind(admin_id)
        .execute(&db)
        .await
        .unwrap();
    server
        .call(
            reqwest::Method::GET,
            "/auth/me",
            Some(&cookie),
            json!({}),
            401,
        )
        .await;
    drop(server);
    db.close().await;
    offline(
        &path,
        &["--recover-admin", "admin"],
        Some(RECOVERED_PASSWORD),
        true,
    );
    offline(
        &path,
        &[
            "--bind-oidc-user",
            "user",
            "--oidc-issuer",
            "https://idp.example.invalid",
            "--oidc-subject",
            "fictional-subject",
        ],
        None,
        true,
    );
    offline(
        &path,
        &[
            "--bind-oidc-user",
            "admin",
            "--oidc-issuer",
            "https://idp.example.invalid",
            "--oidc-subject",
            "fictional-subject",
        ],
        None,
        false,
    );
    let server = Server::start(&path).await;
    server.login("admin", NEW_PASSWORD, 401).await;
    let recovered = server
        .login("admin", RECOVERED_PASSWORD, 200)
        .await
        .unwrap();
    let me = server.me(&recovered).await;
    assert_eq!(me["id"], admin_id);
    assert_eq!(me["role"], "admin");
    server
        .call(
            reqwest::Method::GET,
            "/auth/me",
            Some(&cookie),
            json!({}),
            401,
        )
        .await;
    server.login("user", "user", 401).await;
    let db = pool(&path).await;
    let target: (bool, String) =
        sqlx::query_as("SELECT web_login_enabled,source FROM user_access WHERE user_id=?")
            .bind(ordinary_id)
            .fetch_one(&db)
            .await
            .unwrap();
    assert_eq!(target, (false, "unknown".into()));
    let binding: i64 = sqlx::query_scalar("SELECT user_id FROM user_identities WHERE issuer='https://idp.example.invalid' AND subject='fictional-subject'").fetch_one(&db).await.unwrap();
    assert_eq!(binding, ordinary_id);
    let admin_grants: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM users_groups ug JOIN groups g ON g.id=ug.group_id WHERE ug.user_id=? AND g.name='admins'").bind(ordinary_id).fetch_one(&db).await.unwrap();
    assert_eq!(admin_grants, 0);
    let device_owner: i64 =
        sqlx::query_scalar("SELECT user_id FROM devices WHERE machine_id='recovery-device'")
            .fetch_one(&db)
            .await
            .unwrap();
    assert_eq!(device_owner, admin_id);
    let audit: Vec<(String, String)> =
        sqlx::query_as("SELECT action,result FROM user_events WHERE actor_id IS NULL ORDER BY id")
            .fetch_all(&db)
            .await
            .unwrap();
    assert!(
        audit
            .iter()
            .any(|x| x == &("recover".into(), "success".into()))
    );
    assert!(
        audit
            .iter()
            .any(|x| x == &("identity_bind".into(), "success".into()))
    );
    drop(server);
    db.close().await;
    // An explicit new-name bootstrap does not remove the old migration's seeds.
    let other = fixture.0.join("other-bootstrap.sqlite");
    offline(
        &other,
        &["--recover-admin", "fictional-new-admin"],
        Some(NEW_PASSWORD),
        true,
    );
    let db = pool(&other).await;
    let names: Vec<String> = sqlx::query_scalar("SELECT username FROM users ORDER BY username")
        .fetch_all(&db)
        .await
        .unwrap();
    assert_eq!(names, vec!["admin", "fictional-new-admin", "user"]);
    db.close().await;
}
