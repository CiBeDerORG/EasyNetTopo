use axum::{
    Router,
    http::StatusCode,
    routing::{get, post, put},
};
use axum_login::login_required;
use axum_messages::Message;
use serde::{Deserialize, Serialize};

use crate::restful::users::Backend;

use std::sync::Arc;

use crate::FeatureFlags;

use super::{
    AppStateInner,
    users::{AuthSession, Credentials},
};

#[derive(Debug, Deserialize, Serialize)]
pub struct LoginResult {
    messages: Vec<Message>,
}

pub fn router() -> Router<AppStateInner> {
    let r = Router::new()
        .route("/api/v1/auth/password", put(self::put::change_password))
        .route("/api/v1/auth/me", get(self::get::me))
        .route(
            "/api/v1/auth/check_login_status",
            get(self::get::check_login_status),
        )
        .route_layer(login_required!(Backend));
    Router::new()
        .merge(r)
        .route("/api/v1/auth/policy", get(self::get::policy))
        .route("/api/v1/auth/login", post(self::post::login))
        .route("/api/v1/auth/logout", get(self::get::logout))
        .route("/api/v1/auth/captcha", get(self::get::get_captcha))
        .route("/api/v1/auth/register", post(self::post::register))
}

/// Stable, non-sensitive error boundary shared with the OIDC callback.
pub(crate) fn auth_error(status: StatusCode, code: &str) -> super::HttpHandleError {
    (
        status,
        axum::Json(super::Error {
            message: code.into(),
            code: Some(code.into()),
            current_config_revision: None,
        }),
    )
}

pub(crate) fn authentication_error(error: super::users::Error) -> super::HttpHandleError {
    use super::users::Error;
    match error {
        Error::InvalidRequest => auth_error(StatusCode::BAD_REQUEST, "invalid_request"),
        Error::Unavailable => auth_error(StatusCode::UNAUTHORIZED, "unauthenticated"),
        Error::IdentityConflict => auth_error(StatusCode::CONFLICT, "identity_conflict"),
        Error::LocalPasswordUnavailable => {
            auth_error(StatusCode::FORBIDDEN, "local_password_unavailable")
        }
        Error::Sqlx(_) | Error::TaskJoin(_) => {
            auth_error(StatusCode::INTERNAL_SERVER_ERROR, "internal_error")
        }
    }
}

/// Install inside AuthManagerLayer on the combined browser API router (WUM-006).
/// A temporary-credential session is deliberately unable to reach any workspace
/// route, including public config parsers, until the password is replaced.
pub(crate) async fn require_password_change(
    auth: AuthSession,
    request: axum::extract::Request,
    next: axum::middleware::Next,
) -> axum::response::Response {
    use axum::response::IntoResponse;
    let allowed = matches!(
        (request.method().as_str(), request.uri().path()),
        (
            "GET",
            "/api/v1/auth/me" | "/api/v1/auth/check_login_status" | "/api/v1/auth/logout"
        ) | ("PUT", "/api/v1/auth/password")
    );
    if auth.user.as_ref().is_some_and(|user| user.is_restricted()) && !allowed {
        return auth_error(StatusCode::FORBIDDEN, "password_change_required").into_response();
    }
    next.run(request).await
}

#[derive(Serialize)]
struct CurrentUser {
    id: i32,
    username: String,
    role: String,
    web_login_enabled: bool,
    source: String,
    local_password_enabled: Option<bool>,
    must_change_password: bool,
    created_at: Option<chrono::DateTime<chrono::Utc>>,
    last_login_at: Option<chrono::DateTime<chrono::Utc>>,
    capabilities: Vec<String>,
    default_password_warning: bool,
}

fn uses_historical_default(hash: &str) -> bool {
    // These are the two already-public migration fingerprints. Never send them
    // or their corresponding default passwords to clients or logs.
    matches!(
        hash,
        "$argon2i$v=19$m=16,t=2,p=1$aGVyRDBrcnRycnlaMDhkbw$449SEcv/qXf+0fnI9+fYVQ"
            | "$argon2i$v=19$m=16,t=2,p=1$bW5idXl0cmY$61n+JxL4r3dwLPAEDlDdtg"
    )
}

