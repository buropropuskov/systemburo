// handlers/user_types.rs
use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use log;

use crate::models::user_types::{UserTypeWithCount, CreateUserTypeRequest, UpdateUserTypeRequest};
use crate::auth::decode_token;

/// Получение всех типов пользователей с количеством пользователей
pub async fn get_user_types_with_count(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
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

                        // Используем ручное маппинг вместо query_as! для избежания проблем с Option
                        let types = sqlx::query!(
                            r#"
                            SELECT 
                                ut.id,
                                ut.name,
                                ut.code,
                                COUNT(u.username) as users_count
                            FROM user_types ut
                            LEFT JOIN users u ON ut.id = u.type_id
                            GROUP BY ut.id, ut.name, ut.code
                            ORDER BY ut.name
                            "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch user types: {}", e);
                            error::ErrorInternalServerError("Error fetching user types")
                        })?;

                        let types_with_count: Vec<UserTypeWithCount> = types
                            .into_iter()
                            .map(|record| UserTypeWithCount {
                                id: record.id,
                                name: record.name,
                                code: record.code,
                                users_count: record.users_count.unwrap_or(0) as i64,
                            })
                            .collect();

                        Ok(HttpResponse::Ok().json(types_with_count))
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

/// Создание нового типа пользователя
pub async fn create_user_type(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    type_data: web::Json<CreateUserTypeRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
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

                        // Проверяем, существует ли уже тип с таким кодом
                        let existing_type = sqlx::query!(
                            "SELECT id FROM user_types WHERE code = $1",
                            type_data.code
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error checking user type existence"))?;

                        if existing_type.is_some() {
                            return Err(error::ErrorBadRequest("User type with this code already exists"));
                        }

                        let type_record = sqlx::query!(
                            "INSERT INTO user_types (name, code) VALUES ($1, $2) RETURNING id",
                            type_data.name,
                            type_data.code
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create user type: {}", e);
                            error::ErrorInternalServerError("Error creating user type")
                        })?;

                        Ok(HttpResponse::Ok().json(serde_json::json!({
                            "id": type_record.id,
                            "message": "Тип пользователя успешно создан"
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

/// Обновление типа пользователя
pub async fn update_user_type_by_id(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
    type_data: web::Json<UpdateUserTypeRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
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

                        let type_id = path.into_inner();

                        // Проверяем существование типа
                        let existing_type = sqlx::query!(
                            "SELECT id FROM user_types WHERE id = $1",
                            type_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error checking user type existence"))?;

                        if existing_type.is_none() {
                            return Err(error::ErrorNotFound("User type not found"));
                        }

                        sqlx::query!(
                            "UPDATE user_types SET name = $1 WHERE id = $2",
                            type_data.name,
                            type_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to update user type: {}", e);
                            error::ErrorInternalServerError("Error updating user type")
                        })?;

                        Ok(HttpResponse::Ok().json("Тип пользователя успешно обновлен"))
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

/// Удаление типа пользователя
pub async fn delete_user_type(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
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

                        let type_id = path.into_inner();

                        // Проверяем, есть ли пользователи с этим типом
                        let users_count = sqlx::query!(
                            "SELECT COUNT(*) as count FROM users WHERE type_id = $1",
                            type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error checking users count"))?;

                        if users_count.count.unwrap_or(0) > 0 {
                            return Err(error::ErrorBadRequest("Cannot delete user type that has associated users"));
                        }

                        sqlx::query!(
                            "DELETE FROM user_types WHERE id = $1",
                            type_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete user type: {}", e);
                            error::ErrorInternalServerError("Error deleting user type")
                        })?;

                        Ok(HttpResponse::Ok().json("Тип пользователя успешно удален"))
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