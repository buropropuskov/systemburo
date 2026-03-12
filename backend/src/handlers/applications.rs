// handlers/applications.rs
use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};
use chrono::{DateTime, Utc, NaiveDateTime, NaiveDate, NaiveTime};

use crate::models::applications::{ApplicationWithDetails, ApplicationFilter, ApplicationCreateRequest, ApplicationUpdateRequest};
use crate::auth::decode_token;

use crate::models::applications::{ForwardApplicationRequest, ForwardUser};
use crate::models::application_viewers::ApplicationViewer;

// Структура для полной заявки с вложениями
#[derive(Debug, Deserialize)]
pub struct CompleteApplicationRequest {
    pub message: Option<String>,
    pub organization: String,
    pub company: Option<String>,
    pub responsible_person: String,
    pub contact_phone: String,
    pub data_approval: bool,
    pub attachments: Vec<AttachmentData>,
    pub required_users: Option<Vec<RequiredUser>>,
}

#[derive(Debug, Deserialize)]
pub struct AttachmentData {
    pub attachment_type: String,
    pub attachment_name: String,
    pub attachment_display_name: String,
    pub unique_attachment_id: i32,
    pub entry_date_from: Option<String>,
    pub entry_date_to: Option<String>,
    pub entry_time_from: Option<String>,
    pub entry_time_to: Option<String>,
    pub data: AttachmentContentData,
}

#[derive(Debug, Deserialize)]
pub struct AttachmentContentData {
    pub vehicles: Option<Vec<VehicleData>>,
    pub employees: Option<Vec<EmployeeData>>,
    pub items: Option<Vec<ItemData>>,
}

#[derive(Debug, Deserialize)]
pub struct RequiredUser {
    pub user_id: i32,
    pub required_approval: bool,
}

#[derive(Debug, Deserialize)]
pub struct VehicleData {
    pub car_number: String,
    pub car_brand: String,
    pub unload_place: Option<String>,
    pub unload_places: Vec<i32>,
}

#[derive(Debug, Deserialize)]
pub struct EmployeeData {
    pub last_name: String,
    pub first_name: String,
    pub middle_name: Option<String>,
    pub citizenship_id: i32,
    pub position: String,
    pub passport_series_number: String,
    pub patent_number: Option<String>,
    pub other_permission: Option<String>,
    pub target_tables: Vec<i32>,
}

#[derive(Debug, Deserialize)]
pub struct ItemData {
    pub name: String,
    pub count: i32,
    pub order_index: i32,
}

// Структура для ответа
#[derive(Debug, Serialize)]
pub struct CompleteApplicationResponse {
    pub success: bool,
    pub message: String,
    pub application_id: i32,
    pub application_number: String,
}

// Структура для принятия заявки в работу
#[derive(Debug, Deserialize)]
pub struct TakeToWorkRequest {
    pub user_id: i32,
    pub action: String, // 'accept' или 'reject'
    pub comment: Option<String>, // ДОБАВИТЬ
}

// Структура для отзыва заявки из работы
#[derive(Debug, Deserialize)]
pub struct RevokeFromWorkRequest {
    pub user_id: i32,
    pub comment: Option<String>, // ДОБАВИТЬ
}

/// Функция для согласования заявки отдельным пользователем
#[derive(Debug, Deserialize)]
pub struct UserApprovalRequest {
    pub user_id: i32,
    pub status: String, // 'approved' или 'rejected'
    pub comment: Option<String>,
}

/// Проверяет истекшие вложения и обновляет статусы
pub async fn check_expired_attachments(pool: &PgPool) -> Result<(), Error> {
    log::info!("Checking expired attachments...");

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Находим истекшие вложения
    let expired_attachments = sqlx::query!(
        r#"
        SELECT id, application_id
        FROM attachments
        WHERE status = 1
        AND (
            (entry_date_to IS NOT NULL AND entry_date_to < CURRENT_DATE)
            OR
            (entry_date_to IS NOT NULL AND entry_time_to IS NOT NULL 
             AND ((entry_date_to + entry_time_to) AT TIME ZONE 'Europe/Moscow') < CURRENT_TIMESTAMP)
        )
        "#,
    )
    .fetch_all(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch expired attachments: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if expired_attachments.is_empty() {
        log::info!("No expired attachments found.");
        return Ok(());
    }

    log::info!("Found {} expired attachments", expired_attachments.len());

    let attachment_ids: Vec<i32> = expired_attachments.iter().map(|a| a.id).collect();
    let application_ids: Vec<i32> = expired_attachments.iter().map(|a| a.application_id).collect();

    // Получаем список всех машин, которые будут деактивированы
    let cars_to_deactivate = sqlx::query!(
        r#"
        SELECT c.id, c.car_number, c.car_brand
        FROM cars c
        WHERE c.attachment_id = ANY($1)
        "#,
        &attachment_ids[..]
    )
    .fetch_all(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch cars to deactivate: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    log::info!("Found {} cars to deactivate", cars_to_deactivate.len());

    // Обновляем статус вложений
    sqlx::query!(
        "UPDATE attachments SET status = 0 WHERE id = ANY($1)",
        &attachment_ids[..]
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update attachments status: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Обновляем статус связанных машин
    sqlx::query!(
        "UPDATE cars SET status = 0 WHERE attachment_id = ANY($1)",
        &attachment_ids[..]
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update cars status: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Обновляем статус сотрудников
    sqlx::query!(
        "UPDATE employees SET status = 0 WHERE attachment_id = ANY($1)",
        &attachment_ids[..]
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update employees status: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Записываем в историю для каждой деактивированной машины
    for car in &cars_to_deactivate {
        sqlx::query!(
            r#"
            INSERT INTO cars_history (
                car_id,
                user_id,
                action_type,
                comment,
                created_at
            )
            VALUES ($1, $2, $3, $4, NOW())
            "#,
            car.id,
            None::<i32>,
            "deactivate",
            format!("Срок действия заявки на автомобиль {} {} истёк", car.car_number, car.car_brand)
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to add car history entry for expired car {}: {}", car.id, e);
            error::ErrorInternalServerError("Database error")
        })?;
    }

    // Для каждой заявки, у которой все вложения стали неактивными, обновляем статус на 'Завершено'
    for app_id in application_ids {
        let active_count = sqlx::query!(
            "SELECT COUNT(*) as count FROM attachments WHERE application_id = $1 AND status = 1",
            app_id
        )
        .fetch_one(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to count active attachments: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

        if active_count.count.unwrap_or(0) == 0 {
            sqlx::query!(
                "UPDATE applications SET status = 'Завершено' WHERE id = $1",
                app_id
            )
            .execute(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Failed to update application status to Completed: {}", e);
                error::ErrorInternalServerError("Database error")
            })?;
            log::info!("Updated application {} to status 'Завершено'", app_id);
        }
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    log::info!("Expired attachments check completed successfully.");
    Ok(())
}

/// Функция для согласования заявки отдельным пользователем
pub async fn approve_application_by_user(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<UserApprovalRequest>,
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
    let current_user_row = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let current_user_id = match current_user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    // Проверяем, что user_id соответствует текущему пользователю
    if form.user_id != current_user_id {
        return Err(error::ErrorForbidden("You can only approve for yourself"));
    }

    let application_id = path.into_inner();

    log::info!("User {} approving application {} with status {}", form.user_id, application_id, form.status);

    // Проверяем, что статус валидный
    if form.status != "approved" && form.status != "rejected" {
        return Err(error::ErrorBadRequest("Invalid status. Must be 'approved' or 'rejected'"));
    }

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Проверяем, что пользователь является ответственным для этой заявки
    let responsible = sqlx::query!(
        r#"
        SELECT id, approval_status, required_approval
        FROM application_responsible_users 
        WHERE application_id = $1 AND user_id = $2
        "#,
        application_id,
        form.user_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to check if user is responsible: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let responsible = match responsible {
        Some(r) => r,
        None => return Err(error::ErrorForbidden("You are not responsible for this application")),
    };

    // Проверяем, не голосовал ли уже пользователь
    if responsible.approval_status != Some("pending".to_string()) {
        return Err(error::ErrorBadRequest("You have already voted on this application"));
    }

    // Получаем текущий confirmation до обновления
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

    // Базовое время для операций
    let now_utc = Utc::now();
    let mut history_time = now_utc;

    // Обновляем статус согласования для пользователя
    sqlx::query!(
        r#"
        UPDATE application_responsible_users 
        SET approval_status = $1,
            approval_comment = $2,
            approval_datetime = $3
        WHERE application_id = $4 AND user_id = $5
        "#,
        form.status,
        form.comment,
        now_utc,
        application_id,
        form.user_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update user approval status: {}", e);
        error::ErrorInternalServerError("Error updating approval status")
    })?;

    // Записываем в историю действие пользователя (согласование/отказ)
    sqlx::query!(
        r#"
        INSERT INTO application_history (
            application_id,
            user_id,
            action_type,
            comment,
            metadata,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6)
        "#,
        application_id,
        current_user_id,
        if form.status == "approved" { "approve" } else { "reject" },
        form.comment,
        serde_json::json!({
            "required_approval": responsible.required_approval
        }),
        history_time
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add approval history: {}", e);
        error::ErrorInternalServerError("Error adding history")
    })?;

    // Обновляем общий статус заявки на основе новых правил
    if let Err(e) = update_application_confirmation_based_on_approvals(&mut transaction, application_id).await {
        log::error!("Failed to update application confirmation: {}", e);
        return Err(e);
    }

    // Получаем новый confirmation после обновления
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

    // Если confirmation изменился, записываем в историю (с временем на 1 мс позже)
    if old_confirmation.confirmation != new_confirmation.confirmation {
        let status_change_time = history_time + chrono::Duration::milliseconds(1);
        
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
            VALUES ($1, $2, $3, $4, $5, $6)
            "#,
            application_id,
            current_user_id,
            "confirmation_change",
            old_confirmation.confirmation,
            new_confirmation.confirmation,
            status_change_time
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

    log::info!("Successfully updated approval status for user {} in application {}", form.user_id, application_id);

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Approval status updated successfully"
    })))
}

/// Функция для проверки статуса согласования (НОВАЯ)
pub async fn check_approval_status(
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

    // Получаем текущий статус заявки
    let application = sqlx::query!(
        "SELECT confirmation, status FROM applications WHERE id = $1",
        application_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    match application {
        Some(app) => {
            Ok(HttpResponse::Ok().json(json!({
                "confirmation": app.confirmation,
                "status": app.status
            })))
        },
        None => Err(error::ErrorNotFound("Application not found"))
    }
}

/// Функция для принятия заявки в работу
pub async fn take_application_to_work(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<TakeToWorkRequest>,
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

    // Проверяем, что пользователь является принимающим
    let is_approver = sqlx::query!(
        "SELECT EXISTS(SELECT 1 FROM application_approvers WHERE user_id = $1) as exists",
        current_user_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check if user is approver: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if !is_approver.exists.unwrap_or(false) {
        return Err(error::ErrorForbidden("User is not an approver"));
    }

    let application_id = path.into_inner();
    let action = form.action.clone();

    log::info!("User {} taking application {} to work with action: {}", current_user_id, application_id, action);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем текущий статус заявки
    let application = sqlx::query!(
        "SELECT status FROM applications WHERE id = $1",
        application_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application = match application {
        Some(app) => app,
        None => return Err(error::ErrorNotFound("Application not found")),
    };

    let old_status = application.status.clone();

    if action == "accept" {
        // Принимаем заявку в работу
        if application.status == "В работе" {
            return Err(error::ErrorBadRequest("Application is already in work"));
        }

        // Обновляем статус заявки и сохраняем комментарий
        sqlx::query!(
            "UPDATE applications SET status = 'В работе', responsible_user_id = $1, responsible_comment = $2 WHERE id = $3",
            current_user_id,
            form.comment, // Сохраняем комментарий в responsible_comment
            application_id
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to update application status: {}", e);
            error::ErrorInternalServerError("Error updating application status")
        })?;

        // ПИШЕМ ИСТОРИЮ
        sqlx::query!(
            r#"
            INSERT INTO application_history (
                application_id,
                user_id,
                action_type,
                old_value,
                new_value,
                comment,
                created_at
            )
            VALUES ($1, $2, $3, $4, $5, $6, NOW())
            "#,
            application_id,
            current_user_id,
            "take_to_work",
            old_status,
            "В работе",
            form.comment // Сохраняем комментарий в историю
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to add history entry: {}", e);
            error::ErrorInternalServerError("Error adding history")
        })?;

        // Активируем все машины и сотрудники
        activate_application_items(&mut transaction, application_id, true).await?;

    } else if action == "reject" {
        // Отказываем заявку
        if application.status == "Отказано" {
            return Err(error::ErrorBadRequest("Application is already rejected"));
        }

        sqlx::query!(
            "UPDATE applications SET status = 'Отказано', responsible_user_id = $1, responsible_comment = $2 WHERE id = $3",
            current_user_id,
            form.comment, // Сохраняем комментарий в responsible_comment
            application_id
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to update application status: {}", e);
            error::ErrorInternalServerError("Error updating application status")
        })?;

        // ПИШЕМ ИСТОРИЮ
        sqlx::query!(
            r#"
            INSERT INTO application_history (
                application_id,
                user_id,
                action_type,
                old_value,
                new_value,
                comment,
                created_at
            )
            VALUES ($1, $2, $3, $4, $5, $6, NOW())
            "#,
            application_id,
            current_user_id,
            "reject",
            old_status,
            "Отказано",
            form.comment // Сохраняем комментарий в историю
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to add history entry: {}", e);
            error::ErrorInternalServerError("Error adding history")
        })?;

        // Деактивируем все машины и сотрудники
        activate_application_items(&mut transaction, application_id, false).await?;
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": if action == "accept" { "Application taken to work" } else { "Application rejected" }
    })))
}

