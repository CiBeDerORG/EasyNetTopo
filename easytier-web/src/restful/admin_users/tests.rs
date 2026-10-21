use super::super::{BrowserSecurity, account_routes, browser_session_routes, oidc};
use super::*;
use crate::{
    FeatureFlags,
    client_manager::{ClientManager, HeartbeatPolicy},
    db::Db,
    webhook::WebhookConfig,
};
use axum::http::{Method, header};
use serde_json::{Value, json};
use std::sync::Arc;
use tokio_util::task::AbortOnDropHandle;

const PASSWORD: &str = "fictional-http-password-123";
struct Server {
    base: String,
    db: Db,
    service: UserManagementService,
    admin: i32,
    client: reqwest::Client,
    _task: AbortOnDropHandle<()>,
}
impl Server {
    async fn new(origin: Option<&str>) -> Self {
        easytier::utils::init_crypto_provider();
        let db = Db::memory_db().await;
        let service = UserManagementService::new(db.clone());
        let admin = service
            .recover_admin("http-operator", PASSWORD)
            .await
            .unwrap()
            .id;
        let flags = Arc::new(FeatureFlags {
            disable_registration: true,
            ..Default::default()
        });
        let manager = Arc::new(ClientManager::new(
            db.clone(),
            None,
            HeartbeatPolicy::default(),
            flags.clone(),
            Arc::new(WebhookConfig::new(None, None, None, None, None)),
        ));
        let store = tower_sessions_sqlx_store::SqliteStore::new(db.inner());
        store.migrate().await.unwrap();
        let app = account_routes(&db, flags)
            .with_state(manager)
            .route("/api/v1/parse-config", post(|| async { StatusCode::OK }))
            .layer(Extension(oidc::OidcConfig::disabled()));
        let app = browser_session_routes(
            app,
            db.clone(),
            store,
            BrowserSecurity::from_origin(origin).unwrap(),
        );
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        let task = AbortOnDropHandle::new(tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap()
        }));
        Self {
            base,
            db,
            service,
            admin,
            client: reqwest::Client::builder().no_proxy().build().unwrap(),
            _task: task,
        }
    }
    fn request(
        &self,
        method: Method,
        path: &str,
        cookie: Option<&str>,
        body: Value,
    ) -> reqwest::RequestBuilder {
        let mut req = self
            .client
            .request(method, format!("{}{path}", self.base))
            .header("X-Requested-With", "EasyTier")
            .json(&body);
        if let Some(cookie) = cookie {
            req = req.header(header::COOKIE, cookie)
        }
        req
    }
    async fn call(
        &self,
        method: Method,
        path: &str,
        cookie: Option<&str>,
        body: Value,
    ) -> (StatusCode, Value) {
        let res = self
            .request(method, path, cookie, body)
            .send()
            .await
            .unwrap();
        let status = res.status();
        if path.starts_with("/api/v1/admin/") || path == "/api/v1/auth/me" {
            assert_eq!(res.headers()[header::CACHE_CONTROL], "no-store");
        }
        let bytes = res.bytes().await.unwrap();
        let value = serde_json::from_slice(&bytes).unwrap_or(Value::Null);
        (status, value)
    }
    async fn login(&self, username: &str, password: &str) -> String {
        let res = self
            .request(
                Method::POST,
                "/api/v1/auth/login",
                None,
                json!({"username":username,"password":format!("{:x}",md5::compute(password))}),
            )
            .send()
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::OK);
        let cookie = res
            .headers()
            .get(header::SET_COOKIE)
            .unwrap()
            .to_str()
            .unwrap();
        assert!(cookie.contains("HttpOnly"));
        assert!(cookie.contains("SameSite=Lax"));
        cookie.split(';').next().unwrap().to_owned()
    }
    async fn create(&self, cookie: &str, name: &str) -> Value {
        let (status, value) = self
            .call(
                Method::POST,
                "/api/v1/admin/users",
                Some(cookie),
                json!({"username":name}),
            )
            .await;
        assert_eq!(status, StatusCode::CREATED);
        value
    }
}

