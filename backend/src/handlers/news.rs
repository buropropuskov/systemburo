use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;

use crate::models::news::{
    NewsItem, AnnouncementItem, CreateNewsRequest, UpdateNewsRequest,
    CreateAnnouncementRequest, UpdateAnnouncementRequest, SetActiveAnnouncementRequest
};
use crate::auth::decode_token;

// ==================== НОВОСТИ ====================

pub async fn create_news(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<CreateNewsRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            "SELECT id FROM users WHERE username = $1",
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        let full_text = form.full_text.clone().unwrap_or_else(|| form.description.clone());

                        let news = sqlx::query!(
                            r#"
                            INSERT INTO news (title, description, full_text, created_by)
                            VALUES ($1, $2, $3, $4)
                            RETURNING 
                                id, title, description, full_text, created_by, created_at, is_active
                            "#,
                            form.title,
                            form.description,
                            full_text,
                            user.id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create news: {}", e);
                            error::ErrorInternalServerError("Error creating news")
                        })?;

                        let creator = sqlx::query!(
                            "SELECT first_name, last_name FROM users WHERE id = $1",
                            user.id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .ok();

                        let news_item = NewsItem {
                            id: news.id,
                            title: news.title,
                            description: news.description,
                            full_text: news.full_text.unwrap_or_default(),
                            created_by: news.created_by,
                            created_by_name: creator.and_then(|c| {
                                Some(format!(
                                    "{} {}", 
                                    c.first_name.unwrap_or_default(),
                                    c.last_name.unwrap_or_default()
                                ).trim().to_string())
                            }),
                            created_at: news.created_at,
                            updated_at: None,
                            updated_by: None,
                            updated_by_name: None,
                            is_active: news.is_active,
                        };

                        Ok(HttpResponse::Ok().json(news_item))
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

pub async fn get_news(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                if decode_token(&token).is_err() {
                    return Err(error::ErrorUnauthorized("Invalid token"));
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid token"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid token"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing token"));
    }

    let news = sqlx::query!(
        r#"
        SELECT 
            n.id, n.title, n.description, n.full_text, n.created_by,
            CONCAT(u.first_name, ' ', u.last_name) as created_by_name,
            n.created_at, n.updated_at, n.updated_by,
            CONCAT(u2.first_name, ' ', u2.last_name) as updated_by_name,
            n.is_active
        FROM news n
        LEFT JOIN users u ON n.created_by = u.id
        LEFT JOIN users u2 ON n.updated_by = u2.id
        WHERE n.is_active = true
        ORDER BY n.created_at DESC
        "#
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch news: {}", e);
        error::ErrorInternalServerError("Error fetching news")
    })?;

    let news_items: Vec<NewsItem> = news.into_iter().map(|row| {
        NewsItem {
            id: row.id,
            title: row.title,
            description: row.description,
            full_text: row.full_text.unwrap_or_default(),
            created_by: row.created_by,
            created_by_name: row.created_by_name,
            created_at: row.created_at,
            updated_at: row.updated_at,
            updated_by: row.updated_by,
            updated_by_name: row.updated_by_name,
            is_active: row.is_active,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(news_items))
}

pub async fn get_all_news_for_manage(
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

                        let news = sqlx::query!(
                            r#"
                            SELECT 
                                n.id, n.title, n.description, n.full_text, n.created_by,
                                CONCAT(u.first_name, ' ', u.last_name) as created_by_name,
                                n.created_at, n.updated_at, n.updated_by,
                                CONCAT(u2.first_name, ' ', u2.last_name) as updated_by_name,
                                n.is_active
                            FROM news n
                            LEFT JOIN users u ON n.created_by = u.id
                            LEFT JOIN users u2 ON n.updated_by = u2.id
                            ORDER BY n.created_at DESC
                            "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch all news: {}", e);
                            error::ErrorInternalServerError("Error fetching news")
                        })?;

                        let news_items: Vec<NewsItem> = news.into_iter().map(|row| {
                            NewsItem {
                                id: row.id,
                                title: row.title,
                                description: row.description,
                                full_text: row.full_text.unwrap_or_default(),
                                created_by: row.created_by,
                                created_by_name: row.created_by_name,
                                created_at: row.created_at,
                                updated_at: row.updated_at,
                                updated_by: row.updated_by,
                                updated_by_name: row.updated_by_name,
                                is_active: row.is_active,
                            }
                        }).collect();

                        Ok(HttpResponse::Ok().json(news_items))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}