/// Функция для отзыва заявки из работы
pub async fn revoke_application_from_work(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<RevokeFromWorkRequest>,
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

    // Проверяем, что пользователь является принимающим
    let is_approver = sqlx::query!(
        "SELECT EXISTS(SELECT 1 FROM application_approvers WHERE user_id = $1) as exists",
        current_user_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check if user is approver: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if !is_approver.exists.unwrap_or(false) {
        return Err(error::ErrorForbidden("Only approver can revoke the application"));
    }

    let application_id = path.into_inner();

    log::info!("Approver {} revoking application {} from work", current_user_id, application_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем текущий статус заявки
    let application = sqlx::query!(
        "SELECT status FROM applications WHERE id = $1",
        application_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application = match application {
        Some(app) => app,
        None => return Err(error::ErrorNotFound("Application not found")),
    };

    let old_status = application.status.clone();

    // Обновляем статус заявки и очищаем комментарий ответственного
    sqlx::query!(
        "UPDATE applications SET status = 'В обработке', responsible_user_id = NULL, responsible_comment = NULL WHERE id = $1",
        application_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update application status: {}", e);
        error::ErrorInternalServerError("Error updating application status")
    })?;

    // ПИШЕМ ИСТОРИЮ С КОММЕНТАРИЕМ
    sqlx::query!(
        r#"
        INSERT INTO application_history (
            application_id,
            user_id,
            action_type,
            old_value,
            new_value,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        "#,
        application_id,
        current_user_id,
        "revoke_from_work",
        old_status,
        "В обработке",
        form.comment // Сохраняем комментарий в историю
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add history entry: {}", e);
        error::ErrorInternalServerError("Error adding history")
    })?;

    // Деактивируем все машины и сотрудники
    activate_application_items(&mut transaction, application_id, false).await?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application revoked from work"
    })))
}

/// Функция для возврата заявки в работу
pub async fn restore_application_to_work(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<RevokeFromWorkRequest>,
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

    // Проверяем, что пользователь является принимающим
    let is_approver = sqlx::query!(
        "SELECT EXISTS(SELECT 1 FROM application_approvers WHERE user_id = $1) as exists",
        current_user_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check if user is approver: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if !is_approver.exists.unwrap_or(false) {
        return Err(error::ErrorForbidden("Only approver can restore the application"));
    }

    let application_id = path.into_inner();

    log::info!("Approver {} restoring application {} to work", current_user_id, application_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем текущий статус заявки
    let application = sqlx::query!(
        "SELECT status FROM applications WHERE id = $1",
        application_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application = match application {
        Some(app) => app,
        None => return Err(error::ErrorNotFound("Application not found")),
    };

    let old_status = application.status.clone();

    // Обновляем статус заявки и очищаем комментарий ответственного
    sqlx::query!(
        "UPDATE applications SET status = 'В обработке', responsible_user_id = NULL, responsible_comment = NULL WHERE id = $1",
        application_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update application status: {}", e);
        error::ErrorInternalServerError("Error updating application status")
    })?;

    // ПИШЕМ ИСТОРИЮ С КОММЕНТАРИЕМ
    sqlx::query!(
        r#"
        INSERT INTO application_history (
            application_id,
            user_id,
            action_type,
            old_value,
            new_value,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        "#,
        application_id,
        current_user_id,
        "restore_to_work",
        old_status,
        "В обработке",
        form.comment // Сохраняем комментарий в историю
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add history entry: {}", e);
        error::ErrorInternalServerError("Error adding history")
    })?;

    // Деактивируем все машины и сотрудники
    activate_application_items(&mut transaction, application_id, false).await?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application restored, ready to take to work"
    })))
}

/// Вспомогательная функция для активации/деактивации элементов заявки
async fn activate_application_items(
    transaction: &mut sqlx::Transaction<'_, sqlx::Postgres>,
    application_id: i32,
    activate: bool,
) -> Result<(), Error> {
    let new_status = if activate { 1 } else { 0 };
    
    // Получаем все вложения заявки
    let attachments = sqlx::query!(
        "SELECT id, attachment_type FROM attachments WHERE application_id = $1",
        application_id
    )
    .fetch_all(&mut **transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch attachments: {}", e);
        error::ErrorInternalServerError("Error fetching attachments")
    })?;

    for attachment in attachments {
        match attachment.attachment_type.as_str() {
            "cars" => {
                // Обновляем статусы машин
                sqlx::query!(
                    "UPDATE cars SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = $2",
                    new_status,
                    attachment.id
                )
                .execute(&mut **transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to update cars status: {}", e);
                    error::ErrorInternalServerError("Error updating cars status")
                })?;
                
                log::info!("Updated cars status to {} for attachment {}", new_status, attachment.id);
            },
            "people" => {
                // Обновляем статусы сотрудников
                sqlx::query!(
                    "UPDATE employees SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = $2",
                    new_status,
                    attachment.id
                )
                .execute(&mut **transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to update employees status: {}", e);
                    error::ErrorInternalServerError("Error updating employees status")
                })?;
                
                log::info!("Updated employees status to {} for attachment {}", new_status, attachment.id);
            },
            _ => {} // Для других типов вложений ничего не делаем
        }
    }

    Ok(())
}


pub async fn update_application_confirmation_based_on_approvals(
    transaction: &mut sqlx::Transaction<'_, sqlx::Postgres>,
    application_id: i32,
) -> Result<(), Error> {
    // Получаем информацию о всех ответственных
    let responsibles = sqlx::query!(
        r#"
        SELECT 
            user_id,
            required_approval,
            approval_status
        FROM application_responsible_users
        WHERE application_id = $1
        "#,
        application_id
    )
    .fetch_all(&mut **transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch responsible users: {}", e);
        error::ErrorInternalServerError("Error fetching responsible users")
    })?;

    if responsibles.is_empty() {
        return Ok(());
    }

    // Получаем текущий статус заявки
    let current_application = sqlx::query!(
        "SELECT confirmation FROM applications WHERE id = $1",
        application_id
    )
    .fetch_one(&mut **transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Error fetching application")
    })?;

    // Проверяем по новым правилам:
    // 1. Если хотя бы один обязательный ответственный отказал -> "Не согласовано"
    // 2. Если все обязательные ответственные согласовали -> "Согласовано"
    // 3. Если нет обязательных ответственных и хотя бы один обычный ответственный согласовал -> "Согласовано"
    // 4. Если нет обязательных ответственных и хотя бы один обычный ответственный отказал -> "Не согласовано"
    // 5. В остальных случаях -> "Согласование"

    let required_users = responsibles.iter()
        .filter(|r| r.required_approval)
        .collect::<Vec<_>>();
    
    let non_required_users = responsibles.iter()
        .filter(|r| !r.required_approval)
        .collect::<Vec<_>>();

    let mut new_confirmation = "Согласование".to_string();

    // Проверяем случай 1: обязательный ответственный отказал
    let has_required_rejected = required_users.iter()
        .any(|r| r.approval_status.as_deref() == Some("rejected"));
    
    if has_required_rejected {
        new_confirmation = "Не согласовано".to_string();
    } 
    // Проверяем случай 2: все обязательные ответственные согласовали
    else if !required_users.is_empty() {
        let all_required_approved = required_users.iter()
            .all(|r| r.approval_status.as_deref() == Some("approved"));
        
        if all_required_approved {
            new_confirmation = "Согласовано".to_string();
        }
    }
    // Проверяем случай 3 и 4: нет обязательных ответственных
    else if required_users.is_empty() && !non_required_users.is_empty() {
        let has_any_approved = non_required_users.iter()
            .any(|r| r.approval_status.as_deref() == Some("approved"));
        
        let has_any_rejected = non_required_users.iter()
            .any(|r| r.approval_status.as_deref() == Some("rejected"));
        
        if has_any_approved && !has_any_rejected {
            new_confirmation = "Согласовано".to_string();
        } else if has_any_rejected {
            new_confirmation = "Не согласовано".to_string();
        }
    }

    // Обновляем только confirmation, status НЕ ТРОГАЕМ
    sqlx::query(
        r#"
        UPDATE applications 
        SET confirmation = $1,
            confirmation_datetime = CASE 
                WHEN $1 != 'Согласование' AND confirmation_datetime IS NULL THEN NOW()
                ELSE confirmation_datetime
            END
        WHERE id = $2
        "#
    )
    .bind(&new_confirmation)
    .bind(application_id)
    .execute(&mut **transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update application confirmation: {}", e);
        error::ErrorInternalServerError("Error updating application confirmation")
    })?;

    Ok(())
}

