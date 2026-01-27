use actix_web::{web, HttpResponse, Responder, HttpRequest, error, Error};
use sqlx::PgPool;
use serde_json::json;
use log;
use chrono::{Utc};

use crate::models::auth::*;
use crate::models::user_types::UserType;
use crate::auth::{hash_password, verify_password, create_token, decode_token, create_refresh_token, decode_refresh_token, hash_refresh_token};

/// Регистрация нового пользователя
pub async fn register(pool: web::Data<PgPool>, form: web::Json<UserRegister>) -> impl Responder {
    let hashed_password = hash_password(&form.password);
    let result = sqlx::query!(
        r#"INSERT INTO users 
           (username, password, organization_id, company_id, type_id, 
            last_name, first_name, middle_name, position, email, phone) 
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)"#,
        form.username,
        hashed_password,
        form.organization_id,
        form.company_id,
        form.type_id,
        form.last_name,
        form.first_name,
        form.middle_name,
        form.position,
        form.email,
        form.phone,
    )
    .execute(pool.get_ref())
    .await;

    match result {
        Ok(_) => HttpResponse::Ok().json("User registered successfully"),
        Err(e) => {
            if e.to_string().contains("users_username_key") {
                HttpResponse::BadRequest().json(json!({"message": "Пользователь с таким логином уже существует"}))
            } else {
                log::error!("Registration failed: {}", e);
                HttpResponse::InternalServerError().json("Registration failed")
            }
        }
    }
}

/// Аутентификация пользователя
pub async fn login(pool: web::Data<PgPool>, form: web::Json<UserLogin>) -> impl Responder {
    let user = sqlx::query!(
        r#"SELECT u.id, u.username, u.password, o.name as organization, 
                c.name as company, u.type_id, ut.code as user_type, 
                u.organization_id, u.company_id
         FROM users u 
         JOIN organizations o ON u.organization_id = o.id
         JOIN companies c ON u.company_id = c.id
         JOIN user_types ut ON u.type_id = ut.id
         WHERE u.username = $1"#,
        form.username
    )
    .fetch_one(pool.get_ref())
    .await;

    match user {
        Ok(user) => {
            if verify_password(&user.password, &form.password) {
                // Создаем токены
                let token = create_token(&user.username, user.type_id);
                let refresh_token = create_refresh_token(&user.username);
                let refresh_token_hash = hash_refresh_token(&refresh_token);
                
                // Сохраняем refresh token в БД
                let expires_at = Utc::now() + chrono::Duration::hours(24);
                
                let save_result = sqlx::query!(
                    r#"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) 
                       VALUES ($1, $2, $3)"#,
                    user.id,
                    refresh_token_hash,
                    expires_at
                )
                .execute(pool.get_ref())
                .await;
                
                match save_result {
                    Ok(_) => {
                        HttpResponse::Ok().json(LoginResponse {
                            token,
                            refreshToken: refresh_token,
                            organization: user.organization,
                            organization_id: user.organization_id,
                            company: user.company,
                            company_id: user.company_id,
                            type_id: user.type_id,
                            user_type: user.user_type,
                        })
                    }
                    Err(e) => {
                        log::error!("Failed to save refresh token: {}", e);
                        HttpResponse::InternalServerError().json("Login failed")
                    }
                }
            } else {
                HttpResponse::Unauthorized().json("Invalid credentials")
            }
        }
        Err(_) => HttpResponse::Unauthorized().json("User not found"),
    }
}