pub async fn update_news(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
    form: web::Json<UpdateNewsRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type, u.id
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

                        let news_id = path.into_inner();

                        sqlx::query!(
                            r#"
                            UPDATE news SET
                                title = COALESCE($1, title),
                                description = COALESCE($2, description),
                                full_text = COALESCE($3, full_text),
                                is_active = COALESCE($4, is_active),
                                updated_at = NOW(),
                                updated_by = $5
                            WHERE id = $6
                            "#,
                            form.title,
                            form.description,
                            form.full_text,
                            form.is_active,
                            user.id,
                            news_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Error updating news: {}", e);
                            error::ErrorInternalServerError("Error updating news")
                        })?;

                        Ok(HttpResponse::Ok().json("News updated successfully"))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}

pub async fn delete_news(
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

                        let news_id = path.into_inner();

                        sqlx::query!(
                            "DELETE FROM news WHERE id = $1",
                            news_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error deleting news"))?;

                        Ok(HttpResponse::Ok().json(json!({"message": "News deleted successfully"})))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}

// ==================== ОБЪЯВЛЕНИЯ ====================

pub async fn create_announcement(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<CreateAnnouncementRequest>,
) -> Result<HttpResponse, Error> {
    log::info!("Received announcement data: {:?}", form);
    if form.title.trim().is_empty() {
        return Err(error::ErrorBadRequest("Title is required"));
    }
    if form.description.trim().is_empty() {
        return Err(error::ErrorBadRequest("Description is required"));
    }
    
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            "SELECT id FROM users WHERE username = $1",
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        let full_text = form.full_text.clone().unwrap_or_else(|| form.description.clone());

                        let active_exists = sqlx::query!(
                            "SELECT EXISTS(SELECT 1 FROM announcements WHERE is_active = true) as exists"
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to check active announcement: {}", e);
                            error::ErrorInternalServerError("Database error")
                        })?;
                        
                        let is_active = !active_exists.exists.unwrap_or(false);
                        
                        let announcement = sqlx::query!(
                            r#"
                            INSERT INTO announcements (title, description, full_text, is_important, created_by, is_active)
                            VALUES ($1, $2, $3, $4, $5, $6)
                            RETURNING 
                                id, title, description, full_text, is_important, is_active, created_by, created_at
                            "#,
                            form.title,
                            form.description,
                            full_text,
                            form.is_important,
                            user.id,
                            is_active
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create announcement: {}", e);
                            error::ErrorInternalServerError("Error creating announcement")
                        })?;

                        let creator = sqlx::query!(
                            "SELECT first_name, last_name FROM users WHERE id = $1",
                            user.id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .ok();

                        let announcement_item = AnnouncementItem {
                            id: announcement.id,
                            title: announcement.title,
                            description: announcement.description,
                            full_text: announcement.full_text.unwrap_or_default(),
                            is_important: announcement.is_important,
                            is_active: announcement.is_active,
                            created_by: announcement.created_by,
                            created_by_name: creator.and_then(|c| {
                                let first = c.first_name.unwrap_or_default();
                                let last = c.last_name.unwrap_or_default();
                                let name = if first.is_empty() && last.is_empty() {
                                    None
                                } else {
                                    Some(format!("{} {}", first, last).trim().to_string())
                                };
                                name
                            }),
                            created_at: announcement.created_at,
                            updated_at: None,
                            updated_by: None,
                            updated_by_name: None,
                            activated_at: None,
                            activated_by: None,
                            activated_by_name: None,
                        };

                        Ok(HttpResponse::Ok().json(announcement_item))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}

pub async fn get_active_announcement(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                if decode_token(&token).is_err() {
                    return Err(error::ErrorUnauthorized("Invalid token"));
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid token"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid token"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing token"));
    }

    let announcement = sqlx::query!(
        r#"
        SELECT 
            a.id, a.title, a.description, a.full_text, a.is_important, a.is_active,
            a.created_by, CONCAT(u.first_name, ' ', u.last_name) as created_by_name,
            a.created_at, a.updated_at, a.updated_by,
            CONCAT(u2.first_name, ' ', u2.last_name) as updated_by_name,
            a.activated_at, a.activated_by,
            CONCAT(u3.first_name, ' ', u3.last_name) as activated_by_name
        FROM announcements a
        LEFT JOIN users u ON a.created_by = u.id
        LEFT JOIN users u2 ON a.updated_by = u2.id
        LEFT JOIN users u3 ON a.activated_by = u3.id
        WHERE a.is_active = true
        LIMIT 1
        "#
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch active announcement: {}", e);
        error::ErrorInternalServerError("Error fetching announcement")
    })?;

    let announcement_item = announcement.map(|row| {
        AnnouncementItem {
            id: row.id,
            title: row.title,
            description: row.description,
            full_text: row.full_text.unwrap_or_default(),
            is_important: row.is_important,
            is_active: row.is_active,
            created_by: row.created_by,
            created_by_name: row.created_by_name,
            created_at: row.created_at,
            updated_at: row.updated_at,
            updated_by: row.updated_by,
            updated_by_name: row.updated_by_name,
            activated_at: row.activated_at,
            activated_by: row.activated_by,
            activated_by_name: row.activated_by_name,
        }
    });

    Ok(HttpResponse::Ok().json(announcement_item))
}