/// Функция для пересылки заявки (обновленная)
pub async fn forward_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<ForwardApplicationRequest>,
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
    
    // Получаем ID и имя текущего пользователя
    let user_row = sqlx::query!(
        r#"SELECT id, 
            last_name,
            first_name,
            middle_name
        FROM users 
        WHERE username = $1"#,
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let (current_user_id, current_user_name) = match user_row {
        Some(row) => {
            let name = format!("{} {} {}", 
                row.last_name.as_deref().unwrap_or(""),
                row.first_name.as_deref().unwrap_or(""),
                row.middle_name.as_deref().unwrap_or("")
            ).trim().to_string();
            (row.id, name)
        },
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let application_id = path.into_inner();

    log::info!("Forwarding application {} by user {} ({})", 
        application_id, 
        current_user_id, 
        current_user_name
    );

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Проверяем существование заявки
    let application_exists = sqlx::query!(
        "SELECT EXISTS(SELECT 1 FROM applications WHERE id = $1) as exists",
        application_id
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to check application existence: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if !application_exists.exists.unwrap_or(false) {
        return Err(error::ErrorNotFound("Application not found"));
    }

    // Проверяем права пользователя
    let can_forward = sqlx::query!(
        r#"
        SELECT EXISTS(
            SELECT 1 FROM applications a
            WHERE a.id = $1 
            AND (a.sender_user_id = $2 
                 OR EXISTS(
                     SELECT 1 FROM application_responsible_users aru 
                     WHERE aru.application_id = a.id 
                     AND aru.user_id = $2
                 ))
        ) as can_forward
        "#,
        application_id,
        current_user_id
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to check forwarding permissions: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if !can_forward.can_forward.unwrap_or(false) {
        return Err(error::ErrorForbidden("You don't have permission to forward this application"));
    }

    // Получаем текущий confirmation до изменений
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

    // Базовое время для операций
    let base_time = Utc::now();
    let mut history_time = base_time;

    // Векторы для хранения данных о добавленных пользователях
    let mut added_responsible_users = Vec::new(); // (user_id, required_approval)
    let mut added_viewers = Vec::new();           // user_id

    // ЭТАП 1: Добавляем пользователей (только в БД)
    for forward_user in &form.users {
        // Проверяем существование пользователя
        let user_exists = sqlx::query!(
            "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1) as exists",
            forward_user.user_id
        )
        .fetch_one(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to check user existence: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

        if !user_exists.exists.unwrap_or(false) {
            log::warn!("User {} not found, skipping", forward_user.user_id);
            continue;
        }

        // Проверяем корректность параметров
        if forward_user.required_approval && forward_user.can_view {
            log::warn!("User {} cannot be both responsible and viewer, skipping", forward_user.user_id);
            continue;
        }

        log::info!("Processing user {}: required_approval={}, can_view={}", 
                   forward_user.user_id, forward_user.required_approval, forward_user.can_view);

        if forward_user.required_approval {
            // Добавляем как ответственного с обязательным согласованием
            let already_added = sqlx::query!(
                "SELECT EXISTS(SELECT 1 FROM application_responsible_users WHERE application_id = $1 AND user_id = $2) as exists",
                application_id,
                forward_user.user_id
            )
            .fetch_one(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Failed to check if user already added: {}", e);
                error::ErrorInternalServerError("Database error")
            })?;

            if already_added.exists.unwrap_or(false) {
                // Обновляем только поле required_approval, если пользователь уже добавлен
                sqlx::query!(
                    r#"
                    UPDATE application_responsible_users 
                    SET required_approval = $1,
                        created_by = $2
                    WHERE application_id = $3 AND user_id = $4
                    "#,
                    forward_user.required_approval,
                    current_user_id,
                    application_id,
                    forward_user.user_id
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to update existing responsible user: {}", e);
                    error::ErrorInternalServerError("Error updating responsible user")
                })?;
                
                log::info!("Updated existing responsible user {} with required_approval=true", forward_user.user_id);
            } else {
                // Добавляем нового пользователя
                sqlx::query!(
                    r#"
                    INSERT INTO application_responsible_users (
                        application_id, 
                        user_id, 
                        required_approval,
                        approval_status,
                        created_at,
                        created_by,
                        is_primary
                    )
                    VALUES ($1, $2, $3, 'pending', $4, $5, false)
                    "#,
                    application_id,
                    forward_user.user_id,
                    forward_user.required_approval,
                    base_time.naive_utc(),
                    current_user_id
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to insert responsible user: {}", e);
                    error::ErrorInternalServerError("Error adding responsible user")
                })?;
                
                log::info!("Added new responsible user {} with required_approval=true", forward_user.user_id);
            }
            
            added_responsible_users.push((forward_user.user_id, forward_user.required_approval));
        } 
        else if forward_user.can_view {
            // Добавляем как просматривающего
            let already_added = sqlx::query!(
                "SELECT EXISTS(SELECT 1 FROM application_viewers WHERE application_id = $1 AND user_id = $2) as exists",
                application_id,
                forward_user.user_id
            )
            .fetch_one(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Failed to check if user already added as viewer: {}", e);
                error::ErrorInternalServerError("Database error")
            })?;

            if !already_added.exists.unwrap_or(false) {
                sqlx::query!(
                    r#"
                    INSERT INTO application_viewers (
                        application_id,
                        user_id,
                        created_at,
                        created_by
                    )
                    VALUES ($1, $2, $3, $4)
                    "#,
                    application_id,
                    forward_user.user_id,
                    base_time.naive_utc(),
                    current_user_id
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to insert viewer: {}", e);
                    error::ErrorInternalServerError("Error adding viewer")
                })?;
                
                log::info!("Added viewer user {}", forward_user.user_id);
            }
            
            added_viewers.push(forward_user.user_id);
        }
        else {
            // Случай: пользователь должен быть ответственным, но с required_approval = false
            // (т.е. на фронтенде выбрали только "Требуется согласование", но не "Согласование обязательно")
            
            log::info!("Adding user {} as responsible with required_approval=false", forward_user.user_id);
            
            let already_added = sqlx::query!(
                "SELECT EXISTS(SELECT 1 FROM application_responsible_users WHERE application_id = $1 AND user_id = $2) as exists",
                application_id,
                forward_user.user_id
            )
            .fetch_one(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Failed to check if user already added: {}", e);
                error::ErrorInternalServerError("Database error")
            })?;

            if already_added.exists.unwrap_or(false) {
                // Обновляем запись, устанавливая required_approval = false
                sqlx::query!(
                    r#"
                    UPDATE application_responsible_users 
                    SET required_approval = false,
                        created_by = $1
                    WHERE application_id = $2 AND user_id = $3
                    "#,
                    current_user_id,
                    application_id,
                    forward_user.user_id
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to update existing responsible user: {}", e);
                    error::ErrorInternalServerError("Error updating responsible user")
                })?;
                
                log::info!("Updated existing responsible user {} with required_approval=false", forward_user.user_id);
            } else {
                // Добавляем нового пользователя как ответственного с required_approval = false
                sqlx::query!(
                    r#"
                    INSERT INTO application_responsible_users (
                        application_id, 
                        user_id, 
                        required_approval,
                        approval_status,
                        created_at,
                        created_by,
                        is_primary
                    )
                    VALUES ($1, $2, false, 'pending', $3, $4, false)
                    "#,
                    application_id,
                    forward_user.user_id,
                    base_time.naive_utc(),
                    current_user_id
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to insert responsible user: {}", e);
                    error::ErrorInternalServerError("Error adding responsible user")
                })?;
                
                log::info!("Added new responsible user {} with required_approval=false", forward_user.user_id);
            }
            
            added_responsible_users.push((forward_user.user_id, false));
        }
    }

    // ЭТАП 2: Записываем историю о назначениях
    // Сначала ответственные
    for (user_id, required_approval) in &added_responsible_users {
        history_time = history_time + chrono::Duration::milliseconds(1);
        
        sqlx::query!(
            r#"
            INSERT INTO application_history (
                application_id,
                user_id,
                action_type,
                metadata,
                created_at
            )
            VALUES ($1, $2, $3, $4, $5)
            "#,
            application_id,
            *user_id,
            "assigned_responsible",
            serde_json::json!({
                "required_approval": *required_approval,
                "is_primary": false,
                "forwarded_by": current_user_name,
                "type": "responsible"
            }),
            history_time
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to add assigned responsible history: {}", e);
            error::ErrorInternalServerError("Error adding history")
        })?;
    }

    // Потом просматривающие
    for user_id in &added_viewers {
        history_time = history_time + chrono::Duration::milliseconds(1);
        
        sqlx::query!(
            r#"
            INSERT INTO application_history (
                application_id,
                user_id,
                action_type,
                metadata,
                created_at
            )
            VALUES ($1, $2, $3, $4, $5)
            "#,
            application_id,
            *user_id,
            "assigned_viewer",
            serde_json::json!({
                "forwarded_by": current_user_name,
                "type": "viewer"
            }),
            history_time
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to add assigned viewer history: {}", e);
            error::ErrorInternalServerError("Error adding history")
        })?;
    }

    // ЭТАП 3: Обновляем общий статус заявки на основе новых правил (только если были добавлены ответственные)
    if !added_responsible_users.is_empty() {
        update_application_confirmation_based_on_approvals(&mut transaction, application_id).await?;
    }

    // ЭТАП 4: Получаем новый confirmation после обновления
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

    // ЭТАП 5: Если confirmation изменился, записываем историю изменения статуса
    if old_confirmation.confirmation != new_confirmation.confirmation {
        let status_change_time = history_time + chrono::Duration::milliseconds(1);
        
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
            VALUES ($1, $2, $3, $4, $5, $6)
            "#,
            application_id,
            current_user_id,
            "confirmation_change",
            old_confirmation.confirmation,
            new_confirmation.confirmation,
            status_change_time
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

    log::info!("Successfully forwarded application {}", application_id);

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application forwarded successfully"
    })))
}