pub async fn refresh_token(pool: web::Data<PgPool>, form: web::Json<RefreshRequest>) -> impl Responder {
    let refresh_token = &form.refresh_token;
    
    println!("🔄 DEBUG: Starting refresh token process");
    println!("🔄 DEBUG: Refresh token received: {}", refresh_token);
    
    // 1. Декодируем refresh token
    let claims = match decode_refresh_token(refresh_token) {
        Ok(claims) => {
            println!("✅ DEBUG: Token decoded successfully - user: {}", claims.sub);
            claims
        },
        Err(e) => {
            println!("❌ DEBUG: Token decode failed: {}", e);
            return HttpResponse::Unauthorized().json("Token decode failed");
        }
    };
    
    // 2. Проверяем expiration
    let current_time = Utc::now().timestamp() as usize;
    println!("🔄 DEBUG: Token exp: {}, Current: {}", claims.exp, current_time);
    if claims.exp < current_time {
        println!("❌ DEBUG: Token expired");
        return HttpResponse::Unauthorized().json("Refresh token expired");
    }
    
    // 3. Находим пользователя
    let user = match sqlx::query!(
        r#"SELECT id, username, type_id FROM users WHERE username = $1"#,
        claims.sub
    )
    .fetch_one(pool.get_ref())
    .await {
        Ok(user) => {
            println!("✅ DEBUG: User found - ID: {}, Username: {}", user.id, user.username);
            user
        },
        Err(e) => {
            println!("❌ DEBUG: User not found: {}", e);
            return HttpResponse::Unauthorized().json("User not found");
        }
    };
    
    // 4. Хэшируем полученный токен для поиска в БД
    let refresh_token_hash = hash_refresh_token(refresh_token);
    println!("🔄 DEBUG: Hashed token for search: {}", &refresh_token_hash[..50]);
    
    // 5. Ищем конкретный токен в базе
    let stored_token = sqlx::query!(
        r#"SELECT id, token_hash, expires_at, is_revoked 
           FROM refresh_tokens 
           WHERE token_hash = $1 AND user_id = $2"#,
        refresh_token_hash,
        user.id
    )
    .fetch_optional(pool.get_ref())
    .await;
    
    match stored_token {
        Ok(Some(token_record)) => {
            println!("✅ DEBUG: Token found in DB, ID: {}", token_record.id);
            
            // Проверяем не отозван ли токен
            if token_record.is_revoked.unwrap_or(false) {
                println!("❌ DEBUG: Token revoked");
                return HttpResponse::Unauthorized().json("Refresh token revoked");
            }
            
            // Проверяем срок действия
            if token_record.expires_at < Utc::now() {
                println!("❌ DEBUG: Token expired in DB");
                return HttpResponse::Unauthorized().json("Refresh token expired");
            }
            
            println!("✅ DEBUG: Token validation successful");
            
            // УДАЛЯЕМ ТОЛЬКО ЭТОТ КОНКРЕТНЫЙ ТОКЕН
            let delete_result = sqlx::query!(
                "DELETE FROM refresh_tokens WHERE id = $1",
                token_record.id
            )
            .execute(pool.get_ref())
            .await;
            
            match delete_result {
                Ok(_) => println!("✅ DEBUG: Old token deleted completely"),
                Err(e) => {
                    println!("❌ DEBUG: Failed to delete old token: {}", e);
                    return HttpResponse::InternalServerError().json("Token refresh failed");
                }
            }
            
            // Создаем новые токены
            let new_token = create_token(&user.username, user.type_id);
            let new_refresh_token = create_refresh_token(&user.username);
            let new_refresh_token_hash = hash_refresh_token(&new_refresh_token);
            
            // Сохраняем новый refresh token
            let new_expires_at = Utc::now() + chrono::Duration::hours(24);
            
            let save_result = sqlx::query!(
                r#"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) 
                   VALUES ($1, $2, $3)"#,
                user.id,
                new_refresh_token_hash,
                new_expires_at
            )
            .execute(pool.get_ref())
            .await;
            
            match save_result {
                Ok(_) => {
                    println!("✅ DEBUG: New tokens created successfully");
                    HttpResponse::Ok().json(json!({
                        "token": new_token,
                        "refreshToken": new_refresh_token
                    }))
                }
                Err(e) => {
                    println!("❌ DEBUG: Failed to save new token: {}", e);
                    HttpResponse::InternalServerError().json("Token refresh failed")
                }
            }
        }
        Ok(None) => {
            // Токен не найден в базе
            println!("❌ DEBUG: Token not found in database");
            HttpResponse::Unauthorized().json("Invalid refresh token")
        }
        Err(e) => {
            println!("❌ DEBUG: Database error: {}", e);
            HttpResponse::InternalServerError().json("Server error")
        }
    }
}
pub async fn logout(pool: web::Data<PgPool>, req: HttpRequest, form: web::Json<LogoutRequest>) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|e| error::ErrorUnauthorized(format!("Invalid token: {}", e)))?;

    // Находим пользователя
    let user = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        claims.sub
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to find user: {}", e);
        error::ErrorInternalServerError("Error finding user")
    })?;

    // Хэшируем refresh token для поиска
    let refresh_token_hash = hash_refresh_token(&form.refresh_token);
    
    // УДАЛЯЕМ ТОЛЬКО КОНКРЕТНЫЙ REFRESH TOKEN, который был передан
    let delete_result = sqlx::query!(
        "DELETE FROM refresh_tokens WHERE token_hash = $1 AND user_id = $2",
        refresh_token_hash,
        user.id
    )
    .execute(pool.get_ref())
    .await;

    match delete_result {
        Ok(result) => {
            if result.rows_affected() > 0 {
                println!("✅ DEBUG: Refresh token deleted successfully for user: {}", user.id);
                Ok(HttpResponse::Ok().json("Logged out successfully"))
            } else {
                println!("⚠️ DEBUG: Refresh token not found for deletion, but proceeding with logout");
                Ok(HttpResponse::Ok().json("Logged out successfully"))
            }
        }
        Err(e) => {
            log::error!("Failed to delete refresh token: {}", e);
            Err(error::ErrorInternalServerError("Logout failed"))
        }
    }
}
/// Получение всех типов пользователей (для регистрации)
pub async fn get_user_types(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        UserType,
        "SELECT id, name, code FROM user_types ORDER BY id"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(types) => HttpResponse::Ok().json(types),
        Err(e) => {
            log::error!("Failed to fetch user types: {}", e);
            HttpResponse::InternalServerError().json("Error fetching user types")
        }
    }
}