mod put {
    use axum::Json;
    use axum_login::AuthUser;
    use easytier::proto::common::Void;

    use crate::restful::{HttpHandleError, users::ChangePassword};

    use super::*;

    pub async fn change_password(
        mut auth_session: AuthSession,
        Json(req): Json<ChangePassword>,
    ) -> Result<Json<Void>, HttpHandleError> {
        let user = auth_session
            .user
            .as_ref()
            .ok_or_else(|| auth_error(StatusCode::UNAUTHORIZED, "unauthenticated"))?;
        auth_session
            .backend
            .change_password(user.id(), &req)
            .await
            .map_err(authentication_error)?;

        let _ = auth_session.logout().await;

        Ok(Void::default().into())
    }
}

mod post {
    use axum::{Json, extract::Extension};
    use easytier::proto::common::Void;

    use crate::restful::{
        HttpHandleError,
        captcha::extension::{CaptchaUtil, axum_tower_sessions::CaptchaAxumTowerSessionStaticExt},
        other_error,
        users::RegisterNewUser,
    };

    use super::*;

    pub async fn login(
        mut auth_session: AuthSession,
        Json(creds): Json<Credentials>,
    ) -> Result<Json<Void>, HttpHandleError> {
        let user = match auth_session.authenticate(creds.clone()).await {
            Ok(Some(user)) => user,
            Ok(None) => {
                return Err(auth_error(StatusCode::UNAUTHORIZED, "invalid_credentials"));
            }
            Err(_) => {
                return Err(auth_error(
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "internal_error",
                ));
            }
        };

        auth_session
            .login(&user)
            .await
            .map_err(|_| auth_error(StatusCode::INTERNAL_SERVER_ERROR, "internal_error"))?;

        Ok(Void::default().into())
    }

    pub async fn register(
        Extension(feature_flags): Extension<Arc<FeatureFlags>>,
        auth_session: AuthSession,
        captcha_session: tower_sessions::Session,
        Json(req): Json<RegisterNewUser>,
    ) -> Result<Json<Void>, HttpHandleError> {
        // Check if registration is disabled
        if feature_flags.disable_registration {
            tracing::warn!("Registration attempt blocked: registration is disabled");
            return Err((
                StatusCode::FORBIDDEN,
                other_error("Registration is disabled").into(),
            ));
        }

        // 调用CaptchaUtil的静态方法验证验证码是否正确
        if !CaptchaUtil::ver(&req.captcha, &captcha_session).await {
            return Err((
                StatusCode::BAD_REQUEST,
                other_error(format!("captcha verify error, input: {}", req.captcha)).into(),
            ));
        }

        if let Err(e) = auth_session.backend.register_new_user(&req).await {
            return Err(authentication_error(e));
        }

        Ok(Void::default().into())
    }
}

mod get {
    use crate::restful::{
        HttpHandleError,
        captcha::{
            NewCaptcha as _,
            builder::spec::SpecCaptcha,
            extension::{CaptchaUtil, axum_tower_sessions::CaptchaAxumTowerSessionExt as _},
        },
        other_error,
    };
    use axum::{Json, response::Response};
    use easytier::proto::common::Void;
    use tower_sessions::Session;

    use super::*;

    pub async fn me(auth: AuthSession) -> Result<Json<CurrentUser>, HttpHandleError> {
        use crate::db::user_management as accounts;
        use axum_login::AuthUser;
        let user = auth
            .user
            .as_ref()
            .ok_or_else(|| auth_error(StatusCode::UNAUTHORIZED, "unauthenticated"))?;
        let record = accounts::get_user_record(auth.backend.db().orm_db(), user.id())
            .await
            .map_err(|_| auth_error(StatusCode::INTERNAL_SERVER_ERROR, "internal_error"))?
            .ok_or_else(|| auth_error(StatusCode::UNAUTHORIZED, "unauthenticated"))?;
        let manages = !record.access.must_change_password
            && accounts::has_manage_permission(auth.backend.db().orm_db(), user.id())
                .await
                .map_err(|_| auth_error(StatusCode::INTERNAL_SERVER_ERROR, "internal_error"))?;
        Ok(Json(CurrentUser {
            id: record.user.id,
            default_password_warning: uses_historical_default(&record.user.password),
            username: record.user.username,
            role: record.role,
            web_login_enabled: record.access.web_login_enabled,
            source: record.access.source,
            local_password_enabled: record.access.local_password_enabled,
            must_change_password: record.access.must_change_password,
            created_at: record.access.created_at,
            last_login_at: record.access.last_login_at,
            capabilities: if manages {
                vec!["users.manage".into()]
            } else {
                vec![]
            },
        }))
    }