/// Получение всех заявок с фильтрами (для Центра заявок)
pub async fn get_applications(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    filter: web::Query<ApplicationFilter>,
) -> Result<HttpResponse, Error> {
    // Проверка токена
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let username = &claims.sub;

    // Получаем информацию о текущем пользователе
    let current_user = sqlx::query!(
        r#"
        SELECT 
            u.id,
            EXISTS(SELECT 1 FROM application_approvers aa WHERE aa.user_id = u.id) as is_approver
        FROM users u
        WHERE u.username = $1
        "#,
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch current user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let current_user = match current_user {
        Some(user) => user,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let current_user_id = current_user.id;
    let is_approver = current_user.is_approver.unwrap_or(false);

    log::info!("Getting applications for Center - user {} (approver: {})", current_user_id, is_approver);

    // Базовый запрос
    let mut query = String::from(
        "SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
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
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || ru.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || ru.middle_name
                    ELSE ''
                END
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE 1=1"
    );

    // Фильтр по правам пользователя для Центра заявок
    if is_approver {
        // Принимающие видят все заявки (как и раньше)
        query.push_str(" AND 1=1");
    } else {
        // Обычные пользователи в Центре заявок видят только те заявки, где они:
        // - являются ответственными
        // - являются просматривающими
        // НО НЕ являются отправителями (отправители видят свои заявки в UserApplications)
        query.push_str(&format!(
            r#" AND (
                EXISTS(
                    SELECT 1 FROM application_responsible_users aru 
                    WHERE aru.application_id = a.id 
                    AND aru.user_id = {}
                )
                OR EXISTS(
                    SELECT 1 FROM application_viewers av 
                    WHERE av.application_id = a.id 
                    AND av.user_id = {}
                )
            )"#,
            current_user_id, current_user_id
        ));
    }

    let mut params: Vec<String> = Vec::new();
    let mut param_counter = 1;

    // Фильтр по поиску
    if let Some(ref search) = filter.search_query {
        if !search.is_empty() {
            query.push_str(&format!(" AND ( 
                a.application_number ILIKE ${} OR
                COALESCE(o.name, c.name, '') ILIKE ${} OR
                c.name ILIKE ${} OR
                a.message ILIKE ${} OR
                a.status ILIKE ${} OR
                a.confirmation ILIKE ${}
            )", param_counter, param_counter + 1, param_counter + 2, param_counter + 3, param_counter + 4, param_counter + 5));
            for _ in 0..6 {
                params.push(format!("%{}%", search));
            }
            param_counter += 6;
        }
    }

    // Фильтр по организации
    if let Some(org_id) = filter.organization_id {
        query.push_str(&format!(" AND a.organization_id = ${}", param_counter));
        params.push(org_id.to_string());
        param_counter += 1;
    }

    // Фильтр по компании
    if let Some(company_id) = filter.company_id {
        query.push_str(&format!(" AND a.company_id = ${}", param_counter));
        params.push(company_id.to_string());
        param_counter += 1;
    }

    // Фильтр по подтверждению
    if let Some(ref confirmation) = filter.confirmation {
        query.push_str(&format!(" AND a.confirmation = ${}", param_counter));
        params.push(confirmation.clone());
        param_counter += 1;
    }

    // Фильтр по статусу
    if let Some(ref status) = filter.status {
        query.push_str(&format!(" AND a.status = ${}", param_counter));
        params.push(status.clone());
        param_counter += 1;
    }

    // Фильтр по дате
    if let Some(date_from) = filter.date_from {
        query.push_str(&format!(" AND a.sending_datetime >= ${}", param_counter));
        params.push(date_from.and_hms_opt(0, 0, 0).unwrap().to_string());
        param_counter += 1;
    }

    if let Some(date_to) = filter.date_to {
        query.push_str(&format!(" AND a.sending_datetime <= ${}", param_counter));
        params.push(date_to.and_hms_opt(23, 59, 59).unwrap().to_string());
        param_counter += 1;
    }

    query.push_str(" ORDER BY a.sending_datetime DESC");

    log::debug!("SQL query: {}", query);
    log::debug!("Params: {:?}", params);

    let mut query_builder = sqlx::query(&query);
    
    for param in &params {
        query_builder = query_builder.bind(param);
    }

    let rows = query_builder
        .fetch_all(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to fetch applications: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

    let applications: Vec<ApplicationWithDetails> = rows.iter().map(|row| {
        // Получаем DateTime<Utc> из БД
        let sending_datetime: DateTime<Utc> = row.try_get("sending_datetime")
            .unwrap_or_else(|_| Utc::now());
        
        let reading_datetime: Option<DateTime<Utc>> = row.try_get("reading_datetime").ok();
        let confirmation_datetime: Option<DateTime<Utc>> = row.try_get("confirmation_datetime").ok();

        ApplicationWithDetails {
            id: row.try_get("id").unwrap_or(0),
            application_number: row.try_get("application_number").unwrap_or_default(),
            confirmation: row.try_get("confirmation").unwrap_or_default(),
            sending_datetime,
            reading_datetime,
            confirmation_datetime,
            organization_id: row.try_get("organization_id").unwrap_or(0),
            organization_name: row.try_get("organization_name").unwrap_or_default(),
            company_id: row.try_get("company_id").ok(),
            company_name: row.try_get("company_name").unwrap_or_default(),
            sender_user_id: row.try_get("sender_user_id").unwrap_or(0),
            sender_full_name: row.try_get("sender_full_name").ok(),
            sender_name: row.try_get("sender_name").unwrap_or_default(),
            message: row.try_get("message").ok(),
            status: row.try_get("status").unwrap_or_default(),
            responsible_user_id: row.try_get("responsible_user_id").ok(),
            responsible_full_name: row.try_get("responsible_full_name").ok(),
            responsible_name: row.try_get("responsible_name").unwrap_or_default(),
            responsible_comment: row.try_get("responsible_comment").ok(),
            data_approval: row.try_get("data_approval").unwrap_or(false),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(applications))
}

/// Создание новой заявки
pub async fn create_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<ApplicationCreateRequest>,
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
    
    // Получаем ID пользователя из базы данных
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

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    if !form.data_approval {
        return Err(error::ErrorBadRequest("Data approval is required"));
    }

    log::info!("Creating new application for user: {}", user_id);

    let now_utc = Utc::now();
    let today_local = now_utc.date_naive();
    let date_part = today_local.format("%Y%m%d").to_string();
    
    let count_result = sqlx::query!(
        "SELECT COUNT(*) as count FROM applications WHERE DATE(sending_datetime AT TIME ZONE 'UTC') = $1",
        today_local
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to count applications: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application_number = format!("№ {}/{:03}", date_part, count_result.count.unwrap_or(0) + 1);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    let application_result = sqlx::query!(
        r#"
        INSERT INTO applications (
            application_number, 
            organization_id, 
            company_id,
            sender_user_id, 
            message, 
            data_approval,
            status,
            confirmation,
            sending_datetime
        )
        VALUES ($1, $2, $3, $4, $5, $6, 'Непрочитано', 'Согласование', $7)
        RETURNING id, application_number
        "#,
        application_number,
        form.organization_id,
        form.company_id,
        user_id,
        form.message,
        form.data_approval,
        now_utc
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to create application: {}", e);
        error::ErrorInternalServerError("Error creating application")
    })?;

    let application_id = application_result.id;

    // Получаем ответственных пользователей для организации и компании
    let mut responsible_users = Vec::new();
    let mut primary_responsible_id: Option<i32> = None;

    if let Some(org_id) = form.organization_id {
        // Получаем ответственных для организации
        let org_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary
            FROM organization_users
            WHERE organization_id = $1
            "#,
            org_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch organization responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching organization responsibles")
        })?;

        for row in org_responsibles {
            let is_primary = row.is_primary.unwrap_or(false);
            responsible_users.push((row.user_id, is_primary));
            if is_primary {
                primary_responsible_id = Some(row.user_id);
            }
        }
    }

    if let Some(company_id) = form.company_id {
        // Получаем ответственных для компании
        let company_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary
            FROM companies_users
            WHERE company_id = $1
            "#,
            company_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch company responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching company responsibles")
        })?;

        for row in company_responsibles {
            let exists = responsible_users.iter().any(|&(user_id, _)| user_id == row.user_id);
            if !exists {
                let is_primary = row.is_primary.unwrap_or(false);
                responsible_users.push((row.user_id, is_primary));
                if is_primary && primary_responsible_id.is_none() {
                    primary_responsible_id = Some(row.user_id);
                }
            }
        }
    }

    // Обновляем поле responsible_user_id в заявке (главный ответственный)
    if let Some(primary_id) = primary_responsible_id {
        sqlx::query!(
            "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
            primary_id,
            application_id
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to update primary responsible: {}", e);
            error::ErrorInternalServerError("Error updating primary responsible")
        })?;
    }

    // Добавляем всех ответственных в новую таблицу
    for (user_id, is_primary) in responsible_users {
        sqlx::query!(
            r#"
            INSERT INTO application_responsible_users (application_id, user_id, is_primary, approval_status, created_at)
            VALUES ($1, $2, $3, 'pending', NOW())
            ON CONFLICT (application_id, user_id) DO NOTHING
            "#,
            application_id,
            user_id,
            is_primary
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to insert responsible user: {}", e);
            error::ErrorInternalServerError("Error inserting responsible user")
        })?;
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    log::info!("Successfully created application with ID: {}", application_result.id);

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application created successfully",
        "application_id": application_result.id,
        "application_number": application_result.application_number
    })))
}

/// Создание полной заявки с вложениями
pub async fn submit_complete_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<CompleteApplicationRequest>,
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
    
    // Получаем ID пользователя из базы данных
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

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    if !form.data_approval {
        return Err(error::ErrorBadRequest("Data approval is required"));
    }

    if form.attachments.is_empty() {
        return Err(error::ErrorBadRequest("At least one attachment is required"));
    }

    log::info!("Creating complete application for user: {}", user_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Базовое время для всех операций
    let base_time = Utc::now();
    let mut history_time = base_time;

    // 1. Создаем заявку в таблице applications
    let now_utc = Utc::now();
    let now_naive = now_utc.naive_utc(); 
    let today_local = now_utc.date_naive();
    let date_part = today_local.format("%Y%m%d").to_string();
    
    let count_result = sqlx::query!(
        "SELECT COUNT(*) as count FROM applications WHERE DATE(sending_datetime AT TIME ZONE 'UTC') = $1",
        today_local
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to count applications: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application_number = format!("№ {}/{:03}", date_part, count_result.count.unwrap_or(0) + 1);

    // Получаем ID организации по имени
    let organization_row = sqlx::query!(
        "SELECT id FROM organizations WHERE name = $1",
        form.organization
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch organization: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let organization_id = match organization_row {
        Some(row) => Some(row.id),
        None => None
    };

    // Получаем ID компании по имени (если указана)
    let company_id = if let Some(company_name) = &form.company {
        let company_row = sqlx::query!(
            "SELECT id FROM companies WHERE name = $1",
            company_name
        )
        .fetch_optional(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch company: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;
        
        company_row.map(|row| row.id)
    } else {
        None
    };

    let application_result = sqlx::query!(
        r#"
        INSERT INTO applications (
            application_number, 
            organization_id, 
            company_id,
            sender_user_id, 
            message, 
            data_approval,
            status,
            confirmation,
            sending_datetime
        )
        VALUES ($1, $2, $3, $4, $5, $6, 'Непрочитано', 'Согласование', $7)
        RETURNING id, application_number
        "#,
        application_number,
        organization_id,
        company_id,
        user_id,
        form.message,
        form.data_approval,
        now_utc
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to create application: {}", e);
        error::ErrorInternalServerError("Error creating application")
    })?;

    let application_id = application_result.id;

    // Записываем в историю создание заявки (с базовым временем)
    sqlx::query!(
        r#"
        INSERT INTO application_history (
            application_id,
            user_id,
            action_type,
            new_value,
            metadata,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6)
        "#,
        application_id,
        user_id,
        "create",
        application_result.application_number,
        serde_json::json!({
            "confirmation": "Согласование",
            "status": "Непрочитано"
        }),
        history_time
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add create history: {}", e);
        error::ErrorInternalServerError("Error adding history")
    })?;

    // Получаем ответственных пользователей для организации и компании
    let mut responsible_users: Vec<(i32, bool, bool)> = Vec::new(); // (user_id, is_primary, required_approval)
    let mut primary_responsible_id: Option<i32> = None;

    if let Some(org_id) = organization_id {
        // Получаем ответственных для организации
        let org_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary, required_approval
            FROM organization_users
            WHERE organization_id = $1
            "#,
            org_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch organization responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching organization responsibles")
        })?;

        for row in org_responsibles {
            let is_primary = row.is_primary.unwrap_or(false);
            let required_approval = row.required_approval;
            responsible_users.push((row.user_id, is_primary, required_approval));
            if is_primary {
                primary_responsible_id = Some(row.user_id);
            }
        }
    }

    if let Some(comp_id) = company_id {
        // Получаем ответственных для компании
        let company_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary, required_approval
            FROM companies_users
            WHERE company_id = $1
            "#,
            comp_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch company responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching company responsibles")
        })?;

        for row in company_responsibles {
            // Проверяем, не добавлен ли уже этот пользователь из организации
            let exists = responsible_users.iter().any(|&(user_id, _, _)| user_id == row.user_id);
            if !exists {
                let is_primary = row.is_primary.unwrap_or(false);
                let required_approval = row.required_approval;
                responsible_users.push((row.user_id, is_primary, required_approval));
                if is_primary && primary_responsible_id.is_none() {
                    primary_responsible_id = Some(row.user_id);
                }
            }
        }
    }

    // Добавляем информацию об обязательных ответственных из запроса
    if let Some(required_users) = &form.required_users {
        for req_user in required_users {
            // Проверяем, не добавлен ли уже этот пользователь
            let exists = responsible_users.iter().any(|&(user_id, _, _)| user_id == req_user.user_id);
            if !exists {
                responsible_users.push((req_user.user_id, false, req_user.required_approval));
            } else {
                // Если пользователь уже есть, обновляем флаг required_approval
                if let Some(pos) = responsible_users.iter().position(|&(user_id, _, _)| user_id == req_user.user_id) {
                    responsible_users[pos].2 = req_user.required_approval;
                }
            }
        }
    }

    // Обновляем поле responsible_user_id в заявке (главный ответственный)
    if let Some(primary_id) = primary_responsible_id {
        sqlx::query!(
            "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
            primary_id,
            application_id
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to update primary responsible: {}", e);
            error::ErrorInternalServerError("Error updating primary responsible")
        })?;
    }

    // Добавляем всех ответственных в новую таблицу и записываем в историю
    for (user_id, is_primary, required_approval) in responsible_users {
        sqlx::query!(
            r#"
            INSERT INTO application_responsible_users (
                application_id, 
                user_id, 
                is_primary, 
                required_approval,
                approval_status,
                created_at
            )
            VALUES ($1, $2, $3, $4, 'pending', $5)
            ON CONFLICT (application_id, user_id) 
            DO UPDATE SET 
                is_primary = EXCLUDED.is_primary,
                required_approval = EXCLUDED.required_approval
            "#,
            application_id,
            user_id,
            is_primary,
            required_approval,
            now_naive
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to insert responsible user: {}", e);
            error::ErrorInternalServerError("Error inserting responsible user")
        })?;
        
        // Увеличиваем время для следующей записи истории
        history_time = history_time + chrono::Duration::milliseconds(1);
        
        // Записываем в историю назначение ответственного
        sqlx::query!(
            r#"
            INSERT INTO application_history (
                application_id,
                user_id,
                action_type,
                metadata,
                created_at
            )
            VALUES ($1, $2, $3, $4, $5)
            "#,
            application_id,
            user_id,
            "assigned_responsible",
            serde_json::json!({
                "required_approval": required_approval,
                "is_primary": is_primary
            }),
            history_time
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to add assigned responsible history: {}", e);
            error::ErrorInternalServerError("Error adding history")
        })?;
    }

    // 2. Создаем вложения для заявки
    for attachment in &form.attachments {
        // Конвертируем строки дат и времени
        let entry_date_from: Option<NaiveDate> = attachment.entry_date_from.as_ref()
            .and_then(|s| NaiveDate::parse_from_str(s, "%Y-%m-%d").ok());
        
        let entry_date_to: Option<NaiveDate> = attachment.entry_date_to.as_ref()
            .and_then(|s| NaiveDate::parse_from_str(s, "%Y-%m-%d").ok());
        
        let entry_time_from: Option<NaiveTime> = attachment.entry_time_from.as_ref()
            .and_then(|s| NaiveTime::parse_from_str(s, "%H:%M:%S").ok());
        
        let entry_time_to: Option<NaiveTime> = attachment.entry_time_to.as_ref()
            .and_then(|s| NaiveTime::parse_from_str(s, "%H:%M:%S").ok());

        let attachment_result = sqlx::query!(
            r#"
            INSERT INTO attachments (
                application_id,
                attachment_type,
                attachment_name,
                attachment_display_name,
                unique_attachment_id,
                entry_date_from,
                entry_date_to,
                entry_time_from,
                entry_time_to,
                status
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)
            RETURNING id
            "#,
            application_id,
            attachment.attachment_type,
            attachment.attachment_name,
            attachment.attachment_display_name,
            attachment.unique_attachment_id,
            entry_date_from,
            entry_date_to,
            entry_time_from,
            entry_time_to
        )
        .fetch_one(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to create attachment: {}", e);
            error::ErrorInternalServerError("Error creating attachment")
        })?;

        let attachment_id = attachment_result.id;

        // 3. Создаем данные в зависимости от типа вложения
        match attachment.attachment_type.as_str() {
            "cars" => {
    if let Some(vehicles) = &attachment.data.vehicles {
        for vehicle in vehicles {
            let car_result = sqlx::query!(
                r#"
                INSERT INTO cars (
                    attachment_id,
                    car_number,
                    car_brand,
                    unload_place,
                    entry_date_from,
                    entry_time_from,
                    entry_date_to,
                    entry_time_to,
                    status
                )
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0)
                RETURNING id
                "#,
                attachment_id,
                vehicle.car_number,
                vehicle.car_brand,
                vehicle.unload_place.as_deref(),
                entry_date_from,
                entry_time_from,
                entry_date_to,
                entry_time_to
            )
            .fetch_one(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Failed to create car: {}", e);
                error::ErrorInternalServerError("Error creating car")
            })?;

            let car_id = car_result.id;

            // ДОБАВЛЯЕМ ЗАПИСЬ В ИСТОРИЮ АВТОМОБИЛЯ
            // Преобразуем DateTime<Utc> в NaiveDateTime
            let car_history_time = (base_time + chrono::Duration::milliseconds(1)).naive_utc();
            
            sqlx::query!(
                r#"
                INSERT INTO cars_history (
                    car_id,
                    user_id,
                    action_type,
                    comment,
                    created_at
                )
                VALUES ($1, $2, $3, $4, $5)
                "#,
                car_id,
                user_id,
                "create",
                format!("Автомобиль {} {} создан", vehicle.car_number, vehicle.car_brand),
                car_history_time
            )
            .execute(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Failed to add car history entry: {}", e);
                error::ErrorInternalServerError("Error adding car history entry")
            })?;

            // Создаем связи с местами разгрузки
            for &place_id in &vehicle.unload_places {
                sqlx::query!(
                    r#"
                    INSERT INTO car_unload_places (car_id, unload_place_id, order_index)
                    VALUES ($1, $2, 1)
                    "#,
                    car_id,
                    place_id
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to create car unload place: {}", e);
                    error::ErrorInternalServerError("Error creating car unload place")
                })?;
            }
        }
    }
}
            "people" => {
                if let Some(employees) = &attachment.data.employees {
                    for employee in employees {
                        let employee_result = sqlx::query!(
                            r#"
                            INSERT INTO employees (
                                attachment_id,
                                last_name,
                                first_name,
                                middle_name,
                                citizenship_id,
                                position,
                                passport_series_number,
                                patent_number,
                                other_permission,
                                status
                            )
                            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0)
                            RETURNING id
                            "#,
                            attachment_id,
                            employee.last_name,
                            employee.first_name,
                            employee.middle_name.as_deref(),
                            employee.citizenship_id,
                            employee.position,
                            employee.passport_series_number,
                            employee.patent_number.as_deref(),
                            employee.other_permission.as_deref()
                        )
                        .fetch_one(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create employee: {}", e);
                            error::ErrorInternalServerError("Error creating employee")
                        })?;

                        let employee_id = employee_result.id;

                        // Создаем связи с целевыми таблицами
                        for &table_id in &employee.target_tables {
                            sqlx::query!(
                                r#"
                                INSERT INTO employee_target_tables (employee_id, table_id, order_index)
                                VALUES ($1, $2, 1)
                                "#,
                                employee_id,
                                table_id
                            )
                            .execute(&mut *transaction)
                            .await
                            .map_err(|e| {
                                log::error!("Failed to create employee target table: {}", e);
                                error::ErrorInternalServerError("Error creating employee target table")
                            })?;
                        }
                    }
                }
            }
            "items" => {
                if let Some(items) = &attachment.data.items {
                    for item in items {
                        let now_utc_date = now_utc.date_naive();
                        
                        sqlx::query!(
                            r#"
                            INSERT INTO items (
                                attachment_id,
                                name,
                                count,
                                date_created
                            )
                            VALUES ($1, $2, $3, $4)
                            "#,
                            attachment_id,
                            item.name,
                            item.count,
                            now_utc_date
                        )
                        .execute(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create item: {}", e);
                            error::ErrorInternalServerError("Error creating item")
                        })?;
                    }
                }
            }
            _ => {
                return Err(error::ErrorBadRequest("Invalid attachment type"));
            }
        }
    }

    // Фиксируем транзакцию
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    log::info!("Successfully created complete application with ID: {}", application_id);

    let response = CompleteApplicationResponse {
        success: true,
        message: "Application created successfully".to_string(),
        application_id,
        application_number: application_result.application_number,
    };

    Ok(HttpResponse::Ok().json(response))
}