#[tokio::test]
async fn admin_http_authorization_and_parameter_boundary() {
    let s = Server::new(None).await;
    let admin = s.login("http-operator", PASSWORD).await;
    let created = s.create(&admin, "http-ordinary").await;
    let id = created["user"]["id"].as_i64().unwrap();
    let temporary = created["temporary_password"].as_str().unwrap();
    let restricted = s.login("http-ordinary", temporary).await;
    let routes = vec![
        (Method::GET, "/api/v1/admin/users".into(), json!({})),
        (Method::GET, format!("/api/v1/admin/users/{id}"), json!({})),
        (
            Method::POST,
            "/api/v1/admin/users".into(),
            json!({"username":"blocked"}),
        ),
        (
            Method::PUT,
            format!("/api/v1/admin/users/{id}/access"),
            json!({"web_login_enabled":false}),
        ),
        (
            Method::PUT,
            format!("/api/v1/admin/users/{id}/role"),
            json!({"role":"admin"}),
        ),
        (
            Method::POST,
            format!("/api/v1/admin/users/{id}/password-reset"),
            json!({}),
        ),
        (
            Method::POST,
            format!("/api/v1/admin/users/{id}/revoke-sessions"),
            json!({}),
        ),
        (Method::GET, "/api/v1/admin/user-events".into(), json!({})),
    ];
    for (method, path, body) in &routes {
        assert_eq!(
            s.call(method.clone(), path, None, body.clone()).await.0,
            StatusCode::UNAUTHORIZED
        );
        let denied = s
            .call(method.clone(), path, Some(&restricted), body.clone())
            .await;
        assert_eq!(denied.0, StatusCode::FORBIDDEN);
        assert_eq!(denied.1["code"], "password_change_required");
    }
    assert_eq!(
        s.call(
            Method::POST,
            "/api/v1/parse-config",
            Some(&restricted),
            json!({})
        )
        .await
        .0,
        StatusCode::FORBIDDEN
    );
    assert_eq!(
        s.call(
            Method::PUT,
            "/api/v1/auth/password",
            Some(&restricted),
            json!({"new_password":format!("{:x}",md5::compute(PASSWORD))})
        )
        .await
        .0,
        StatusCode::OK
    );
    let ordinary = s.login("http-ordinary", PASSWORD).await;
    for (method, path, body) in &routes {
        let denied = s
            .call(method.clone(), path, Some(&ordinary), body.clone())
            .await;
        assert_eq!(denied.0, StatusCode::FORBIDDEN);
        assert_eq!(denied.1["code"], "forbidden")
    }
    for path in [
        "/api/v1/admin/users?page=0",
        "/api/v1/admin/users?page=no",
        "/api/v1/admin/users?page_size=101",
        "/api/v1/admin/users?web_login_enabled=bad",
        "/api/v1/admin/users/-1",
        "/api/v1/admin/users/not-a-number",
        "/api/v1/admin/user-events?target_id=-1",
    ] {
        let res = s.call(Method::GET, path, Some(&admin), json!({})).await;
        assert_eq!(res.0, StatusCode::BAD_REQUEST, "{path}");
        assert_eq!(res.1["code"], "invalid_request");
    }
    assert_eq!(
        s.call(
            Method::GET,
            "/api/v1/admin/users/2147483647",
            Some(&admin),
            json!({})
        )
        .await
        .0,
        StatusCode::NOT_FOUND
    );
    let malformed = s
        .client
        .post(format!("{}/api/v1/admin/users", s.base))
        .header(header::COOKIE, &admin)
        .header("X-Requested-With", "EasyTier")
        .header(header::CONTENT_TYPE, "application/json")
        .body("{")
        .send()
        .await
        .unwrap();
    assert_eq!(malformed.status(), StatusCode::BAD_REQUEST);
    assert_eq!(
        malformed.json::<Value>().await.unwrap()["code"],
        "invalid_request"
    );
    assert_eq!(
        s.call(Method::GET, "/api/v1/auth/policy", None, json!({}))
            .await
            .1["registration_enabled"],
        false
    );
}

