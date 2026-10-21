//! HTTP boundary for account management. Policy and transactions live in the service.
use super::{AppStateInner, HttpHandleError, auth::auth_error, users::AuthSession};
use crate::{
    db::user_management::has_manage_permission,
    user_management::{ManagementError, UserListFilter, UserManagementService},
};
use axum::{
    Extension, Json, Router,
    extract::{
        Path, Query, Request,
        rejection::{JsonRejection, PathRejection, QueryRejection},
    },
    http::StatusCode,
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::{get, post, put},
};
use axum_login::AuthUser;
use serde::Deserialize;

pub(super) fn router() -> Router<AppStateInner> {
    Router::new()
        .route("/api/v1/admin/users", get(list).post(create))
        .route("/api/v1/admin/users/{id}", get(detail))
        .route("/api/v1/admin/users/{id}/access", put(access))
        .route("/api/v1/admin/users/{id}/role", put(role))
        .route("/api/v1/admin/users/{id}/password-reset", post(reset))
        .route("/api/v1/admin/users/{id}/revoke-sessions", post(revoke))
        .route("/api/v1/admin/user-events", get(events))
        .route_layer(middleware::from_fn(authorize))
}
async fn authorize(auth: AuthSession, request: Request, next: Next) -> Response {
    let Some(user) = auth.user.as_ref() else {
        return auth_error(StatusCode::UNAUTHORIZED, "unauthenticated").into_response();
    };
    if user.is_restricted() {
        return auth_error(StatusCode::FORBIDDEN, "password_change_required").into_response();
    }
    match has_manage_permission(auth.backend.db().orm_db(), user.id()).await {
        Ok(true) => next.run(request).await,
        Ok(false) => auth_error(StatusCode::FORBIDDEN, "forbidden").into_response(),
        Err(_) => auth_error(StatusCode::INTERNAL_SERVER_ERROR, "internal_error").into_response(),
    }
}
fn map_error(error: ManagementError) -> HttpHandleError {
    use ManagementError::*;
    let status = match error {
        InvalidRequest => StatusCode::BAD_REQUEST,
        Unauthenticated => StatusCode::UNAUTHORIZED,
        Forbidden | PasswordChangeRequired | SelfActionForbidden | LocalPasswordUnavailable => {
            StatusCode::FORBIDDEN
        }
        UserNotFound => StatusCode::NOT_FOUND,
        UsernameConflict | LastAdmin | IdentityConflict => StatusCode::CONFLICT,
        InternalError => StatusCode::INTERNAL_SERVER_ERROR,
    };
    auth_error(status, &error.code())
}
fn invalid() -> HttpHandleError {
    auth_error(StatusCode::BAD_REQUEST, "invalid_request")
}
fn actor(auth: &AuthSession) -> Result<i32, HttpHandleError> {
    auth.user
        .as_ref()
        .map(AuthUser::id)
        .ok_or_else(|| auth_error(StatusCode::UNAUTHORIZED, "unauthenticated"))
}
fn target(path: Result<Path<i32>, PathRejection>) -> Result<i32, HttpHandleError> {
    let Path(id) = path.map_err(|_| invalid())?;
    if id <= 0 {
        return Err(invalid());
    }
    Ok(id)
}
fn body<T>(json: Result<Json<T>, JsonRejection>) -> Result<T, HttpHandleError> {
    json.map(|Json(value)| value).map_err(|_| invalid())
}
#[derive(Deserialize, Default)]
struct ListQuery {
    search: Option<String>,
    role: Option<String>,
    web_login_enabled: Option<bool>,
    page: Option<u64>,
    page_size: Option<u64>,
}
#[derive(Deserialize, Default)]
struct EventQuery {
    target_id: Option<i32>,
    page: Option<u64>,
    page_size: Option<u64>,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Create {
    username: String,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Access {
    web_login_enabled: bool,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Role {
    role: String,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Empty {}
async fn list(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    query: Result<Query<ListQuery>, QueryRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    let Query(q) = query.map_err(|_| invalid())?;
    Ok(Json(
        service
            .list_users(
                actor(&auth)?,
                UserListFilter {
                    search: q.search,
                    role: q.role,
                    web_login_enabled: q.web_login_enabled,
                    page: q.page.unwrap_or(1),
                    page_size: q.page_size.unwrap_or(20),
                },
            )
            .await
            .map_err(map_error)?,
    ))
}
async fn detail(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    path: Result<Path<i32>, PathRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    Ok(Json(
        service
            .get_user(actor(&auth)?, target(path)?)
            .await
            .map_err(map_error)?,
    ))
}
async fn create(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    json: Result<Json<Create>, JsonRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    let req = body(json)?;
    Ok((
        StatusCode::CREATED,
        Json(
            service
                .create_user(actor(&auth)?, &req.username)
                .await
                .map_err(map_error)?,
        ),
    ))
}
async fn access(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    path: Result<Path<i32>, PathRejection>,
    json: Result<Json<Access>, JsonRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    let req = body(json)?;
    Ok(Json(
        service
            .set_access(actor(&auth)?, target(path)?, req.web_login_enabled)
            .await
            .map_err(map_error)?,
    ))
}
async fn role(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    path: Result<Path<i32>, PathRejection>,
    json: Result<Json<Role>, JsonRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    let req = body(json)?;
    Ok(Json(
        service
            .set_role(actor(&auth)?, target(path)?, &req.role)
            .await
            .map_err(map_error)?,
    ))
}
async fn reset(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    path: Result<Path<i32>, PathRejection>,
    json: Result<Json<Empty>, JsonRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    body(json)?;
    Ok(Json(
        service
            .reset_password(actor(&auth)?, target(path)?)
            .await
            .map_err(map_error)?,
    ))
}
async fn revoke(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    path: Result<Path<i32>, PathRejection>,
    json: Result<Json<Empty>, JsonRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    body(json)?;
    Ok(Json(
        service
            .revoke_sessions(actor(&auth)?, target(path)?)
            .await
            .map_err(map_error)?,
    ))
}
async fn events(
    auth: AuthSession,
    Extension(service): Extension<UserManagementService>,
    query: Result<Query<EventQuery>, QueryRejection>,
) -> Result<impl IntoResponse, HttpHandleError> {
    let Query(q) = query.map_err(|_| invalid())?;
    if q.target_id.is_some_and(|id| id <= 0) {
        return Err(invalid());
    }
    Ok(Json(
        service
            .list_events(
                actor(&auth)?,
                q.target_id,
                q.page.unwrap_or(1),
                q.page_size.unwrap_or(20),
            )
            .await
            .map_err(map_error)?,
    ))
}
#[cfg(test)]
mod tests;