/// Получение ответственных пользователей для заявки с информацией о согласовании
pub async fn get_application_responsible_users(
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

    log::info!("Getting responsible users for application: {}", application_id);

    #[derive(Debug, sqlx::FromRow)]
    struct DbResponsibleUser {
        id: i32,
        username: String,
        last_name: Option<String>,
        first_name: Option<String>,
        middle_name: Option<String>,
        position: Option<String>,
        is_primary: bool,
        required_approval: bool,
        approval_status: Option<String>,
        approval_comment: Option<String>,
        approval_datetime: Option<DateTime<Utc>>,
    }

    let responsibles = sqlx::query_as!(
        DbResponsibleUser,
        r#"
        SELECT 
            u.id,
            u.username,
            u.last_name,
            u.first_name,
            u.middle_name,
            u.position,
            COALESCE(aru.is_primary, false) as "is_primary!",
            COALESCE(aru.required_approval, false) as "required_approval!",
            aru.approval_status,
            aru.approval_comment,
            aru.approval_datetime
        FROM application_responsible_users aru
        JOIN users u ON aru.user_id = u.id
        WHERE aru.application_id = $1
        ORDER BY aru.is_primary DESC, u.last_name, u.first_name
        "#,
        application_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch responsible users: {}", e);
        error::ErrorInternalServerError("Error fetching responsible users")
    })?;

    // Преобразуем в ResponsibleUserInfo
    use crate::models::applications::ResponsibleUserInfo;
    let responsibles_info: Vec<ResponsibleUserInfo> = responsibles.iter().map(|row| {
        ResponsibleUserInfo {
            id: row.id,
            username: row.username.clone(),
            last_name: row.last_name.clone(),
            first_name: row.first_name.clone(),
            middle_name: row.middle_name.clone(),
            position: row.position.clone(),
            is_primary: row.is_primary,
            required_approval: row.required_approval,
            approval_status: row.approval_status.clone(),
            approval_comment: row.approval_comment.clone(),
            approval_datetime: row.approval_datetime,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(responsibles_info))
}

/// Обновление заявки (подтверждение, статус и т.д.)
/// Обновление заявки (подтверждение, статус и т.д.)
pub async fn update_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    update_data: web::Json<ApplicationUpdateRequest>,
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
    
    // Получаем ID пользователя из базы данных
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

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };
    
    let application_id = path.into_inner();

    log::info!("Updating application {} by user {}", application_id, user_id);

    // Получаем текущее время в UTC
    let now_utc = Utc::now();

    // Строим динамический запрос с типизированными параметрами
    let mut query_parts: Vec<String> = Vec::new();
    let mut query = String::from("UPDATE applications SET ");
    let mut param_counter = 1;
    
    // Собираем параметры с правильными типами
    if let Some(ref confirmation) = update_data.confirmation {
        query_parts.push(format!("confirmation = ${}", param_counter));
        param_counter += 1;
        
        if confirmation == "Согласовано" || confirmation == "Не согласовано" {
            query_parts.push(format!("confirmation_datetime = ${}", param_counter));
            param_counter += 1;
        }
    }

    if let Some(ref status) = update_data.status {
        query_parts.push(format!("status = ${}", param_counter));
        param_counter += 1;
        
        // Если статус меняется на "В обработке", обновляем reading_datetime
        if status == "В обработке" {
            query_parts.push(format!("reading_datetime = ${}", param_counter));
            param_counter += 1;
        }
    }

    if let Some(ref comment) = update_data.responsible_comment {
        query_parts.push(format!("responsible_comment = ${}", param_counter));
        param_counter += 1;
        query_parts.push(format!("responsible_user_id = ${}", param_counter));
        param_counter += 1;
    }

    if query_parts.is_empty() {
        return Err(error::ErrorBadRequest("No data to update"));
    }

    query.push_str(&query_parts.join(", "));
    query.push_str(&format!(" WHERE id = ${}", param_counter));

    log::debug!("Update query: {}", query);

    // Создаем query builder и добавляем параметры с правильными типами
    let mut query_builder = sqlx::query(&query);
    
    if let Some(ref confirmation) = update_data.confirmation {
        query_builder = query_builder.bind(confirmation);
        
        if confirmation == "Согласовано" || confirmation == "Не согласовано" {
            query_builder = query_builder.bind(now_utc);
        }
    }

    if let Some(ref status) = update_data.status {
        query_builder = query_builder.bind(status);
        
        if status == "В обработке" {
            query_builder = query_builder.bind(now_utc);
        }
    }

    if let Some(ref comment) = update_data.responsible_comment {
        query_builder = query_builder.bind(comment);
        query_builder = query_builder.bind(user_id);
    }

    // Добавляем ID заявки
    query_builder = query_builder.bind(application_id);

    let result = query_builder
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to update application: {:?}", e);
            error::ErrorInternalServerError(format!("Error updating application: {}", e))
        })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application updated successfully",
        "rows_affected": result.rows_affected()
    })))
}

