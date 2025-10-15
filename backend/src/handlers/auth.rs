use actix_web::{web, HttpResponse, Responder, HttpRequest, error, Error};
use sqlx::PgPool;
use serde_json::json;
use log;

use crate::models::auth::{UserLogin, LoginResponse};
use crate::models::users::{UserType, UserInfo, UserData};
use crate::models::users::UserRegister;
use crate::auth::{hash_password, verify_password, create_token, decode_token};

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

pub async fn login(pool: web::Data<PgPool>, form: web::Json<UserLogin>) -> impl Responder {
    let user = sqlx::query!(
        r#"SELECT u.username, u.password, o.name as organization, 
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
                let token = create_token(&user.username, user.type_id);
                HttpResponse::Ok().json(LoginResponse {
                    token,
                    organization: user.organization,
                    organization_id: user.organization_id,
                    company: user.company,
                    company_id: user.company_id,
                    type_id: user.type_id,
                    user_type: user.user_type,
                })
            } else {
                HttpResponse::Unauthorized().json("Invalid credentials")
            }
        }
        Err(_) => HttpResponse::Unauthorized().json("User not found"),
    }
}

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

pub async fn get_current_user(pool: web::Data<PgPool>, req: HttpRequest) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|e| error::ErrorUnauthorized(format!("Invalid token: {}", e)))?;

    let user = sqlx::query_as!(
         UserInfo,
    r#"
    SELECT 
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

    Ok(HttpResponse::Ok().json(user))
}

pub async fn get_current_user_data(pool: web::Data<PgPool>, req: HttpRequest) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|e| error::ErrorUnauthorized(format!("Invalid token: {}", e)))?;

    let user = sqlx::query_as!(
        UserData,
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

    Ok(HttpResponse::Ok().json(user))
}
