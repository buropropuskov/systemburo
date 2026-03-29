use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;

use crate::models::users::{
    UserInfo,
    UpdatePasswordRequest, UpdateOrganizationRequest,
    UpdateCompanyRequest, UpdateUserRequest, UpdateUserTypeRequest
};

use crate::auth::{hash_password, decode_token};
use crate::handlers::notifications::{create_notification};
use crate::models::notifications::CreateNotificationRequest;

pub async fn update_user_type(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateUserTypeRequest>,
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

                        let username = path.into_inner();

                        let type_exists = sqlx::query!(
                            "SELECT EXISTS(SELECT 1 FROM user_types WHERE id = $1) as exists",
                            form.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error checking user type"))?;

                        if !type_exists.exists.unwrap_or(false) {
                            return Err(error::ErrorBadRequest("Invalid user type"));
                        }

                        // Получаем название нового типа
                        let type_name = sqlx::query!(
                            "SELECT name FROM user_types WHERE id = $1",
                            form.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error fetching user type name"))?
                        .name;

                        sqlx::query!(
                            "UPDATE users SET type_id = $1 WHERE username = $2",
                            form.type_id,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating user type"))?;

                        let user_row = sqlx::query!(
                            "SELECT id FROM users WHERE username = $1",
                            username
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("User not found"))?;

                        let notif_req = CreateNotificationRequest {
                            user_id: user_row.id,
                            type_: "user_type_change".to_string(),
                            title: "Изменение типа пользователя".to_string(),
                            message: format!("Ваш тип пользователя изменён на \"{}\".", type_name),
                            data: None,
                        };
                        create_notification(pool.get_ref(), notif_req).await?;

                        Ok(HttpResponse::Ok().json("User type updated successfully"))
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

pub async fn update_user_organization(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateOrganizationRequest>,
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

                        let username = path.into_inner();

                        let new_org_id = form.organization_id;
                        let org_name = if let (org_id) = new_org_id {
                            sqlx::query!(
                                "SELECT name FROM organizations WHERE id = $1",
                                org_id
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|_| error::ErrorInternalServerError("Error fetching organization name"))?
                            .map(|row| row.name)
                        } else {
                            None
                        };

                        sqlx::query!(
                            "UPDATE users SET organization_id = $1 WHERE username = $2",
                            new_org_id,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating organization"))?;

                        let user_row = sqlx::query!(
                            "SELECT id FROM users WHERE username = $1",
                            username
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("User not found"))?;

                        let message = if let Some(name) = org_name {
                            format!("Ваша организация / отдел изменено на \"{}\".", name)
                        } else {
                            "Ваша организация изменена (организация не указана).".to_string()
                        };

                        let notif_req = CreateNotificationRequest {
                            user_id: user_row.id,
                            type_: "organization_change".to_string(),
                            title: "Смена организации".to_string(),
                            message,
                            data: None,
                        };
                        create_notification(pool.get_ref(), notif_req).await?;

                        Ok(HttpResponse::Ok().json("Organization updated successfully"))
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

pub async fn update_user_company(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateCompanyRequest>,
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

                        let username = path.into_inner();

                        let new_company_id = form.company_id;
                        let company_name = if let (comp_id) = new_company_id {
                            sqlx::query!(
                                "SELECT name FROM companies WHERE id = $1",
                                comp_id
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|_| error::ErrorInternalServerError("Error fetching company name"))?
                            .map(|row| row.name)
                        } else {
                            None
                        };

                        sqlx::query!(
                            "UPDATE users SET company_id = $1 WHERE username = $2",
                            new_company_id,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating company"))?;

                        let user_row = sqlx::query!(
                            "SELECT id FROM users WHERE username = $1",
                            username
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("User not found"))?;

                        let message = if let Some(name) = company_name {
                            format!("Ваша компания изменена на \"{}\".", name)
                        } else {
                            "Ваша компания изменена (компания не указана).".to_string()
                        };

                        let notif_req = CreateNotificationRequest {
                            user_id: user_row.id,
                            type_: "company_change".to_string(),
                            title: "Смена компании".to_string(),
                            message,
                            data: None,
                        };
                        create_notification(pool.get_ref(), notif_req).await?;

                        Ok(HttpResponse::Ok().json("Company updated successfully"))
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

pub async fn get_all_users(pool: web::Data<PgPool>, req: HttpRequest) -> Result<HttpResponse, Error> {
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

                        let users = sqlx::query_as!(
                            UserInfo,
                            r#"
                            SELECT 
                                u.id,
                                u.username, 
                                o.name as organization, 
                                u.organization_id, 
                                c.name as company,
                                u.company_id,
                                u.type_id,
                                ut.name as user_type,
                                u.last_name,
                                u.first_name,
                                u.middle_name,
                                u.position,
                                u.email,
                                u.phone
                            FROM users u 
                            LEFT JOIN organizations o ON u.organization_id = o.id
                            LEFT JOIN companies c ON u.company_id = c.id
                            LEFT JOIN user_types ut ON u.type_id = ut.id
                            ORDER BY u.username
                            "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch users: {}", e);
                            error::ErrorInternalServerError("Error fetching users")
                        })?;

                        Ok(HttpResponse::Ok().json(users))
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

pub async fn update_user_password(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdatePasswordRequest>,
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

                        let username = path.into_inner();
                        let hashed_password = hash_password(&form.password);

                        sqlx::query!(
                            "UPDATE users SET password = $1 WHERE username = $2",
                            hashed_password,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating password"))?;

                        let user_row = sqlx::query!(
                            "SELECT id FROM users WHERE username = $1",
                            username
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("User not found"))?;

                        let notif_req = CreateNotificationRequest {
                            user_id: user_row.id,
                            type_: "password_change".to_string(),
                            title: "Изменение пароля".to_string(),
                            message: "Ваш пароль от учётной записи был изменён.".to_string(),
                            data: None,
                        };
                        create_notification(pool.get_ref(), notif_req).await?;

                        Ok(HttpResponse::Ok().json("Password updated successfully"))
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

pub async fn update_user_info(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateUserRequest>,
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

                        let username = path.into_inner();

                        sqlx::query!(
                            r#"UPDATE users SET 
                                last_name = $1, 
                                first_name = $2, 
                                middle_name = $3, 
                                position = $4, 
                                email = $5, 
                                phone = $6 
                            WHERE username = $7"#,
                            form.last_name,
                            form.first_name,
                            form.middle_name,
                            form.position,
                            form.email,
                            form.phone,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating user info"))?;

                        Ok(HttpResponse::Ok().json("User info updated successfully"))
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

pub async fn delete_user(pool: web::Data<PgPool>, path: web::Path<String>, req: HttpRequest) -> Result<HttpResponse, Error> {
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

                        let username = path.into_inner();

                        sqlx::query!(
                            "DELETE FROM users WHERE username = $1",
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete user: {}", e);
                            error::ErrorInternalServerError("Error deleting user")
                        })?;

                        Ok(HttpResponse::Ok().json(json!({"message": "User deleted successfully"})))
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

pub async fn get_this_user(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query_as!(
                            UserInfo,
                            r#"
                            SELECT 
                                u.id,
                                u.username, 
                                COALESCE(o.name::text, '') as organization, 
                                u.organization_id, 
                                COALESCE(c.name::text, '') as company,
                                u.company_id,
                                u.type_id,
                                ut.name as user_type,
                                u.last_name,
                                u.first_name,
                                u.middle_name,
                                u.position,
                                u.email,
                                u.phone
                            FROM users u 
                            LEFT JOIN organizations o ON u.organization_id = o.id
                            LEFT JOIN companies c ON u.company_id = c.id
                            LEFT JOIN user_types ut ON u.type_id = ut.id
                            WHERE u.username = $1
                            "#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch current user: {}, error: {}", claims.sub, e);
                            error::ErrorInternalServerError("Error fetching user data")
                        })?;

                        log::info!("User data fetched for {}: company_id={:?}, organization_id={:?}", 
                                  user.username, user.company_id, user.organization_id);
                        
                        Ok(HttpResponse::Ok().json(user))
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