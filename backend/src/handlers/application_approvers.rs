use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;

use crate::models::application_approvers::{
    ApplicationApprover, ApplicationApproverWithUser, CreateApproverRequest
};
use crate::auth::decode_token;

#[derive(Debug, serde::Serialize)]
struct AvailableUser {
    pub id: i32,
    pub username: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub organization: Option<String>,
    pub company: Option<String>,
}

/// Получение всех принимающих
pub async fn get_application_approvers(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав
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

                        let approvers = sqlx::query_as!(
                            ApplicationApproverWithUser,
                            r#"
                            SELECT 
                                a.id,
                                a.user_id,
                                u.username,
                                u.last_name,
                                u.first_name,
                                u.middle_name,
                                u.position,
                                o.name as organization,
                                c.name as company,
                                a.created_at
                            FROM application_approvers a
                            JOIN users u ON a.user_id = u.id
                            LEFT JOIN organizations o ON u.organization_id = o.id
                            LEFT JOIN companies c ON u.company_id = c.id
                            ORDER BY u.last_name, u.first_name
                            "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch approvers: {}", e);
                            error::ErrorInternalServerError("Error fetching approvers")
                        })?;

                        Ok(HttpResponse::Ok().json(approvers))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

/// Добавление принимающего
pub async fn create_application_approver(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<CreateApproverRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав
                        let current_user = sqlx::query!(
                            r#"SELECT u.id, ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if current_user.user_type != "manager" && current_user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        // Проверяем, существует ли пользователь
                        let user_exists = sqlx::query!(
                            "SELECT id FROM users WHERE id = $1",
                            form.user_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Error checking user: {}", e);
                            error::ErrorInternalServerError("Error checking user")
                        })?;

                        if user_exists.is_none() {
                            return Err(error::ErrorBadRequest("User not found"));
                        }

                        // Проверяем, не добавлен ли уже
                        let existing = sqlx::query!(
                            "SELECT id FROM application_approvers WHERE user_id = $1",
                            form.user_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Error checking existing: {}", e);
                            error::ErrorInternalServerError("Error checking existing")
                        })?;

                        if existing.is_some() {
                            return Err(error::ErrorBadRequest("User is already an approver"));
                        }

                        // Добавляем
                        sqlx::query!(
                            r#"
                            INSERT INTO application_approvers (user_id, created_by)
                            VALUES ($1, $2)
                            "#,
                            form.user_id,
                            current_user.id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create approver: {}", e);
                            error::ErrorInternalServerError("Error creating approver")
                        })?;

                        Ok(HttpResponse::Created().json(json!({"message": "Approver added successfully"})))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

/// Удаление принимающего
pub async fn delete_application_approver(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    let approver_id = path.into_inner();

    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав
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

                        let result = sqlx::query!(
                            "DELETE FROM application_approvers WHERE id = $1",
                            approver_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete approver: {}", e);
                            error::ErrorInternalServerError("Error deleting approver")
                        })?;

                        if result.rows_affected() == 0 {
                            return Ok(HttpResponse::NotFound().json(json!({"error": "Approver not found"})));
                        }

                        Ok(HttpResponse::Ok().json(json!({"message": "Approver deleted successfully"})))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

/// Получение пользователей, которых можно добавить
pub async fn get_available_users_for_approvers(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав
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

                        let records = sqlx::query!(
                            r#"
                            SELECT 
                                u.id,
                                u.username,
                                u.last_name,
                                u.first_name,
                                u.middle_name,
                                u.position,
                                o.name as organization,
                                c.name as company
                            FROM users u
                            LEFT JOIN organizations o ON u.organization_id = o.id
                            LEFT JOIN companies c ON u.company_id = c.id
                            WHERE NOT EXISTS (
                                SELECT 1 FROM application_approvers a WHERE a.user_id = u.id
                            )
                            ORDER BY u.last_name, u.first_name
                            "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch available users: {}", e);
                            error::ErrorInternalServerError("Error fetching available users")
                        })?;

                        // Преобразуем в сериализуемую структуру
                        let users: Vec<AvailableUser> = records.into_iter().map(|r| AvailableUser {
                            id: r.id,
                            username: r.username,
                            last_name: r.last_name,
                            first_name: r.first_name,
                            middle_name: r.middle_name,
                            position: r.position,
                            organization: Some(r.organization),
                            company: Some(r.company),
                        }).collect();

                        Ok(HttpResponse::Ok().json(users))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}