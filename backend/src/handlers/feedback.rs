// handlers/feedback.rs
use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use log;
use chrono::{Utc, NaiveDateTime};

use crate::models::feedback::*;
use crate::models::feedback::FeedbackWithUser;

use crate::auth::decode_token;

/// Создание нового обращения
/// Создание нового обращения
pub async fn create_feedback(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    feedback_data: web::Json<CreateFeedbackRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Получаем ID пользователя
                        let user_record = sqlx::query!(
                            r#"SELECT id FROM users WHERE username = $1"#,
                            claims.sub
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch user: {}", e);
                            error::ErrorInternalServerError("Error fetching user")
                        })?;

                        let user_id = match user_record {
                            Some(record) => record.id,
                            None => return Err(error::ErrorUnauthorized("User not found")),
                        };

                        // Проверяем минимальную длину сообщения
                        if feedback_data.message.trim().len() < 10 {
                            return Err(error::ErrorBadRequest("Message must be at least 10 characters"));
                        }

                        // Проверяем максимальную длину сообщения
                        if feedback_data.message.len() > 1000 {
                            return Err(error::ErrorBadRequest("Message cannot exceed 1000 characters"));
                        }

                        let now = Utc::now().naive_utc();

                        let feedback_record = sqlx::query!(
                            r#"
                            INSERT INTO feedback (user_id, message, status, is_read, created_at, updated_at)
                            VALUES ($1, $2, $3, $4, $5, $6)
                            RETURNING id
                            "#,
                            user_id,
                            feedback_data.message.trim(),
                            "Нерешено",
                            false, // is_read = false по умолчанию
                            now,
                            now
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create feedback: {}", e);
                            error::ErrorInternalServerError("Error creating feedback")
                        })?;

                        Ok(HttpResponse::Ok().json(serde_json::json!({
                            "id": feedback_record.id,
                            "message": "Сообщение отправлено успешно"
                        })))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

/// Получение всех обращений (для администраторов)
/// Получение всех обращений (для администраторов)
pub async fn get_all_feedback(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверяем права администратора
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let feedbacks = sqlx::query!(
                            r#"
                            SELECT 
                                f.id,
                                f.user_id,
                                CONCAT(u.last_name, ' ', u.first_name) as user_name,
                                f.message,
                                f.status,
                                f.is_read,
                                f.created_at,
                                f.updated_at
                            FROM feedback f
                            JOIN users u ON f.user_id = u.id
                            ORDER BY f.created_at DESC
                            "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch feedback: {}", e);
                            error::ErrorInternalServerError("Error fetching feedback")
                        })?;

                        let feedbacks_with_user: Vec<FeedbackWithUser> = feedbacks
                            .into_iter()
                            .map(|record| FeedbackWithUser {
                                id: record.id,
                                user_id: record.user_id,
                                user_name: record.user_name.unwrap_or("Неизвестный пользователь".to_string()),
                                message: record.message,
                                status: record.status,
                                is_read: record.is_read,
                                created_at: record.created_at,
                                updated_at: record.updated_at,
                            })
                            .collect();

                        Ok(HttpResponse::Ok().json(feedbacks_with_user))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

/// Получение статистики по обращениям
/// Получение статистики по обращениям
pub async fn get_feedback_stats(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверяем права администратора
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let stats = sqlx::query!(
                            r#"
                            SELECT 
                                COUNT(*) as total,
                                COUNT(CASE WHEN status = 'Решено' THEN 1 END) as resolved,
                                COUNT(CASE WHEN status = 'Нерешено' THEN 1 END) as unresolved,
                                COUNT(CASE WHEN is_read = false THEN 1 END) as unread
                            FROM feedback
                            "#
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch feedback stats: {}", e);
                            error::ErrorInternalServerError("Error fetching feedback stats")
                        })?;

                        let feedback_stats = FeedbackStats {
                            total: stats.total.unwrap_or(0) as i64,
                            resolved: stats.resolved.unwrap_or(0) as i64,
                            unresolved: stats.unresolved.unwrap_or(0) as i64,
                            unread: stats.unread.unwrap_or(0) as i64,
                        };

                        Ok(HttpResponse::Ok().json(feedback_stats))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}
/// Обновление статуса обращения
pub async fn update_feedback_status(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
    status_data: web::Json<UpdateFeedbackStatusRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверяем права администратора
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let feedback_id = path.into_inner();
                        let now = Utc::now().naive_utc();

                        // Проверяем существование обращения
                        let existing_feedback = sqlx::query!(
                            "SELECT id FROM feedback WHERE id = $1",
                            feedback_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error checking feedback existence"))?;

                        if existing_feedback.is_none() {
                            return Err(error::ErrorNotFound("Feedback not found"));
                        }

                        // Проверяем допустимость статуса
                        if status_data.status != "Решено" && status_data.status != "Нерешено" {
                            return Err(error::ErrorBadRequest("Invalid status. Must be 'Решено' or 'Нерешено'"));
                        }

                        sqlx::query!(
                            r#"
                            UPDATE feedback 
                            SET status = $1, updated_at = $2 
                            WHERE id = $3
                            "#,
                            status_data.status,
                            now,
                            feedback_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to update feedback status: {}", e);
                            error::ErrorInternalServerError("Error updating feedback status")
                        })?;

                        Ok(HttpResponse::Ok().json("Статус обращения успешно обновлен"))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

/// Получение обращений текущего пользователя
/// Получение обращений текущего пользователя
pub async fn get_my_feedback(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Получаем ID пользователя
                        let user_record = sqlx::query!(
                            r#"SELECT id FROM users WHERE username = $1"#,
                            claims.sub
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch user: {}", e);
                            error::ErrorInternalServerError("Error fetching user")
                        })?;

                        let user_id = match user_record {
                            Some(record) => record.id,
                            None => return Err(error::ErrorUnauthorized("User not found")),
                        };

                        let feedbacks = sqlx::query!(
                            r#"
                            SELECT 
                                id,
                                user_id,
                                message,
                                status,
                                is_read,
                                created_at,
                                updated_at
                            FROM feedback
                            WHERE user_id = $1
                            ORDER BY created_at DESC
                            "#,
                            user_id
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch user feedback: {}", e);
                            error::ErrorInternalServerError("Error fetching user feedback")
                        })?;

                        let my_feedbacks: Vec<MyFeedback> = feedbacks
                            .into_iter()
                            .map(|record| MyFeedback {
                                id: record.id,
                                user_id: record.user_id,
                                message: record.message,
                                status: record.status,
                                is_read: record.is_read,
                                created_at: record.created_at,
                                updated_at: record.updated_at,
                            })
                            .collect();

                        Ok(HttpResponse::Ok().json(my_feedbacks))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

/// Отметить обращение как прочитанное/непрочитанное
pub async fn mark_feedback_as_read(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
    read_data: web::Json<MarkAsReadRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверяем права администратора
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let feedback_id = path.into_inner();
                        let now = Utc::now().naive_utc();

                        // Обновляем только is_read, но не updated_at для read/unread
                        sqlx::query!(
                            r#"
                            UPDATE feedback 
                            SET is_read = $1 
                            WHERE id = $2
                            "#,
                            read_data.is_read,
                            feedback_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to update feedback read status: {}", e);
                            error::ErrorInternalServerError("Error updating feedback read status")
                        })?;

                        Ok(HttpResponse::Ok().json("Статус прочтения обновлен"))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}