pub async fn get_all_announcements(
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

                        let announcements = sqlx::query!(
                            r#"
                            SELECT 
                                a.id, a.title, a.description, a.full_text, a.is_important, a.is_active,
                                a.created_by, CONCAT(u.first_name, ' ', u.last_name) as created_by_name,
                                a.created_at, a.updated_at, a.updated_by,
                                CONCAT(u2.first_name, ' ', u2.last_name) as updated_by_name,
                                a.activated_at, a.activated_by,
                                CONCAT(u3.first_name, ' ', u3.last_name) as activated_by_name
                            FROM announcements a
                            LEFT JOIN users u ON a.created_by = u.id
                            LEFT JOIN users u2 ON a.updated_by = u2.id
                            LEFT JOIN users u3 ON a.activated_by = u3.id
                            ORDER BY a.created_at DESC
                            "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch all announcements: {}", e);
                            error::ErrorInternalServerError("Error fetching announcements")
                        })?;

                        let announcement_items: Vec<AnnouncementItem> = announcements.into_iter().map(|row| {
                            AnnouncementItem {
                                id: row.id,
                                title: row.title,
                                description: row.description,
                                full_text: row.full_text.unwrap_or_default(),
                                is_important: row.is_important,
                                is_active: row.is_active,
                                created_by: row.created_by,
                                created_by_name: row.created_by_name,
                                created_at: row.created_at,
                                updated_at: row.updated_at,
                                updated_by: row.updated_by,
                                updated_by_name: row.updated_by_name,
                                activated_at: row.activated_at,
                                activated_by: row.activated_by,
                                activated_by_name: row.activated_by_name,
                            }
                        }).collect();

                        Ok(HttpResponse::Ok().json(announcement_items))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}

pub async fn update_announcement(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
    form: web::Json<UpdateAnnouncementRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type, u.id
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

                        let announcement_id = path.into_inner();

                        println!("Updating announcement {} with is_active = {:?}", announcement_id, form.is_active);

                        sqlx::query!(
                            r#"
                            UPDATE announcements SET
                                title = COALESCE($1, title),
                                description = COALESCE($2, description),
                                full_text = COALESCE($3, full_text),
                                is_important = COALESCE($4, is_important),
                                is_active = COALESCE($5, is_active),
                                updated_at = NOW(),
                                updated_by = $6
                            WHERE id = $7
                            "#,
                            form.title,
                            form.description,
                            form.full_text,
                            form.is_important,
                            form.is_active,
                            user.id,
                            announcement_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Error updating announcement: {}", e);
                            error::ErrorInternalServerError("Error updating announcement")
                        })?;

                        Ok(HttpResponse::Ok().json("Announcement updated successfully"))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}

pub async fn set_active_announcement(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<SetActiveAnnouncementRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type, u.id
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

                        sqlx::query!(
                            "UPDATE announcements SET is_active = false, updated_at = NOW(), updated_by = $1",
                            user.id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error deactivating announcements"))?;

                        sqlx::query!(
                            "UPDATE announcements SET is_active = true, activated_at = NOW(), activated_by = $1 WHERE id = $2",
                            user.id,
                            form.announcement_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error activating announcement"))?;

                        Ok(HttpResponse::Ok().json("Announcement activated successfully"))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}

pub async fn delete_announcement(
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

                        let announcement_id = path.into_inner();

                        sqlx::query!(
                            "DELETE FROM announcements WHERE id = $1",
                            announcement_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error deleting announcement"))?;

                        Ok(HttpResponse::Ok().json(json!({"message": "Announcement deleted successfully"})))
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
        Err(error::ErrorUnauthorized("Missing token"))
    }
}