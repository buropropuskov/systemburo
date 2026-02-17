use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;
use chrono::{DateTime, Utc};
use serde::{Serialize, Deserialize};

use crate::auth::decode_token;

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationHistoryItem {
    pub id: i32,
    pub application_id: i32,
    pub user_id: i32,
    pub user_name: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub action_type: String,
    pub action_status: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub created_at: DateTime<Utc>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct AddHistoryRequest {
    pub application_id: i32,
    pub user_id: i32,
    pub action_type: String,
    pub action_status: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct RevokeApprovalRequest {
    pub comment: Option<String>,
}

/// Получение истории заявки
pub async fn get_application_history(
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

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let application_id = path.into_inner();

    log::info!("Getting history for application: {}", application_id);

    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.application_id,
            h.user_id,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || u.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || u.middle_name
                    ELSE ''
                END
            ) as "user_name!",
            u.last_name,
            u.first_name,
            u.middle_name,
            h.action_type as "action_type!",
            h.action_status,
            h.old_value,
            h.new_value,
            h.comment,
            h.created_at as "created_at!",
            h.metadata as "metadata?"
        FROM application_history h
        JOIN users u ON h.user_id = u.id
        WHERE h.application_id = $1
        ORDER BY h.created_at DESC
        "#,
        application_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application history: {}", e);
        error::ErrorInternalServerError("Error fetching application history")
    })?;

    log::info!("Found {} history items", history.len());

    let items: Vec<ApplicationHistoryItem> = history.into_iter().map(|row| {
        ApplicationHistoryItem {
            id: row.id,
            application_id: row.application_id,
            user_id: row.user_id,
            user_name: row.user_name,
            last_name: row.last_name,
            first_name: row.first_name,
            middle_name: row.middle_name,
            action_type: row.action_type,
            action_status: row.action_status,
            old_value: row.old_value,
            new_value: row.new_value,
            comment: row.comment,
            created_at: row.created_at,
            metadata: row.metadata,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Добавление записи в историю (для ручного добавления)
pub async fn add_history_entry(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<AddHistoryRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Adding history entry for application {} by user {}", form.application_id, form.user_id);

    sqlx::query!(
        r#"
        INSERT INTO application_history (
            application_id,
            user_id,
            action_type,
            action_status,
            old_value,
            new_value,
            comment,
            metadata
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        "#,
        form.application_id,
        form.user_id,
        form.action_type,
        form.action_status,
        form.old_value,
        form.new_value,
        form.comment,
        form.metadata
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to add history entry: {}", e);
        error::ErrorInternalServerError("Error adding history entry")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "History entry added successfully"
    })))
}

/// Отзыв согласования
pub async fn revoke_approval(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<RevokeApprovalRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let username = &claims.sub;
    
    // Получаем ID текущего пользователя
    let user_row = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let current_user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let application_id = path.into_inner();

    log::info!("User {} revoking approval for application {}", current_user_id, application_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Проверяем, что пользователь является ответственным для этой заявки
    let responsible = sqlx::query!(
        r#"
        SELECT approval_status, required_approval
        FROM application_responsible_users
        WHERE application_id = $1 AND user_id = $2
        "#,
        application_id,
        current_user_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch responsible user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let responsible = match responsible {
        Some(r) => r,
        None => return Err(error::ErrorForbidden("You are not responsible for this application")),
    };

    // Проверяем, что пользователь уже проголосовал
    if responsible.approval_status == Some("pending".to_string()) || responsible.approval_status.is_none() {
        return Err(error::ErrorBadRequest("You haven't voted yet"));
    }

    // Получаем текущий confirmation до отзыва
    let old_confirmation = sqlx::query!(
        "SELECT confirmation FROM applications WHERE id = $1",
        application_id
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch confirmation: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Сохраняем старый статус для истории
    let old_status = responsible.approval_status.clone();

    // Отзываем согласование (возвращаем в pending) и очищаем комментарий
    sqlx::query!(
        r#"
        UPDATE application_responsible_users 
        SET approval_status = 'pending',
            approval_comment = NULL,
            approval_datetime = NULL
        WHERE application_id = $1 AND user_id = $2
        "#,
        application_id,
        current_user_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to revoke approval: {}", e);
        error::ErrorInternalServerError("Error revoking approval")
    })?;

    // Обновляем общий статус заявки на основе новых правил
    use crate::handlers::applications::update_application_confirmation_based_on_approvals;
    update_application_confirmation_based_on_approvals(&mut transaction, application_id).await?;

    // Получаем новый confirmation после отзыва
    let new_confirmation = sqlx::query!(
        "SELECT confirmation FROM applications WHERE id = $1",
        application_id
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch new confirmation: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Добавляем запись в историю об отзыве согласования с комментарием
    sqlx::query!(
        r#"
        INSERT INTO application_history (
            application_id,
            user_id,
            action_type,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, NOW())
        "#,
        application_id,
        current_user_id,
        "revoke_approval",
        form.comment // Сохраняем комментарий
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add history entry: {}", e);
        error::ErrorInternalServerError("Error adding history entry")
    })?;

    // Если confirmation изменился, добавляем запись об изменении статуса
    if old_confirmation.confirmation != new_confirmation.confirmation {
        sqlx::query!(
            r#"
            INSERT INTO application_history (
                application_id,
                user_id,
                action_type,
                old_value,
                new_value,
                created_at
            )
            VALUES ($1, $2, $3, $4, $5, NOW() + INTERVAL '1 millisecond')
            "#,
            application_id,
            current_user_id,
            "confirmation_change",
            old_confirmation.confirmation,
            new_confirmation.confirmation
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to add confirmation change history: {}", e);
            error::ErrorInternalServerError("Error adding history")
        })?;
    }

    // Фиксируем транзакцию
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    // Получаем обновленный статус заявки для ответа
    let updated_application = sqlx::query!(
        "SELECT confirmation, status FROM applications WHERE id = $1",
        application_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch updated application: {}", e);
        error::ErrorInternalServerError("Error fetching updated application")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Approval revoked successfully",
        "confirmation": updated_application.confirmation,
        "status": updated_application.status
    })))
}