    pub async fn policy(
        axum::extract::Extension(flags): axum::extract::Extension<Arc<FeatureFlags>>,
        axum::extract::Extension(oidc): axum::extract::Extension<super::super::oidc::OidcConfig>,
    ) -> Json<serde_json::Value> {
        Json(
            serde_json::json!({ "registration_enabled": !flags.disable_registration, "oidc_enabled": oidc.enabled }),
        )
    }

    pub async fn logout(mut auth_session: AuthSession) -> Result<Json<Void>, HttpHandleError> {
        match auth_session.logout().await {
            Ok(_) => Ok(Json(Void::default())),
            Err(e) => {
                tracing::error!("Failed to logout: {:?}", e);
                Err((
                    StatusCode::INTERNAL_SERVER_ERROR,
                    Json::from(other_error(format!("{:?}", e))),
                ))
            }
        }
    }

    pub async fn get_captcha(session: Session) -> Result<Response, HttpHandleError> {
        let mut captcha: CaptchaUtil<SpecCaptcha> = CaptchaUtil::with_size_and_len(127, 48, 4);
        match captcha.out(&session).await {
            Ok(response) => Ok(response),
            Err(e) => Err((
                StatusCode::INTERNAL_SERVER_ERROR,
                Json::from(other_error(format!("{:?}", e))),
            )),
        }
    }

    pub async fn check_login_status(
        auth_session: AuthSession,
    ) -> Result<Json<Void>, HttpHandleError> {
        if auth_session.user.is_some() {
            Ok(Json(Void::default()))
        } else {
            Err((
                StatusCode::UNAUTHORIZED,
                Json::from(other_error("Not logged in")),
            ))
        }
    }
}

#[cfg(test)]
mod lifecycle_http_tests {
    use super::*;
    use crate::db::{Db, user_management as accounts};
    use axum::{
        body::{Body, to_bytes},
        http::{Request, header},
        middleware,
    };
    use axum_login::{AuthManagerLayerBuilder, AuthUser};
    use sea_orm::TransactionTrait;
    use tower::ServiceExt;
    use tower_sessions::{SessionManagerLayer, cookie::SameSite};
    use tower_sessions_sqlx_store::SqliteStore;

    async fn request(
        app: &Router,
        method: &str,
        path: &str,
        cookie: Option<&str>,
        body: serde_json::Value,
    ) -> axum::response::Response {
        let mut req = Request::builder()
            .method(method)
            .uri(path)
            .header(header::CONTENT_TYPE, "application/json");
        if let Some(cookie) = cookie {
            req = req.header(header::COOKIE, cookie);
        }
        app.clone()
            .oneshot(req.body(Body::from(body.to_string())).unwrap())
            .await
            .unwrap()
    }
    async fn login(app: &Router, name: &str, password: &str) -> String {
        let response = request(
            app,
            "POST",
            "/api/v1/auth/login",
            None,
            serde_json::json!({"username":name,"password":password}),
        )
        .await;
        assert_eq!(response.status(), StatusCode::OK);
        response
            .headers()
            .get(header::SET_COOKIE)
            .unwrap()
            .to_str()
            .unwrap()
            .split(';')
            .next()
            .unwrap()
            .to_owned()
    }