#[tokio::test]
async fn admin_http_lifecycle_pagination_audit_and_concurrency() {
    let s = Server::new(None).await;
    let admin = s.login("http-operator", PASSWORD).await;
    let created = s.create(&admin, "http-target").await;
    let id = created["user"]["id"].as_i64().unwrap();
    let base = format!("/api/v1/admin/users/{id}");
    let restricted = s
        .login(
            "http-target",
            created["temporary_password"].as_str().unwrap(),
        )
        .await;
    assert_eq!(
        s.call(
            Method::PUT,
            "/api/v1/auth/password",
            Some(&restricted),
            json!({"new_password":format!("{:x}",md5::compute(PASSWORD))})
        )
        .await
        .0,
        StatusCode::OK
    );
    let cookie1 = s.login("http-target", PASSWORD).await;
    let cookie2 = s.login("http-target", PASSWORD).await;
    let before = crate::db::user_management::get_user_record(s.db.orm_db(), id as i32)
        .await
        .unwrap()
        .unwrap();
    assert_eq!(
        s.call(
            Method::POST,
            &format!("{base}/revoke-sessions"),
            Some(&admin),
            json!({})
        )
        .await
        .0,
        StatusCode::OK
    );
    let after = crate::db::user_management::get_user_record(s.db.orm_db(), id as i32)
        .await
        .unwrap()
        .unwrap();
    assert_eq!(before.user, after.user);
    assert_eq!(before.role, after.role);
    assert_eq!(
        before.access.web_login_enabled,
        after.access.web_login_enabled
    );
    assert_eq!(before.access.auth_version + 1, after.access.auth_version);
    for cookie in [&cookie1, &cookie2] {
        assert_eq!(
            s.call(Method::GET, "/api/v1/auth/me", Some(cookie), json!({}))
                .await
                .0,
            StatusCode::UNAUTHORIZED
        )
    }
    let cookie = s.login("http-target", PASSWORD).await;
    for enabled in [false, true] {
        assert_eq!(
            s.call(
                Method::PUT,
                &format!("{base}/access"),
                Some(&admin),
                json!({"web_login_enabled":enabled})
            )
            .await
            .0,
            StatusCode::OK
        )
    }
    assert_eq!(
        s.call(Method::GET, "/api/v1/auth/me", Some(&cookie), json!({}))
            .await
            .0,
        StatusCode::UNAUTHORIZED
    );
    let reset = s
        .call(
            Method::POST,
            &format!("{base}/password-reset"),
            Some(&admin),
            json!({}),
        )
        .await;
    assert_eq!(reset.0, StatusCode::OK);
    assert_eq!(
        s.call(
            Method::POST,
            "/api/v1/auth/login",
            None,
            json!({"username":"http-target","password":format!("{:x}",md5::compute(PASSWORD))})
        )
        .await
        .0,
        StatusCode::UNAUTHORIZED
    );
    let _temporary = s
        .login(
            "http-target",
            reset.1["temporary_password"].as_str().unwrap(),
        )
        .await;
    assert_eq!(
        s.call(
            Method::PUT,
            &format!("{base}/role"),
            Some(&admin),
            json!({"role":"admin"})
        )
        .await
        .0,
        StatusCode::OK
    );
    let list = s
        .call(
            Method::GET,
            "/api/v1/admin/users?search=HTTP-TARGET&role=admin&web_login_enabled=true&page_size=1",
            Some(&admin),
            json!({}),
        )
        .await;
    assert_eq!(list.0, StatusCode::OK);
    assert_eq!(list.1["total"], 1);
    assert_eq!(list.1["items"][0]["id"], id);
    let empty = s
        .call(
            Method::GET,
            "/api/v1/admin/users?search=http-target&page=2&page_size=1",
            Some(&admin),
            json!({}),
        )
        .await
        .1;
    assert_eq!(empty["total"], 1);
    assert_eq!(empty["items"], json!([]));
    let audit = s
        .call(
            Method::GET,
            &format!("/api/v1/admin/user-events?target_id={id}"),
            Some(&admin),
            json!({}),
        )
        .await
        .1;
    for action in [
        "create",
        "access",
        "password_reset",
        "role",
        "revoke_sessions",
    ] {
        assert!(
            audit["items"]
                .as_array()
                .unwrap()
                .iter()
                .any(|event| event["action"] == action)
        )
    }
    let serialized = serde_json::to_string(&audit).unwrap();
    assert!(!serialized.contains(created["temporary_password"].as_str().unwrap()));
    assert!(!serialized.contains(&after.user.password));
    assert!(list.1["items"][0].get("password").is_none());
    assert!(list.1["items"][0].get("auth_version").is_none());
    let (a, b) = tokio::join!(
        s.call(
            Method::POST,
            "/api/v1/admin/users",
            Some(&admin),
            json!({"username":"duplicate-http"})
        ),
        s.call(
            Method::POST,
            "/api/v1/admin/users",
            Some(&admin),
            json!({"username":"duplicate-http"})
        )
    );
    let statuses = [a.0, b.0];
    assert!(statuses.contains(&StatusCode::CREATED));
    assert!(statuses.contains(&StatusCode::CONFLICT));
    let b = s
        .service
        .recover_admin("http-second-admin", PASSWORD)
        .await
        .unwrap();
    let bc = s.login("http-second-admin", PASSWORD).await;
    let other_path = format!("/api/v1/admin/users/{}/role", b.id);
    let own_path = format!("/api/v1/admin/users/{}/role", s.admin);
    let (a, b) = tokio::join!(
        s.call(
            Method::PUT,
            &other_path,
            Some(&admin),
            json!({"role":"user"})
        ),
        s.call(Method::PUT, &own_path, Some(&bc), json!({"role":"user"}))
    );
    assert_eq!(
        [a.0, b.0].iter().filter(|s| **s == StatusCode::OK).count(),
        1
    );
    assert!(
        [a.0, b.0]
            .iter()
            .any(|s| matches!(*s, StatusCode::FORBIDDEN | StatusCode::UNAUTHORIZED))
    );
}

