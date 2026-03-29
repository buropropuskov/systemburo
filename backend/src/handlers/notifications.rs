use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;

use crate::models::notifications::{Notification, MarkReadRequest, CreateNotificationRequest};
use crate::auth::decode_token;

pub async fn get_notifications(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let user = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        claims.sub
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user {
        Some(u) => u.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let notifications = sqlx::query_as!(
        Notification,
        r#"
        SELECT 
            id,
            user_id,
            type as type_,
            title,
            message,
            data,
            is_read,
            created_at
        FROM notifications
        WHERE user_id = $1
        ORDER BY created_at DESC
        "#,
        user_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch notifications: {}", e);
        error::ErrorInternalServerError("Error fetching notifications")
    })?;

    Ok(HttpResponse::Ok().json(notifications))
}

pub async fn mark_notification_read(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<MarkReadRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let user = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        claims.sub
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user {
        Some(u) => u.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let notification_id = path.into_inner();

    let owned = sqlx::query!(
        "SELECT user_id FROM notifications WHERE id = $1",
        notification_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check notification ownership: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    match owned {
        Some(row) if row.user_id == user_id => {
            sqlx::query!(
                "UPDATE notifications SET is_read = $1 WHERE id = $2",
                form.is_read,
                notification_id
            )
            .execute(pool.get_ref())
            .await
            .map_err(|e| {
                log::error!("Failed to update notification: {}", e);
                error::ErrorInternalServerError("Error updating notification")
            })?;

            Ok(HttpResponse::Ok().json(json!({
                "success": true,
                "message": "Notification updated"
            })))
        }
        Some(_) => Err(error::ErrorForbidden("Not your notification")),
        None => Err(error::ErrorNotFound("Notification not found")),
    }
}

pub async fn delete_notification(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let user = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        claims.sub
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user {
        Some(u) => u.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let notification_id = path.into_inner();

    let owned = sqlx::query!(
        "SELECT user_id FROM notifications WHERE id = $1",
        notification_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check notification ownership: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    match owned {
        Some(row) if row.user_id == user_id => {
            sqlx::query!("DELETE FROM notifications WHERE id = $1", notification_id)
                .execute(pool.get_ref())
                .await
                .map_err(|e| {
                    log::error!("Failed to delete notification: {}", e);
                    error::ErrorInternalServerError("Error deleting notification")
                })?;

            Ok(HttpResponse::Ok().json(json!({
                "success": true,
                "message": "Notification deleted"
            })))
        }
        Some(_) => Err(error::ErrorForbidden("Not your notification")),
        None => Err(error::ErrorNotFound("Notification not found")),
    }
}

pub async fn clear_all_notifications(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let user = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        claims.sub
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user {
        Some(u) => u.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    sqlx::query!("DELETE FROM notifications WHERE user_id = $1", user_id)
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to clear notifications: {}", e);
            error::ErrorInternalServerError("Error clearing notifications")
        })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "All notifications cleared"
    })))
}

pub async fn create_notification(
    pool: &PgPool,
    req: CreateNotificationRequest,
) -> Result<(), Error> {
    sqlx::query!(
        r#"
        INSERT INTO notifications (user_id, type, title, message, data, created_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
        "#,
        req.user_id,
        req.type_,
        req.title,
        req.message,
        req.data,
    )
    .execute(pool)
    .await
    .map_err(|e| {
        log::error!("Failed to create notification: {}", e);
        error::ErrorInternalServerError("Error creating notification")
    })?;

    Ok(())
}