/// Получение информации о текущем пользователе
// В методе get_current_user
pub async fn get_current_user(pool: web::Data<PgPool>, req: HttpRequest) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|e| error::ErrorUnauthorized(format!("Invalid token: {}", e)))?;

    let user = sqlx::query!(
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
            ut.code as user_type_code,
            u.last_name,
            u.first_name,
            u.middle_name,
            u.position,
            u.email,
            u.phone
        FROM users u 
        JOIN organizations o ON u.organization_id = o.id
        JOIN companies c ON u.company_id = c.id
        JOIN user_types ut ON u.type_id = ut.id
        WHERE u.username = $1
        "#,
        claims.sub
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Error fetching user")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "id": user.id,
        "username": user.username,
        "organization": user.organization,
        "organization_id": user.organization_id,
        "company": user.company,
        "company_id": user.company_id,
        "type_id": user.type_id,
        "user_type": user.user_type,
        "user_type_code": user.user_type_code,
        "last_name": user.last_name,
        "first_name": user.first_name,
        "middle_name": user.middle_name,
        "position": user.position,
        "email": user.email,
        "phone": user.phone
    })))
} 

/// Получение основных данных текущего пользователя
pub async fn get_current_user_data(pool: web::Data<PgPool>, req: HttpRequest) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|e| error::ErrorUnauthorized(format!("Invalid token: {}", e)))?;

    let user = sqlx::query!(
        r#"
        SELECT 
            u.username,
            o.name as organization,
            u.organization_id,
            c.name as company,
            u.company_id,
            u.last_name,
            u.first_name,
            u.middle_name,
            u.phone
        FROM users u 
        JOIN organizations o ON u.organization_id = o.id
        JOIN companies c ON u.company_id = c.id
        WHERE u.username = $1
        "#,
        claims.sub
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user data: {}", e);
        error::ErrorInternalServerError("Error fetching user data")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "username": user.username,
        "organization": user.organization,
        "organization_id": user.organization_id,
        "company": user.company,
        "company_id": user.company_id,
        "last_name": user.last_name,
        "first_name": user.first_name,
        "middle_name": user.middle_name,
        "phone": user.phone
    })))
}