#[tokio::test]
async fn admin_http_origin_policy_cookie_and_legacy_compatibility() {
    let s = Server::new(None).await;
    let admin = s.login("http-operator", PASSWORD).await;
    for method in [Method::GET, Method::POST, Method::OPTIONS] {
        let res = s
            .request(
                method,
                "/api/v1/admin/users",
                Some(&admin),
                json!({"username":"must-not-create"}),
            )
            .header(header::ORIGIN, "https://cross-site.invalid")
            .header("Access-Control-Request-Method", "POST")
            .header(
                "Access-Control-Request-Headers",
                "x-requested-with,content-type",
            )
            .send()
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::FORBIDDEN);
        assert!(
            res.headers()
                .get(header::ACCESS_CONTROL_ALLOW_ORIGIN)
                .is_none()
        );
    }
    for origin in ["null", "http://127.0.0.1:1"] {
        assert_eq!(
            s.request(
                Method::POST,
                "/api/v1/admin/users",
                Some(&admin),
                json!({"username":"must-not-create"})
            )
            .header(header::ORIGIN, origin)
            .send()
            .await
            .unwrap()
            .status(),
            StatusCode::FORBIDDEN
        )
    }
    assert_eq!(
        s.request(Method::GET, "/api/v1/admin/users", Some(&admin), json!({}))
            .header(header::HOST, "non-loopback.invalid")
            .send()
            .await
            .unwrap()
            .status(),
        StatusCode::FORBIDDEN
    );
    let no_header = s
        .client
        .post(format!("{}/api/v1/admin/users", s.base))
        .header(header::COOKIE, &admin)
        .json(&json!({"username":"must-not-create"}))
        .send()
        .await
        .unwrap();
    assert_eq!(no_header.status(), StatusCode::FORBIDDEN);
    assert_eq!(
        s.request(
            Method::POST,
            "/api/v1/admin/users",
            Some(&admin),
            json!({"username":"same-origin-test"})
        )
        .header(header::ORIGIN, &s.base)
        .send()
        .await
        .unwrap()
        .status(),
        StatusCode::CREATED
    );
    let legacy = s
        .request(
            Method::POST,
            "/api/v1/auth/login",
            None,
            json!({"username":"http-operator","password":format!("{:x}",md5::compute(PASSWORD))}),
        )
        .header(header::ORIGIN, "https://legacy-ui.invalid")
        .send()
        .await
        .unwrap();
    assert_eq!(legacy.status(), StatusCode::OK);
    let configured = Server::new(Some("https://console.example.invalid")).await;
    let res = configured
        .request(
            Method::POST,
            "/api/v1/auth/login",
            None,
            json!({"username":"http-operator","password":format!("{:x}",md5::compute(PASSWORD))}),
        )
        .send()
        .await
        .unwrap();
    assert!(
        res.headers()[header::SET_COOKIE]
            .to_str()
            .unwrap()
            .contains("Secure")
    );
    let cookie = res.headers()[header::SET_COOKIE]
        .to_str()
        .unwrap()
        .split(';')
        .next()
        .unwrap();
    assert_eq!(
        configured
            .request(Method::GET, "/api/v1/admin/users", Some(cookie), json!({}))
            .header(header::ORIGIN, "https://console.example.invalid")
            .send()
            .await
            .unwrap()
            .status(),
        StatusCode::OK
    );
    for invalid in [
        "not a URL",
        "http://public.invalid",
        "https://user:pass@host.invalid",
        "https://host.invalid/path",
        "https://host.invalid?query=x",
        "https://host.invalid#fragment",
    ] {
        assert!(BrowserSecurity::from_origin(Some(invalid)).is_err())
    }
}