    #[tokio::test]
    async fn auth_lifecycle_real_sessions_revoke_restore_restricted_and_policy() {
        let db = Db::memory_db().await;
        let backend = Backend::new(db.clone());
        let digest = format!("{:x}", md5::compute("fictional-browser-password"));
        backend
            .register_new_user(&super::super::users::RegisterNewUser {
                credentials: Credentials {
                    username: "browser-auth-test".into(),
                    password: digest.clone(),
                },
                captcha: "test".into(),
            })
            .await
            .unwrap();
        let user = axum_login::AuthnBackend::authenticate(
            &backend,
            Credentials {
                username: "browser-auth-test".into(),
                password: digest.clone(),
            },
        )
        .await
        .unwrap()
        .unwrap();
        let store = SqliteStore::new(db.inner());
        store.migrate().await.unwrap();
        let session = SessionManagerLayer::new(store)
            .with_secure(false)
            .with_same_site(SameSite::Lax);
        let layer = AuthManagerLayerBuilder::new(backend.clone(), session).build();
        let app = Router::new()
            .route("/api/v1/auth/login", post(super::post::login))
            .route("/api/v1/auth/me", get(super::get::me))
            .route("/api/v1/auth/password", put(super::put::change_password))
            .route("/api/v1/auth/logout", get(super::get::logout))
            .route("/api/v1/auth/policy", get(super::get::policy))
            .route(
                "/workspace",
                get(|auth: AuthSession| async move {
                    if auth.user.is_some() {
                        StatusCode::OK
                    } else {
                        StatusCode::UNAUTHORIZED
                    }
                }),
            )
            .layer(middleware::from_fn(require_password_change))
            .layer(axum::extract::Extension(Arc::new(FeatureFlags::default())))
            .layer(axum::extract::Extension(
                super::super::oidc::OidcConfig::disabled(),
            ))
            .layer(layer);
        let cookie1 = login(&app, "browser-auth-test", &digest).await;
        let cookie2 = login(&app, "browser-auth-test", &digest).await;
        for c in [&cookie1, &cookie2] {
            assert_eq!(
                request(&app, "GET", "/workspace", Some(c), serde_json::json!({}))
                    .await
                    .status(),
                StatusCode::OK
            );
        }
        let tx = db.orm_db().begin().await.unwrap();
        accounts::lock_user_management(&tx).await.unwrap();
        let mut access = accounts::ensure_user_access(&tx, user.id()).await.unwrap();
        access.auth_version += 1;
        accounts::save_user_access(&tx, &access).await.unwrap();
        tx.commit().await.unwrap();
        for c in [&cookie1, &cookie2] {
            assert_eq!(
                request(&app, "GET", "/workspace", Some(c), serde_json::json!({}))
                    .await
                    .status(),
                StatusCode::UNAUTHORIZED
            );
        }
        let cookie = login(&app, "browser-auth-test", &digest).await;
        let tx = db.orm_db().begin().await.unwrap();
        accounts::lock_user_management(&tx).await.unwrap();
        access.must_change_password = true;
        access.temporary_password_expires_at =
            Some(chrono::Utc::now() + chrono::Duration::hours(1));
        access.auth_version += 1;
        accounts::save_user_access(&tx, &access).await.unwrap();
        tx.commit().await.unwrap();
        assert_eq!(
            request(
                &app,
                "GET",
                "/workspace",
                Some(&cookie),
                serde_json::json!({})
            )
            .await
            .status(),
            StatusCode::UNAUTHORIZED
        );
        let restricted = login(&app, "browser-auth-test", &digest).await;
        let denied = request(
            &app,
            "GET",
            "/workspace",
            Some(&restricted),
            serde_json::json!({}),
        )
        .await;
        assert_eq!(denied.status(), StatusCode::FORBIDDEN);
        let body: serde_json::Value =
            serde_json::from_slice(&to_bytes(denied.into_body(), 4096).await.unwrap()).unwrap();
        assert_eq!(body["code"], "password_change_required");
        let me = request(
            &app,
            "GET",
            "/api/v1/auth/me",
            Some(&restricted),
            serde_json::json!({}),
        )
        .await;
        assert_eq!(me.status(), StatusCode::OK);
        let value: serde_json::Value =
            serde_json::from_slice(&to_bytes(me.into_body(), 4096).await.unwrap()).unwrap();
        assert_eq!(value["must_change_password"], true);
        assert_eq!(value["capabilities"], serde_json::json!([]));
        assert!(value.get("password").is_none());
        assert!(value.get("auth_version").is_none());
        let changed=request(&app,"PUT","/api/v1/auth/password",Some(&restricted),serde_json::json!({"new_password":format!("{:x}",md5::compute("fictional-new-browser-password"))})).await;
        assert_eq!(changed.status(), StatusCode::OK);
        assert_eq!(
            request(
                &app,
                "GET",
                "/api/v1/auth/me",
                Some(&restricted),
                serde_json::json!({})
            )
            .await
            .status(),
            StatusCode::UNAUTHORIZED
        );
        let policy = request(
            &app,
            "GET",
            "/api/v1/auth/policy",
            None,
            serde_json::json!({}),
        )
        .await;
        assert_eq!(policy.status(), StatusCode::OK);
        let value: serde_json::Value =
            serde_json::from_slice(&to_bytes(policy.into_body(), 4096).await.unwrap()).unwrap();
        assert_eq!(
            value,
            serde_json::json!({"registration_enabled":true,"oidc_enabled":false})
        );
    }