/// Получение заявки по ID с расширенной информацией (включая ответственных с информацией о согласовании)
/// Получение заявки по ID с обновлением статуса
pub async fn get_application_by_id(
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

    let username = &claims.sub;
    
    // Получаем ID текущего пользователя
    let current_user = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let current_user_id = match current_user {
        Some(user) => user.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let application_id = path.into_inner();

    log::info!("Getting application by ID: {} for user {}", application_id, current_user_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем информацию о заявке
    let application_row = sqlx::query!(
        r#"
        SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
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
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || ru.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || ru.middle_name
                    ELSE ''
                END
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE a.id = $1
        "#,
        application_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application = match application_row {
        Some(r) => {
            // Если заявка непрочитана и ее читает не отправитель, обновляем статус
            if r.status == "Непрочитано" && r.sender_user_id != current_user_id {
                sqlx::query!(
                    "UPDATE applications SET status = 'В обработке', reading_datetime = NOW() WHERE id = $1",
                    application_id
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to update application status: {}", e);
                    error::ErrorInternalServerError("Error updating application status")
                })?;
                
                // Записываем в историю прочтение заявки
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
                    VALUES ($1, $2, $3, $4, $5, NOW())
                    "#,
                    application_id,
                    current_user_id,
                    "read",
                    "Непрочитано",
                    "В обработке"
                )
                .execute(&mut *transaction)
                .await
                .map_err(|e| {
                    log::error!("Failed to add read history: {}", e);
                    error::ErrorInternalServerError("Error adding history")
                })?;
                
                log::info!("Application {} marked as read by user {}", application_id, current_user_id);
            }

            // Получаем список всех ответственных
            #[derive(Debug, sqlx::FromRow)]
            struct DbResponsibleUser {
                id: i32,
                username: String,
                last_name: Option<String>,
                first_name: Option<String>,
                middle_name: Option<String>,
                position: Option<String>,
                is_primary: bool,
                required_approval: bool,
                approval_status: Option<String>,
                approval_comment: Option<String>,
                approval_datetime: Option<DateTime<Utc>>,
            }

            let responsibles = sqlx::query_as!(
                DbResponsibleUser,
                r#"
                SELECT 
                    u.id,
                    u.username,
                    u.last_name,
                    u.first_name,
                    u.middle_name,
                    u.position,
                    COALESCE(aru.is_primary, false) as "is_primary!",
                    COALESCE(aru.required_approval, false) as "required_approval!",
                    aru.approval_status,
                    aru.approval_comment,
                    aru.approval_datetime
                FROM application_responsible_users aru
                JOIN users u ON aru.user_id = u.id
                WHERE aru.application_id = $1
                ORDER BY aru.is_primary DESC, u.last_name, u.first_name
                "#,
                application_id
            )
            .fetch_all(&mut *transaction)
            .await
            .unwrap_or_else(|_| Vec::new());

            // Преобразуем в ResponsibleUserInfo
            use crate::models::applications::ResponsibleUserInfo;
            let responsibles_info: Vec<ResponsibleUserInfo> = responsibles.iter().map(|row| {
                ResponsibleUserInfo {
                    id: row.id,
                    username: row.username.clone(),
                    last_name: row.last_name.clone(),
                    first_name: row.first_name.clone(),
                    middle_name: row.middle_name.clone(),
                    position: row.position.clone(),
                    is_primary: row.is_primary,
                    required_approval: row.required_approval,
                    approval_status: row.approval_status.clone(),
                    approval_comment: row.approval_comment.clone(),
                    approval_datetime: row.approval_datetime,
                }
            }).collect();

            let sending_datetime: DateTime<Utc> = r.sending_datetime;
            let reading_datetime: Option<DateTime<Utc>> = r.reading_datetime;
            let confirmation_datetime: Option<DateTime<Utc>> = r.confirmation_datetime;

            let application_with_details = ApplicationWithDetails {
                id: r.id,
                application_number: r.application_number,
                confirmation: r.confirmation,
                sending_datetime,
                reading_datetime,
                confirmation_datetime,
                organization_id: r.organization_id,
                organization_name: r.organization_name.unwrap_or_default(),
                company_id: r.company_id,
                company_name: r.company_name,
                sender_user_id: r.sender_user_id,
                sender_full_name: r.sender_full_name,
                sender_name: r.sender_name.unwrap_or_default(),
                message: r.message,
                status: r.status,
                responsible_user_id: r.responsible_user_id,
                responsible_full_name: r.responsible_full_name,
                responsible_name: r.responsible_name.unwrap_or_default(),
                responsible_comment: r.responsible_comment,
                data_approval: r.data_approval,
            };

            let mut response = serde_json::to_value(application_with_details)
                .map_err(|e| {
                    log::error!("Failed to serialize application: {}", e);
                    error::ErrorInternalServerError("Error serializing application")
                })?;
            
            if let serde_json::Value::Object(ref mut map) = response {
                map.insert("responsible_users".to_string(), serde_json::to_value(responsibles_info)
                    .map_err(|e| {
                        log::error!("Failed to serialize responsibles: {}", e);
                        error::ErrorInternalServerError("Error serializing responsibles")
                    })?);
            }

            response
        },
        None => return Err(error::ErrorNotFound("Application not found")),
    };

    // Фиксируем транзакцию
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(application))
}

/// Получение заявок текущего пользователя
pub async fn get_user_applications(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    filter: web::Query<ApplicationFilter>,
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

    log::info!("Getting applications for user: {}", username);

    // Получаем ID пользователя из базы данных
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

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let mut query = String::from(
        "SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
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
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || ru.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || ru.middle_name
                    ELSE ''
                END
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE 1=1"
    );

    let mut params: Vec<String> = Vec::new();
    let mut param_counter = 1;

    // Этот endpoint возвращает ВСЕ заявки, фильтрация по вкладкам происходит на фронтенде

    if let Some(ref search) = filter.search_query {
        if !search.is_empty() {
            query.push_str(&format!(" AND (
                a.application_number ILIKE ${} OR
                COALESCE(o.name, c.name, '') ILIKE ${} OR
                c.name ILIKE ${} OR
                a.message ILIKE ${} OR
                a.status ILIKE ${} OR
                a.confirmation ILIKE ${} OR
                u.last_name ILIKE ${} OR
                u.first_name ILIKE ${} OR
                u.middle_name ILIKE ${} OR
                ru.last_name ILIKE ${} OR
                ru.first_name ILIKE ${} OR
                ru.middle_name ILIKE ${}
            )", param_counter, param_counter + 1, param_counter + 2, param_counter + 3, param_counter + 4, param_counter + 5, 
               param_counter + 6, param_counter + 7, param_counter + 8, param_counter + 9, param_counter + 10, param_counter + 11));
            for _ in 0..12 {
                params.push(format!("%{}%", search));
            }
            param_counter += 12;
        }
    }

    if let Some(ref confirmation) = filter.confirmation {
        query.push_str(&format!(" AND a.confirmation = ${}", param_counter));
        params.push(confirmation.clone());
        param_counter += 1;
    }

    if let Some(ref status) = filter.status {
        query.push_str(&format!(" AND a.status = ${}", param_counter));
        params.push(status.clone());
        param_counter += 1;
    }

    if let Some(date_from) = filter.date_from {
        query.push_str(&format!(" AND a.sending_datetime >= ${}", param_counter));
        params.push(date_from.and_hms_opt(0, 0, 0).unwrap().to_string());
        param_counter += 1;
    }

    if let Some(date_to) = filter.date_to {
        query.push_str(&format!(" AND a.sending_datetime <= ${}", param_counter));
        params.push(date_to.and_hms_opt(23, 59, 59).unwrap().to_string());
        param_counter += 1;
    }

    query.push_str(" ORDER BY a.sending_datetime DESC");

    log::debug!("SQL query for user applications: {}", query);

    let mut query_builder = sqlx::query(&query);
    
    for param in &params {
        query_builder = query_builder.bind(param);
    }

    let rows = query_builder
        .fetch_all(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to fetch user applications: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

    let applications: Vec<ApplicationWithDetails> = rows.iter().map(|row| {
        // Получаем DateTime<Utc> из БД
        let sending_datetime: DateTime<Utc> = row.try_get("sending_datetime")
            .unwrap_or_else(|_| Utc::now());
        
        let reading_datetime: Option<DateTime<Utc>> = row.try_get("reading_datetime").ok();
        let confirmation_datetime: Option<DateTime<Utc>> = row.try_get("confirmation_datetime").ok();

        ApplicationWithDetails {
            id: row.try_get("id").unwrap_or(0),
            application_number: row.try_get("application_number").unwrap_or_default(),
            confirmation: row.try_get("confirmation").unwrap_or_default(),
            sending_datetime,
            reading_datetime,
            confirmation_datetime,
            organization_id: row.try_get("organization_id").unwrap_or(0),
            organization_name: row.try_get("organization_name").unwrap_or_default(),
            company_id: row.try_get("company_id").ok(),
            company_name: row.try_get("company_name").unwrap_or_default(),
            sender_user_id: row.try_get("sender_user_id").unwrap_or(0),
            sender_full_name: row.try_get("sender_full_name").ok(),
            sender_name: row.try_get("sender_name").unwrap_or_default(),
            message: row.try_get("message").ok(),
            status: row.try_get("status").unwrap_or_default(),
            responsible_user_id: row.try_get("responsible_user_id").ok(),
            responsible_full_name: row.try_get("responsible_full_name").ok(),
            responsible_name: row.try_get("responsible_name").unwrap_or_default(),
            responsible_comment: row.try_get("responsible_comment").ok(),
            data_approval: row.try_get("data_approval").unwrap_or(false),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(applications))
}

/// Получение вложений для заявки с информацией о unique_attachments
pub async fn get_application_attachments(
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

    log::info!("Getting attachments for application: {}", application_id);

    #[derive(Debug, Serialize)]
    struct AttachmentInfo {
        id: i32,
        attachment_type: String,
        attachment_name: String,
        attachment_display_name: String,
        entry_date_from: Option<NaiveDate>,
        entry_date_to: Option<NaiveDate>,
        entry_time_from: Option<NaiveTime>,
        entry_time_to: Option<NaiveTime>,
        created_at: Option<NaiveDateTime>,
        unique_attachment_id: Option<i32>,
        unique_attachment_title: Option<String>,
        unique_attachment_display_name: Option<String>,
    }

    let rows = sqlx::query!(
        r#"
        SELECT 
            a.id,
            a.attachment_type,
            a.attachment_name,
            a.attachment_display_name,
            a.entry_date_from,
            a.entry_date_to,
            a.entry_time_from,
            a.entry_time_to,
            a.created_at,
            a.unique_attachment_id,
            ua.title as "unique_attachment_title?",
            ua.display_name as "unique_attachment_display_name?"
        FROM attachments a
        LEFT JOIN unique_attachments ua ON a.unique_attachment_id = ua.id
        WHERE a.application_id = $1
        ORDER BY ua.title, a.created_at
        "#,
        application_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch attachments: {}", e);
        error::ErrorInternalServerError("Error fetching attachments")
    })?;

    let attachments: Vec<AttachmentInfo> = rows.iter().map(|row| {
        AttachmentInfo {
            id: row.id,
            attachment_type: row.attachment_type.clone(),
            attachment_name: row.attachment_name.clone(),
            attachment_display_name: row.attachment_display_name.clone().unwrap_or_default(),
            entry_date_from: row.entry_date_from,
            entry_date_to: row.entry_date_to,
            entry_time_from: row.entry_time_from,
            entry_time_to: row.entry_time_to,
            created_at: row.created_at,
            unique_attachment_id: row.unique_attachment_id,
            unique_attachment_title: row.unique_attachment_title.clone(),
            unique_attachment_display_name: row.unique_attachment_display_name.clone(),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(attachments))
}

/// Получение автомобилей для вложения
pub async fn get_attachment_cars(
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

    let attachment_id = path.into_inner();

    #[derive(Debug, Serialize)]
    struct UnloadPlaceInfo {
        id: i32,
        name: String,
        description: Option<String>,
    }

    #[derive(Debug, Serialize)]
    struct CarWithPlaces {
        id: i32,
        car_number: String,
        car_brand: String,
        unload_place: Option<String>,
        entry_date_from: Option<NaiveDate>,
        entry_time_from: Option<NaiveTime>,
        entry_date_to: Option<NaiveDate>,
        entry_time_to: Option<NaiveTime>,
        unload_places: Vec<UnloadPlaceInfo>,
    }

    let cars = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.car_number,
            c.car_brand,
            c.unload_place,
            c.entry_date_from,
            c.entry_time_from,
            c.entry_date_to,
            c.entry_time_to
        FROM cars c
        WHERE c.attachment_id = $1
        "#,
        attachment_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch cars: {}", e);
        error::ErrorInternalServerError("Error fetching cars")
    })?;

    let mut car_with_places: Vec<CarWithPlaces> = Vec::new();

    for car in cars {
        let places = sqlx::query!(
            r#"
            SELECT up.id, up.name, up.description
            FROM car_unload_places cup
            JOIN unload_places up ON cup.unload_place_id = up.id
            WHERE cup.car_id = $1
            ORDER BY cup.order_index
            "#,
            car.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());

        let place_infos: Vec<UnloadPlaceInfo> = places.iter().map(|p| {
            UnloadPlaceInfo {
                id: p.id,
                name: p.name.clone(),
                description: p.description.clone(),
            }
        }).collect();

        car_with_places.push(CarWithPlaces {
            id: car.id,
            car_number: car.car_number,
            car_brand: car.car_brand,
            unload_place: car.unload_place,
            entry_date_from: Some(car.entry_date_from),
            entry_time_from: Some(car.entry_time_from),
            entry_date_to: Some(car.entry_date_to),
            entry_time_to: Some(car.entry_time_to),
            unload_places: place_infos,
        });
    }

    Ok(HttpResponse::Ok().json(car_with_places))
}

/// Получение сотрудников для вложения
pub async fn get_attachment_employees(
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

    let attachment_id = path.into_inner();

    #[derive(Debug, Serialize)]
    struct TableInfo {
        id: i32,
        name: String,
        display_name: String,
    }

    #[derive(Debug, Serialize)]
    struct EmployeeWithTables {
        id: i32,
        last_name: String,
        first_name: String,
        middle_name: Option<String>,
        position: Option<String>,
        citizenship_id: Option<i32>,
        passport_series_number: Option<String>,
        patent_number: Option<String>,
        other_permission: Option<String>,
        target_tables: Vec<TableInfo>,
    }

    let employees = sqlx::query!(
        r#"
        SELECT 
            e.id,
            e.last_name,
            e.first_name,
            e.middle_name,
            e.position,
            e.citizenship_id,
            e.passport_series_number,
            e.patent_number,
            e.other_permission
        FROM employees e
        WHERE e.attachment_id = $1
        "#,
        attachment_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employees: {}", e);
        error::ErrorInternalServerError("Error fetching employees")
    })?;

    let mut employee_with_tables: Vec<EmployeeWithTables> = Vec::new();

    for employee in employees {
        let tables = sqlx::query!(
            r#"
            SELECT st.id, st.name, st.display_name
            FROM employee_target_tables ett
            JOIN system_tables st ON ett.table_id = st.id
            WHERE ett.employee_id = $1
            ORDER BY ett.order_index
            "#,
            employee.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());

        let table_infos: Vec<TableInfo> = tables.iter().map(|t| {
            TableInfo {
                id: t.id,
                name: t.name.clone(),
                display_name: t.display_name.clone(),
            }
        }).collect();

        employee_with_tables.push(EmployeeWithTables {
            id: employee.id,
            last_name: employee.last_name,
            first_name: employee.first_name,
            middle_name: employee.middle_name,
            position: employee.position,
            citizenship_id: employee.citizenship_id,
            passport_series_number: employee.passport_series_number,
            patent_number: employee.patent_number,
            other_permission: employee.other_permission,
            target_tables: table_infos,
        });
    }

    Ok(HttpResponse::Ok().json(employee_with_tables))
}