    #[tokio::test]
    async fn auth_lifecycle_default_password_warning_preserves_login() {
        let db = Db::memory_db().await;
        let backend = Backend::new(db);
        let user = axum_login::AuthnBackend::authenticate(
            &backend,
            Credentials {
                username: "admin".into(),
                password: format!("{:x}", md5::compute("admin")),
            },
        )
        .await
        .unwrap()
        .unwrap();
        assert!(uses_historical_default(&user.db_user.password));
        assert!(!user.is_restricted());
        assert!(user.access.web_login_enabled);
        backend
            .change_password(
                user.id(),
                &super::super::users::ChangePassword {
                    new_password: format!(
                        "{:x}",
                        md5::compute("fictional-replaced-default-password")
                    ),
                },
            )
            .await
            .unwrap();
        let user = axum_login::AuthnBackend::get_user(&backend, &user.id())
            .await
            .unwrap()
            .unwrap();
        assert!(!uses_historical_default(&user.db_user.password));
    }

    #[tokio::test]
    async fn auth_lifecycle_registration_without_admin_does_not_promote() {
        let db = Db::memory_db().await;
        let pool = db.inner();
        sqlx::query("DELETE FROM users_groups WHERE group_id IN (SELECT id FROM groups WHERE name = 'admins')")
            .execute(&pool).await.unwrap();
        let admin_members: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM users_groups ug JOIN groups g ON g.id = ug.group_id WHERE g.name = 'admins'")
            .fetch_one(&pool).await.unwrap();
        assert_eq!(admin_members, 0);
        let backend = Backend::new(db.clone());
        let credentials = Credentials {
            username: "first-public-registration-test".into(),
            password: format!(
                "{:x}",
                md5::compute("fictional-public-registration-password")
            ),
        };
        backend
            .register_new_user(&super::super::users::RegisterNewUser {
                credentials: credentials.clone(),
                captcha: "isolated-test".into(),
            })
            .await
            .unwrap();
        let user = axum_login::AuthnBackend::authenticate(&backend, credentials)
            .await
            .unwrap()
            .expect("registered account can authenticate");
        let record = accounts::get_user_record(db.orm_db(), user.id())
            .await
            .unwrap()
            .unwrap();
        assert_eq!(record.role, "user");
        assert_eq!(record.access.source, "local");
        assert!(record.access.web_login_enabled);
        assert!(
            !accounts::has_manage_permission(db.orm_db(), user.id())
                .await
                .unwrap()
        );
        let admin_members: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM users_groups ug JOIN groups g ON g.id = ug.group_id WHERE g.name = 'admins'")
            .fetch_one(&pool).await.unwrap();
        assert_eq!(
            admin_members, 0,
            "registration must not bootstrap an administrator"
        );
    }
}