/// Получение ТМЦ для вложения
pub async fn get_attachment_items(
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

    let attachment_id = path.into_inner();

    #[derive(Debug, Serialize)]
    struct ItemInfo {
        id: i32,
        name: String,
        count: i32,
        date_created: Option<NaiveDate>,
    }

    let items = sqlx::query!(
        r#"
        SELECT id, name, count, date_created
        FROM items
        WHERE attachment_id = $1
        ORDER BY id
        "#,
        attachment_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch items: {}", e);
        error::ErrorInternalServerError("Error fetching items")
    })?;

    let item_infos: Vec<ItemInfo> = items.iter().map(|row| {
        ItemInfo {
            id: row.id,
            name: row.name.clone(),
            count: row.count,
            date_created: row.date_created,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(item_infos))
}

/// Получение заявки по ID с расширенной информацией (включая ответственных с информацией о согласовании)
pub async fn get_application_details(
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

    log::info!("Getting application details by ID: {}", application_id);

    // Получаем основную информацию о заявке
    let application_row = sqlx::query!(
        r#"
        SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
            CONCAT(
                COALESCE(u.last_name, ''), ' ',
                COALESCE(u.first_name, ''), ' ',
                COALESCE(u.middle_name, '')
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''), ' ',
                COALESCE(ru.first_name, ''), ' ',
                COALESCE(ru.middle_name, '')
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE a.id = $1
        "#,
        application_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application = match application_row {
        Some(r) => {
            // Получаем список всех ответственных для этой заявки
            #[derive(Debug, Serialize)]
            struct ResponsibleUser {
                id: i32,
                username: String,
                last_name: Option<String>,
                first_name: Option<String>,
                middle_name: Option<String>,
                position: Option<String>,
                is_primary: bool,
                required_approval: bool,
                approval_status: Option<String>,
                approval_comment: Option<String>,
                approval_datetime: Option<DateTime<Utc>>,
            }

            let responsibles = sqlx::query_as!(
                ResponsibleUser,
                r#"
                SELECT 
                    u.id,
                    u.username,
                    u.last_name,
                    u.first_name,
                    u.middle_name,
                    u.position,
                    COALESCE(aru.is_primary, false) as "is_primary!",
                    COALESCE(aru.required_approval, false) as "required_approval!",
                    aru.approval_status,
                    aru.approval_comment,
                    aru.approval_datetime
                FROM application_responsible_users aru
                JOIN users u ON aru.user_id = u.id
                WHERE aru.application_id = $1
                ORDER BY aru.is_primary DESC, u.last_name, u.first_name
                "#,
                application_id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());

            #[derive(Debug, Serialize)]
            struct ApplicationDetails {
                id: i32,
                application_number: String,
                confirmation: String,
                sending_datetime: DateTime<Utc>,
                reading_datetime: Option<DateTime<Utc>>,
                confirmation_datetime: Option<DateTime<Utc>>,
                organization_id: i32,
                organization_name: String,
                company_id: Option<i32>,
                company_name: String,
                sender_user_id: i32,
                sender_full_name: Option<String>,
                sender_name: String,
                message: Option<String>,
                status: String,
                responsible_user_id: Option<i32>,
                responsible_full_name: Option<String>,
                responsible_name: String,
                responsible_comment: Option<String>,
                data_approval: bool,
                responsible_users: Vec<ResponsibleUser>,
            }

            let details = ApplicationDetails {
                id: r.id,
                application_number: r.application_number,
                confirmation: r.confirmation,
                sending_datetime: r.sending_datetime,
                reading_datetime: r.reading_datetime,
                confirmation_datetime: r.confirmation_datetime,
                organization_id: r.organization_id,
                organization_name: r.organization_name.unwrap_or_default(),
                company_id: r.company_id,
                company_name: r.company_name,
                sender_user_id: r.sender_user_id,
                sender_full_name: r.sender_full_name,
                sender_name: r.sender_name.unwrap_or_default(),
                message: r.message,
                status: r.status,
                responsible_user_id: r.responsible_user_id,
                responsible_full_name: r.responsible_full_name,
                responsible_name: r.responsible_name.unwrap_or_default(),
                responsible_comment: r.responsible_comment,
                data_approval: r.data_approval,
                responsible_users: responsibles,
            };

            serde_json::to_value(details)
                .map_err(|e| {
                    log::error!("Failed to serialize application: {}", e);
                    error::ErrorInternalServerError("Error serializing application")
                })?
        },
        None => return Err(error::ErrorNotFound("Application not found")),
    };

    Ok(HttpResponse::Ok().json(application))
}

// Структура для обновления статуса машины
#[derive(Debug, Deserialize)]
pub struct UpdateCarStatusRequest {
    pub status: i32,
}

// Структура для обновления статуса сотрудника
#[derive(Debug, Deserialize)]
pub struct UpdateEmployeeStatusRequest {
    pub status: i32,
}

/// Обновление статуса машины
pub async fn update_car_status(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    update_data: web::Json<UpdateCarStatusRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let car_id = path.into_inner();

    log::info!("Updating car {} status to {}", car_id, update_data.status);

    match sqlx::query!(
        "UPDATE cars SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
        update_data.status,
        car_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                Ok(HttpResponse::Ok().json(json!({
                    "success": true,
                    "message": "Car status updated successfully"
                })))
            } else {
                Ok(HttpResponse::NotFound().json(json!({
                    "success": false,
                    "message": "Car not found"
                })))
            }
        },
        Err(e) => {
            log::error!("Failed to update car status: {}", e);
            Err(error::ErrorInternalServerError("Error updating car status"))
        }
    }
}

/// Обновление статуса сотрудника
pub async fn update_employee_status(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    update_data: web::Json<UpdateEmployeeStatusRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let employee_id = path.into_inner();

    log::info!("Updating employee {} status to {}", employee_id, update_data.status);

    match sqlx::query!(
        "UPDATE employees SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
        update_data.status,
        employee_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                Ok(HttpResponse::Ok().json(json!({
                    "success": true,
                    "message": "Employee status updated successfully"
                })))
            } else {
                Ok(HttpResponse::NotFound().json(json!({
                    "success": false,
                    "message": "Employee not found"
                })))
            }
        },
        Err(e) => {
            log::error!("Failed to update employee status: {}", e);
            Err(error::ErrorInternalServerError("Error updating employee status"))
        }
    }
}

/// Обновление статусов всех машин и сотрудников в заявке
pub async fn update_application_items_status(
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

    log::info!("Updating statuses for all items in application {}", application_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем все вложения заявки
    let attachments = sqlx::query!(
        "SELECT id, attachment_type FROM attachments WHERE application_id = $1",
        application_id
    )
    .fetch_all(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch attachments: {}", e);
        error::ErrorInternalServerError("Error fetching attachments")
    })?;

    for attachment in attachments {
        match attachment.attachment_type.as_str() {
            "cars" => {
                // Обновляем статусы машин
                match sqlx::query!(
                    "UPDATE cars SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = $1",
                    attachment.id
                )
                .execute(&mut *transaction)
                .await {
                    Ok(result) => {
                        log::info!("Updated {} cars for attachment {}", result.rows_affected(), attachment.id);
                    },
                    Err(e) => {
                        log::error!("Failed to update cars status: {}", e);
                    }
                }
            },
            "people" => {
                // Обновляем статусы сотрудников
                match sqlx::query!(
                    "UPDATE employees SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = $1",
                    attachment.id
                )
                .execute(&mut *transaction)
                .await {
                    Ok(result) => {
                        log::info!("Updated {} employees for attachment {}", result.rows_affected(), attachment.id);
                    },
                    Err(e) => {
                        log::error!("Failed to update employees status: {}", e);
                    }
                }
            },
            _ => {} // Для других типов вложений ничего не делаем
        }
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "All items statuses updated successfully"
    })))
}

/// Обновление ответственных пользователей для заявок организации/компании
pub async fn update_responsible_users_for_entity(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    entity_id: i32,
    entity_type: String, // "organization" или "company"
    removed_user_ids: Vec<i32>,
    added_users: Vec<(i32, bool)>, // (user_id, is_primary)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Updating responsible users for {} {}: removed={:?}, added={:?}", 
               entity_type, entity_id, removed_user_ids, added_users);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to begin transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Получаем заявки организации/компании, которые можно обновлять
    let applications = sqlx::query!(
        r#"
        SELECT a.id, a.responsible_user_id
        FROM applications a
        WHERE (
            ($1 = 'organization' AND a.organization_id = $2)
            OR ($1 = 'company' AND a.company_id = $2)
        )
        AND a.status IN ('Непрочитано', 'В обработке')
        AND a.confirmation = 'Согласование'
        "#,
        entity_type,
        entity_id
    )
    .fetch_all(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch applications: {}", e);
        error::ErrorInternalServerError("Error fetching applications")
    })?;

    let mut updated_apps_count = 0;
    
    // Определяем нового главного (если есть)
    let new_primary_user = added_users.iter()
        .find(|&&(_, is_primary)| is_primary)
        .map(|&(user_id, _)| user_id);
    
    for app in applications {
        let application_id = app.id;
        let current_responsible_user_id = app.responsible_user_id;
        let mut app_updated = false;
        
        // 1. Обрабатываем удаленных пользователей
        for &user_id in &removed_user_ids {
            // Если удаляемый пользователь был главным в заявке
            if current_responsible_user_id == Some(user_id) {
                // Устанавливаем нового главного или NULL
                if let Some(new_primary_id) = new_primary_user {
                    let result = sqlx::query!(
                        "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
                        new_primary_id,
                        application_id
                    )
                    .execute(&mut *transaction)
                    .await;
                    
                    if let Ok(r) = result {
                        if r.rows_affected() > 0 {
                            app_updated = true;
                            log::debug!("Replaced primary responsible {} with {} in application {}", 
                                       user_id, new_primary_id, application_id);
                        }
                    }
                } else {
                    // Нет нового главного - сбрасываем поле
                    let result = sqlx::query!(
                        "UPDATE applications SET responsible_user_id = NULL WHERE id = $1",
                        application_id
                    )
                    .execute(&mut *transaction)
                    .await;
                    
                    if let Ok(r) = result {
                        if r.rows_affected() > 0 {
                            app_updated = true;
                            log::debug!("Removed primary responsible {} from application {}", 
                                       user_id, application_id);
                        }
                    }
                }
            }
            
            // Удаляем пользователя из списка ответственных
            let result = sqlx::query!(
                "DELETE FROM application_responsible_users WHERE application_id = $1 AND user_id = $2",
                application_id,
                user_id
            )
            .execute(&mut *transaction)
            .await;
            
            if let Ok(r) = result {
                if r.rows_affected() > 0 {
                    app_updated = true;
                    log::debug!("Removed user {} from application {}", 
                               user_id, application_id);
                }
            }
        }
        
        // 2. Добавляем/обновляем новых пользователей
        for &(user_id, is_primary) in &added_users {
            let result = sqlx::query!(
                r#"
                INSERT INTO application_responsible_users (application_id, user_id, is_primary, approval_status, created_at)
                VALUES ($1, $2, $3, 'pending', NOW())
                ON CONFLICT (application_id, user_id) 
                DO UPDATE SET is_primary = EXCLUDED.is_primary
                "#,
                application_id,
                user_id,
                is_primary
            )
            .execute(&mut *transaction)
            .await;
            
            if let Ok(r) = result {
                if r.rows_affected() > 0 {
                    app_updated = true;
                    log::debug!("Updated user {} in application {} with is_primary={}", 
                               user_id, application_id, is_primary);
                }
            }
            
            // Если это главный пользователь, обновляем поле в applications
            if is_primary && current_responsible_user_id != Some(user_id) {
                let result = sqlx::query!(
                    "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
                    user_id,
                    application_id
                )
                .execute(&mut *transaction)
                .await;
                
                if let Ok(r) = result {
                    if r.rows_affected() > 0 {
                        app_updated = true;
                        log::debug!("Set user {} as primary responsible for application {}", 
                                   user_id, application_id);
                    }
                }
            }
        }
        
        // 3. Если у заявки еще есть ответственный, но он не является главным среди added_users,
        //    обновляем его статус в application_responsible_users
        if let Some(current_resp_id) = current_responsible_user_id {
            // Проверяем, является ли этот пользователь главным среди добавленных
            let is_primary_in_added = added_users.iter()
                .any(|&(id, is_primary)| id == current_resp_id && is_primary);
            
            if !is_primary_in_added && !removed_user_ids.contains(&current_resp_id) {
                // Пользователь остался, но не как главный - обновляем его статус
                let result = sqlx::query!(
                    "UPDATE application_responsible_users SET is_primary = false WHERE application_id = $1 AND user_id = $2",
                    application_id,
                    current_resp_id
                )
                .execute(&mut *transaction)
                .await;
                
                if let Ok(r) = result {
                    if r.rows_affected() > 0 {
                        app_updated = true;
                        log::debug!("Set is_primary=false for existing user {} in application {}", 
                                   current_resp_id, application_id);
                        
                        // Устанавливаем нового главного (если есть)
                        if let Some(new_primary_id) = new_primary_user {
                            let set_result = sqlx::query!(
                                "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
                                new_primary_id,
                                application_id
                            )
                            .execute(&mut *transaction)
                            .await;
                            
                            if let Ok(_) = set_result {
                                log::debug!("Set new primary responsible {} for application {}", 
                                           new_primary_id, application_id);
                            }
                        }
                    }
                }
            }
        }
        
        if app_updated {
            updated_apps_count += 1;
        }
    }
    
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": format!("Responsible users updated for {} applications", updated_apps_count),
        "applications_updated": updated_apps_count
    })))
}

/// Обновление ответственных пользователей для заявок организации/компании с поддержкой обязательного согласования
pub async fn update_responsible_users_for_entity_with_required(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    entity_id: i32,
    entity_type: String, // "organization" или "company"
    removed_user_ids: Vec<i32>,
    added_users: Vec<(i32, bool, bool)>, // (user_id, is_primary, required_approval)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Updating responsible users for {} {}: removed={:?}, added={:?}", 
               entity_type, entity_id, removed_user_ids, added_users);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to begin transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Получаем заявки организации/компании, которые можно обновлять
    let applications = sqlx::query!(
        r#"
        SELECT a.id, a.responsible_user_id
        FROM applications a
        WHERE (
            ($1 = 'organization' AND a.organization_id = $2)
            OR ($1 = 'company' AND a.company_id = $2)
        )
        AND a.status IN ('Непрочитано', 'В обработке')
        AND a.confirmation = 'Согласование'
        "#,
        entity_type,
        entity_id
    )
    .fetch_all(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch applications: {}", e);
        error::ErrorInternalServerError("Error fetching applications")
    })?;

    let mut updated_apps_count = 0;
    
    // Определяем нового главного (если есть)
    let new_primary_user = added_users.iter()
        .find(|&&(_, is_primary, _)| is_primary)
        .map(|&(user_id, _, _)| user_id);
    
    for app in applications {
        let application_id = app.id;
        let current_responsible_user_id = app.responsible_user_id;
        let mut app_updated = false;
        
        // 1. Обрабатываем удаленных пользователей
        for &user_id in &removed_user_ids {
            // Если удаляемый пользователь был главным в заявке
            if current_responsible_user_id == Some(user_id) {
                // Устанавливаем нового главного или NULL
                if let Some(new_primary_id) = new_primary_user {
                    let result = sqlx::query!(
                        "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
                        new_primary_id,
                        application_id
                    )
                    .execute(&mut *transaction)
                    .await;
                    
                    if let Ok(r) = result {
                        if r.rows_affected() > 0 {
                            app_updated = true;
                            log::debug!("Replaced primary responsible {} with {} in application {}", 
                                       user_id, new_primary_id, application_id);
                        }
                    }
                } else {
                    // Нет нового главного - сбрасываем поле
                    let result = sqlx::query!(
                        "UPDATE applications SET responsible_user_id = NULL WHERE id = $1",
                        application_id
                    )
                    .execute(&mut *transaction)
                    .await;
                    
                    if let Ok(r) = result {
                        if r.rows_affected() > 0 {
                            app_updated = true;
                            log::debug!("Removed primary responsible {} from application {}", 
                                       user_id, application_id);
                        }
                    }
                }
            }
            
            // Удаляем пользователя из списка ответственных
            let result = sqlx::query!(
                "DELETE FROM application_responsible_users WHERE application_id = $1 AND user_id = $2",
                application_id,
                user_id
            )
            .execute(&mut *transaction)
            .await;
            
            if let Ok(r) = result {
                if r.rows_affected() > 0 {
                    app_updated = true;
                    log::debug!("Removed user {} from application {}", 
                               user_id, application_id);
                }
            }
        }
        
        // 2. Добавляем/обновляем новых пользователей
        for &(user_id, is_primary, required_approval) in &added_users {
            let result = sqlx::query!(
                r#"
                INSERT INTO application_responsible_users (
                    application_id, 
                    user_id, 
                    is_primary, 
                    required_approval,
                    approval_status,
                    created_at
                )
                VALUES ($1, $2, $3, $4, 'pending', NOW())
                ON CONFLICT (application_id, user_id) 
                DO UPDATE SET 
                    is_primary = EXCLUDED.is_primary,
                    required_approval = EXCLUDED.required_approval
                "#,
                application_id,
                user_id,
                is_primary,
                required_approval
            )
            .execute(&mut *transaction)
            .await;
            
            if let Ok(r) = result {
                if r.rows_affected() > 0 {
                    app_updated = true;
                    log::debug!("Updated user {} in application {} with is_primary={}, required_approval={}", 
                               user_id, application_id, is_primary, required_approval);
                }
            }
            
            // Если это главный пользователь, обновляем поле в applications
            if is_primary && current_responsible_user_id != Some(user_id) {
                let result = sqlx::query!(
                    "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
                    user_id,
                    application_id
                )
                .execute(&mut *transaction)
                .await;
                
                if let Ok(r) = result {
                    if r.rows_affected() > 0 {
                        app_updated = true;
                        log::debug!("Set user {} as primary responsible for application {}", 
                                   user_id, application_id);
                    }
                }
            }
        }
        
        // 3. Если у заявки еще есть ответственный, но он не является главным среди added_users,
        //    обновляем его статус в application_responsible_users
        if let Some(current_resp_id) = current_responsible_user_id {
            // Проверяем, является ли этот пользователь главным среди добавленных
            let is_primary_in_added = added_users.iter()
                .any(|&(id, is_primary, _)| id == current_resp_id && is_primary);
            
            if !is_primary_in_added && !removed_user_ids.contains(&current_resp_id) {
                // Находим информацию о required_approval для этого пользователя
                let user_required_approval = added_users.iter()
                    .find(|&&(id, _, _)| id == current_resp_id)
                    .map(|&(_, _, required_approval)| required_approval)
                    .unwrap_or(false);
                
                // Пользователь остался, но не как главный - обновляем его статус
                let result = sqlx::query!(
                    r#"
                    UPDATE application_responsible_users 
                    SET is_primary = false,
                        required_approval = $1
                    WHERE application_id = $2 AND user_id = $3
                    "#,
                    user_required_approval,
                    application_id,
                    current_resp_id
                )
                .execute(&mut *transaction)
                .await;
                
                if let Ok(r) = result {
                    if r.rows_affected() > 0 {
                        app_updated = true;
                        log::debug!("Set is_primary=false for existing user {} in application {}", 
                                   current_resp_id, application_id);
                        
                        // Устанавливаем нового главного (если есть)
                        if let Some(new_primary_id) = new_primary_user {
                            let set_result = sqlx::query!(
                                "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
                                new_primary_id,
                                application_id
                            )
                            .execute(&mut *transaction)
                            .await;
                            
                            if let Ok(_) = set_result {
                                log::debug!("Set new primary responsible {} for application {}", 
                                           new_primary_id, application_id);
                            }
                        }
                    }
                }
            }
        }
        
        if app_updated {
            updated_apps_count += 1;
            // Обновляем общий статус заявки на основе новых правил
            update_application_confirmation_based_on_approvals(&mut transaction, application_id).await?;
        }
    }
    
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": format!("Responsible users updated for {} applications", updated_apps_count),
        "applications_updated": updated_apps_count
    })))
}

// Добавьте эту функцию в applications.rs
async fn add_application_history_entry(
    pool: &PgPool,
    application_id: i32,
    user_id: i32,
    action_type: &str,
    old_value: Option<&str>,
    new_value: Option<&str>,
    comment: Option<&str>,
) -> Result<(), Error> {
    sqlx::query!(
        r#"
        INSERT INTO application_history (
            application_id,
            user_id,
            action_type,
            old_value,
            new_value,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        "#,
        application_id,
        user_id,
        action_type,
        old_value,
        new_value,
        comment
    )
    .execute(pool)
    .await
    .map_err(|e| {
        log::error!("Failed to add history entry: {}", e);
        error::ErrorInternalServerError("Error adding history entry")
    })?;

    Ok(())
}