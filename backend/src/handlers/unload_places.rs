use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};
use chrono::{NaiveTime, Local, NaiveDateTime, Datelike, Timelike};
use std::fs;
use std::path::Path;
use actix_multipart::Multipart;
use futures_util::stream::StreamExt as _;
use uuid::Uuid;

use crate::auth::decode_token;
use crate::models::unload_places::{
    UnloadPlace, UnloadPlaceTimeSlot, UnloadPlacePhoto, UnloadPlaceWithDetails,
    CreateUnloadPlaceRequest, UpdateUnloadPlaceRequest,
    CreateTimeSlotRequest, UpdateTimeSlotRequest
};

const MAX_FILE_SIZE: usize = 10 * 1024 * 1024; // 10 MB
const UPLOAD_DIR: &str = "./uploads/unload_places";

/// Получение всех мест разгрузки с деталями
pub async fn get_unload_places(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Getting all unload places");

    let rows = sqlx::query!(
        r#"
        SELECT 
            id, 
            name, 
            description, 
            map_link, 
            status, 
            status_comment, 
            is_active, 
            created_at, 
            updated_at
        FROM unload_places 
        ORDER BY name
        "#
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch unload places: {}", e);
        error::ErrorInternalServerError("Error fetching unload places")
    })?;

    // Получаем текущий день недели и время для определения статуса
    let now = Local::now();
    let current_day = now.weekday().num_days_from_monday() as i32; // 0=Пн, 6=Вс
    let current_time = now.time();
    
    let mut places_with_details = Vec::new();
    
    for row in rows {
        // Получаем временные слоты для места (все, независимо от is_active)
        let slots_rows = sqlx::query!(
            r#"
            SELECT 
                id, 
                unload_place_id, 
                day_of_week, 
                open_time, 
                close_time, 
                is_next_day, 
                is_active, 
                created_at, 
                updated_at
            FROM unload_place_time_slots 
            WHERE unload_place_id = $1
            ORDER BY day_of_week, open_time
            "#,
            row.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());
        
        let time_slots: Vec<UnloadPlaceTimeSlot> = slots_rows.into_iter().map(|s| {
            UnloadPlaceTimeSlot {
                id: s.id,
                unload_place_id: s.unload_place_id,
                day_of_week: s.day_of_week,
                open_time: s.open_time,
                close_time: s.close_time,
                is_next_day: s.is_next_day.unwrap_or(false),
                is_active: s.is_active.unwrap_or(true),
                created_at: s.created_at,
                updated_at: s.updated_at,
            }
        }).collect();
        
        // Получаем фотографии
        let photos_rows = sqlx::query!(
            r#"
            SELECT 
                id, 
                unload_place_id, 
                photo_url, 
                file_name, 
                file_size, 
                mime_type, 
                is_main, 
                uploaded_at, 
                uploaded_by
            FROM unload_place_photos 
            WHERE unload_place_id = $1
            ORDER BY is_main DESC, uploaded_at DESC
            "#,
            row.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());
        
        let photos: Vec<UnloadPlacePhoto> = photos_rows.into_iter().map(|p| {
            UnloadPlacePhoto {
                id: p.id,
                unload_place_id: p.unload_place_id,
                photo_url: p.photo_url,
                file_name: p.file_name,
                file_size: p.file_size,
                mime_type: p.mime_type,
                is_main: p.is_main.unwrap_or(false),
                uploaded_at: p.uploaded_at,
                uploaded_by: p.uploaded_by,
            }
        }).collect();
        
        // Определяем текущий статус (открыто/закрыто)
        let mut current_status = "closed".to_string();
        
        let place_status = row.status.as_deref().unwrap_or("active");
        
        // Проверяем только если место активно
        if place_status == "active" {
            // Проверяем есть ли круглосуточное окно (00:00-23:59) для текущего дня
            let has_round_the_clock = time_slots.iter().any(|s| 
                s.day_of_week == current_day && 
                s.is_active && 
                s.open_time == NaiveTime::from_hms_opt(0, 0, 0).unwrap() &&
                s.close_time == NaiveTime::from_hms_opt(23, 59, 0).unwrap() &&
                !s.is_next_day
            );
            
            if has_round_the_clock {
                current_status = "open".to_string();
            } else {
                // Иначе проверяем по активным временным слотам
                for slot in &time_slots {
                    if slot.day_of_week == current_day && slot.is_active {
                        let open_time = slot.open_time;
                        let close_time = slot.close_time;
                        
                        let is_open = if slot.is_next_day {
                            if current_time >= open_time {
                                true
                            } else {
                                false
                            }
                        } else {
                            current_time >= open_time && current_time <= close_time
                        };
                        
                        if is_open {
                            current_status = "open".to_string();
                            break;
                        }
                    }
                }
            }
        }
        
        places_with_details.push(UnloadPlaceWithDetails {
            id: row.id,
            name: row.name,
            description: row.description,
            map_link: row.map_link,
            status: place_status.to_string(),
            status_comment: row.status_comment,
            is_active: row.is_active,
            current_status,
            time_slots,
            photos,
            created_at: row.created_at,
            updated_at: row.updated_at,
        });
    }

    Ok(HttpResponse::Ok().json(places_with_details))
}



/// Получение места разгрузки по ID с деталями
pub async fn get_unload_place_by_id(
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

    let place_id = path.into_inner();

    log::info!("Getting unload place by ID: {}", place_id);

    let row = sqlx::query!(
        r#"
        SELECT 
            id, 
            name, 
            description, 
            map_link, 
            status, 
            status_comment, 
            is_active, 
            created_at, 
            updated_at
        FROM unload_places 
        WHERE id = $1
        "#,
        place_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch unload place: {}", e);
        error::ErrorInternalServerError("Error fetching unload place")
    })?;

    match row {
        Some(row) => {
            // Получаем временные слоты
            let slots_rows = sqlx::query!(
                r#"
                SELECT 
                    id, 
                    unload_place_id, 
                    day_of_week, 
                    open_time, 
                    close_time, 
                    is_next_day, 
                    is_active, 
                    created_at, 
                    updated_at
                FROM unload_place_time_slots 
                WHERE unload_place_id = $1
                ORDER BY day_of_week, open_time
                "#,
                place_id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());
            
            let time_slots: Vec<UnloadPlaceTimeSlot> = slots_rows.into_iter().map(|s| {
                UnloadPlaceTimeSlot {
                    id: s.id,
                    unload_place_id: s.unload_place_id,
                    day_of_week: s.day_of_week,
                    open_time: s.open_time,
                    close_time: s.close_time,
                    is_next_day: s.is_next_day.unwrap_or(false),
                    is_active: s.is_active.unwrap_or(true),
                    created_at: s.created_at,
                    updated_at: s.updated_at,
                }
            }).collect();
            
            // Получаем фотографии
            let photos_rows = sqlx::query!(
                r#"
                SELECT 
                    id, 
                    unload_place_id, 
                    photo_url, 
                    file_name, 
                    file_size, 
                    mime_type, 
                    is_main, 
                    uploaded_at, 
                    uploaded_by
                FROM unload_place_photos 
                WHERE unload_place_id = $1
                ORDER BY is_main DESC, uploaded_at DESC
                "#,
                place_id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());
            
            let photos: Vec<UnloadPlacePhoto> = photos_rows.into_iter().map(|p| {
                UnloadPlacePhoto {
                    id: p.id,
                    unload_place_id: p.unload_place_id,
                    photo_url: p.photo_url,
                    file_name: p.file_name,
                    file_size: p.file_size,
                    mime_type: p.mime_type,
                    is_main: p.is_main.unwrap_or(false),
                    uploaded_at: p.uploaded_at,
                    uploaded_by: p.uploaded_by,
                }
            }).collect();
            
            // Определяем текущий статус
            let now = Local::now();
            let current_day = now.weekday().num_days_from_monday() as i32;
            let current_time = now.time();
            
            let mut current_status = "closed".to_string();
            
            let place_status = row.status.as_deref().unwrap_or("active");
            
            if place_status == "active" {
                // Проверяем есть ли круглосуточное окно (00:00-23:59) для текущего дня
                let has_round_the_clock = time_slots.iter().any(|s| 
                    s.day_of_week == current_day && 
                    s.is_active && 
                    s.open_time == NaiveTime::from_hms_opt(0, 0, 0).unwrap() &&
                    s.close_time == NaiveTime::from_hms_opt(23, 59, 0).unwrap() &&
                    !s.is_next_day
                );
                
                if has_round_the_clock {
                    current_status = "open".to_string();
                } else {
                    for slot in &time_slots {
                        if slot.day_of_week == current_day && slot.is_active {
                            let open_time = slot.open_time;
                            let close_time = slot.close_time;
                            
                            let is_open = if slot.is_next_day {
                                if current_time >= open_time {
                                    true
                                } else {
                                    false
                                }
                            } else {
                                current_time >= open_time && current_time <= close_time
                            };
                            
                            if is_open {
                                current_status = "open".to_string();
                                break;
                            }
                        }
                    }
                }
            }
            
            let place_with_details = UnloadPlaceWithDetails {
                id: row.id,
                name: row.name,
                description: row.description,
                map_link: row.map_link,
                status: place_status.to_string(),
                status_comment: row.status_comment,
                is_active: row.is_active,
                current_status,
                time_slots,
                photos,
                created_at: row.created_at,
                updated_at: row.updated_at,
            };
            
            Ok(HttpResponse::Ok().json(place_with_details))
        },
        None => Err(error::ErrorNotFound("Место разгрузки не найдено")),
    }
}
/// Создание нового места разгрузки
pub async fn create_unload_place(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    place_data: web::Json<CreateUnloadPlaceRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Creating new unload place: {}", place_data.name);

    let status = place_data.status.as_deref().unwrap_or("active");

    let result = sqlx::query!(
        r#"
        INSERT INTO unload_places (name, description, map_link, status, status_comment, is_active, updated_at)
        VALUES ($1, $2, $3, $4, $5, true, NOW())
        RETURNING id
        "#,
        place_data.name,
        place_data.description,
        place_data.map_link,
        status,
        place_data.status_comment
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to create unload place: {}", e);
        error::ErrorInternalServerError("Error creating unload place")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "id": result.id,
        "message": "Место разгрузки успешно создано"
    })))
}

/// Обновление места разгрузки
pub async fn update_unload_place(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    place_data: web::Json<UpdateUnloadPlaceRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let place_id = path.into_inner();

    log::info!("Updating unload place ID: {}", place_id);

    // Получаем текущие данные
    let current = sqlx::query!(
        r#"
        SELECT name, description, map_link, status, status_comment
        FROM unload_places
        WHERE id = $1
        "#,
        place_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch unload place: {}", e);
        error::ErrorInternalServerError("Error fetching unload place")
    })?;

    let current = match current {
        Some(c) => c,
        None => return Err(error::ErrorNotFound("Место разгрузки не найдено")),
    };

    // Строим динамический запрос
    let mut updates = Vec::new();
    let mut params: Vec<String> = Vec::new();
    let mut param_counter = 1;

    if let Some(name) = &place_data.name {
        updates.push(format!("name = ${}", param_counter));
        params.push(name.clone());
        param_counter += 1;
    }

    if let Some(description) = &place_data.description {
        updates.push(format!("description = ${}", param_counter));
        params.push(description.clone());
        param_counter += 1;
    }

    if let Some(map_link) = &place_data.map_link {
        updates.push(format!("map_link = ${}", param_counter));
        params.push(map_link.clone());
        param_counter += 1;
    }

    if let Some(status) = &place_data.status {
        updates.push(format!("status = ${}", param_counter));
        params.push(status.clone());
        param_counter += 1;
    }

    if let Some(status_comment) = &place_data.status_comment {
        updates.push(format!("status_comment = ${}", param_counter));
        params.push(status_comment.clone());
        param_counter += 1;
    }

    updates.push(format!("updated_at = NOW()"));

    if updates.is_empty() {
        return Ok(HttpResponse::Ok().json("Нет данных для обновления"));
    }

    let query = format!(
        "UPDATE unload_places SET {} WHERE id = ${}",
        updates.join(", "),
        param_counter
    );

    let mut query_builder = sqlx::query(&query);
    
    for param in &params {
        query_builder = query_builder.bind(param);
    }
    
    query_builder = query_builder.bind(place_id);

    let result = query_builder
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to update unload place: {}", e);
            error::ErrorInternalServerError("Error updating unload place")
        })?;

    if result.rows_affected() > 0 {
        Ok(HttpResponse::Ok().json("Место разгрузки успешно обновлено"))
    } else {
        Err(error::ErrorNotFound("Место разгрузки не найдено"))
    }
}

/// Удаление места разгрузки
pub async fn delete_unload_place(
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

    let place_id = path.into_inner();

    log::info!("Deleting unload place ID: {}", place_id);

    // Получаем фотографии для удаления файлов
    let photos = sqlx::query!(
        r#"
        SELECT photo_url
        FROM unload_place_photos
        WHERE unload_place_id = $1
        "#,
        place_id
    )
    .fetch_all(pool.get_ref())
    .await
    .unwrap_or_else(|_| Vec::new());

    // Проверяем, используется ли место разгрузки в организациях
    let org_count = sqlx::query!(
        r#"
        SELECT COUNT(*) as count
        FROM organization_unload_places
        WHERE unload_place_id = $1
        "#,
        place_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check organization unload places: {}", e);
        error::ErrorInternalServerError("Error checking organization dependencies")
    })?;

    // Проверяем, используется ли место разгрузки в компаниях
    let company_count = sqlx::query!(
        r#"
        SELECT COUNT(*) as count
        FROM companies_unload_places
        WHERE unload_place_id = $1
        "#,
        place_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check company unload places: {}", e);
        error::ErrorInternalServerError("Error checking company dependencies")
    })?;

    // Если место разгрузки привязано к организациям или компаниям, не удаляем
    if org_count.count.unwrap_or(0) > 0 || company_count.count.unwrap_or(0) > 0 {
        let mut error_message = String::from("Невозможно удалить место разгрузки, так как оно привязано к: ");
        let mut parts = Vec::new();
        
        if org_count.count.unwrap_or(0) > 0 {
            parts.push(format!("организациям ({})", org_count.count.unwrap_or(0)));
        }
        if company_count.count.unwrap_or(0) > 0 {
            parts.push(format!("компаниям ({})", company_count.count.unwrap_or(0)));
        }
        
        error_message.push_str(&parts.join(" и "));
        return Err(error::ErrorBadRequest(error_message));
    }

    // Удаляем файлы фотографий
    for photo in photos {
        let file_path = format!("./{}", photo.photo_url);
        if Path::new(&file_path).exists() {
            let _ = fs::remove_file(file_path);
        }
    }

    // Удаляем место разгрузки
    let result = sqlx::query!(
        r#"
        DELETE FROM unload_places
        WHERE id = $1
        "#,
        place_id
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to delete unload place: {}", e);
        error::ErrorInternalServerError("Error deleting unload place")
    })?;

    if result.rows_affected() > 0 {
        Ok(HttpResponse::Ok().json("Место разгрузки успешно удалено"))
    } else {
        Err(error::ErrorNotFound("Место разгрузки не найдено"))
    }
}

/// Получение временных слотов места разгрузки
pub async fn get_unload_place_time_slots(
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

    let place_id = path.into_inner();

    log::info!("Getting time slots for unload place ID: {}", place_id);

    let rows = sqlx::query!(
        r#"
        SELECT 
            id, 
            unload_place_id, 
            day_of_week, 
            open_time, 
            close_time, 
            is_next_day, 
            is_active, 
            created_at, 
            updated_at
        FROM unload_place_time_slots 
        WHERE unload_place_id = $1
        ORDER BY day_of_week, open_time
        "#,
        place_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch time slots: {}", e);
        error::ErrorInternalServerError("Error fetching time slots")
    })?;

    let slots: Vec<UnloadPlaceTimeSlot> = rows.into_iter().map(|row| {
        UnloadPlaceTimeSlot {
            id: row.id,
            unload_place_id: row.unload_place_id,
            day_of_week: row.day_of_week,
            open_time: row.open_time,
            close_time: row.close_time,
            is_next_day: row.is_next_day.unwrap_or(false),
            is_active: row.is_active.unwrap_or(true),
            created_at: row.created_at,
            updated_at: row.updated_at,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(slots))
}

/// Добавление временного слота для места разгрузки
pub async fn add_unload_place_time_slot(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    slot_data: web::Json<CreateTimeSlotRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let place_id = path.into_inner();

    log::info!("Adding time slot for unload place ID: {}", place_id);

    // Проверяем существование места
    let place_exists = sqlx::query!(
        r#"
        SELECT id
        FROM unload_places
        WHERE id = $1
        "#,
        place_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check unload place: {}", e);
        error::ErrorInternalServerError("Error checking unload place")
    })?;

    if place_exists.is_none() {
        return Err(error::ErrorNotFound("Место разгрузки не найдено"));
    }

    // Парсим время
    let open_time = match NaiveTime::parse_from_str(&slot_data.open_time, "%H:%M") {
        Ok(t) => t,
        Err(_) => return Err(error::ErrorBadRequest("Неверный формат времени открытия. Используйте ЧЧ:ММ")),
    };
    
    let close_time = match NaiveTime::parse_from_str(&slot_data.close_time, "%H:%M") {
        Ok(t) => t,
        Err(_) => return Err(error::ErrorBadRequest("Неверный формат времени закрытия. Используйте ЧЧ:ММ")),
    };
    
    // Проверяем день недели
    if slot_data.day_of_week < 0 || slot_data.day_of_week > 6 {
        return Err(error::ErrorBadRequest("День недели должен быть от 0 (Пн) до 6 (Вс)"));
    }
    
    let is_next_day = slot_data.is_next_day.unwrap_or(false);
    let is_active = slot_data.is_active.unwrap_or(true);

    let result = sqlx::query!(
        r#"
        INSERT INTO unload_place_time_slots 
            (unload_place_id, day_of_week, open_time, close_time, is_next_day, is_active, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        RETURNING id
        "#,
        place_id,
        slot_data.day_of_week,
        open_time,
        close_time,
        is_next_day,
        is_active
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to add time slot: {}", e);
        error::ErrorInternalServerError("Error adding time slot")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "id": result.id,
        "message": "Временной слот успешно добавлен"
    })))
}

/// Обновление временного слота
pub async fn update_unload_place_time_slot(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (place_id, slot_id)
    slot_data: web::Json<UpdateTimeSlotRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let (place_id, slot_id) = path.into_inner();

    log::info!("Updating time slot ID: {} for place ID: {}", slot_id, place_id);

    // Получаем текущие данные слота
    let current = sqlx::query!(
        r#"
        SELECT day_of_week, open_time, close_time, is_next_day, is_active
        FROM unload_place_time_slots
        WHERE id = $1 AND unload_place_id = $2
        "#,
        slot_id,
        place_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch time slot: {}", e);
        error::ErrorInternalServerError("Error fetching time slot")
    })?;

    let current = match current {
        Some(c) => c,
        None => return Err(error::ErrorNotFound("Временной слот не найден")),
    };

    // Определяем новые значения
    let day_of_week = slot_data.day_of_week.unwrap_or(current.day_of_week as i32);
    
    let open_time = if let Some(ref ot) = slot_data.open_time {
        match NaiveTime::parse_from_str(ot, "%H:%M") {
            Ok(t) => t,
            Err(_) => return Err(error::ErrorBadRequest("Неверный формат времени открытия")),
        }
    } else {
        current.open_time
    };
    
    let close_time = if let Some(ref ct) = slot_data.close_time {
        match NaiveTime::parse_from_str(ct, "%H:%M") {
            Ok(t) => t,
            Err(_) => return Err(error::ErrorBadRequest("Неверный формат времени закрытия")),
        }
    } else {
        current.close_time
    };
    
    let is_next_day = slot_data.is_next_day.unwrap_or(current.is_next_day.unwrap_or(false));
    let is_active = slot_data.is_active.unwrap_or(current.is_active.unwrap_or(true));

    // Проверяем день недели
    if day_of_week < 0 || day_of_week > 6 {
        return Err(error::ErrorBadRequest("День недели должен быть от 0 (Пн) до 6 (Вс)"));
    }

    let result = sqlx::query!(
        r#"
        UPDATE unload_place_time_slots 
        SET day_of_week = $1, open_time = $2, close_time = $3, is_next_day = $4, is_active = $5, updated_at = NOW()
        WHERE id = $6 AND unload_place_id = $7
        "#,
        day_of_week as i16,
        open_time,
        close_time,
        is_next_day,
        is_active,
        slot_id,
        place_id
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to update time slot: {}", e);
        error::ErrorInternalServerError("Error updating time slot")
    })?;

    if result.rows_affected() > 0 {
        Ok(HttpResponse::Ok().json("Временной слот успешно обновлен"))
    } else {
        Err(error::ErrorNotFound("Временной слот не найден"))
    }
}

/// Удаление временного слота
pub async fn delete_unload_place_time_slot(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (place_id, slot_id)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let (place_id, slot_id) = path.into_inner();

    log::info!("Deleting time slot ID: {} for place ID: {}", slot_id, place_id);

    let result = sqlx::query!(
        r#"
        DELETE FROM unload_place_time_slots
        WHERE id = $1 AND unload_place_id = $2
        "#,
        slot_id,
        place_id
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to delete time slot: {}", e);
        error::ErrorInternalServerError("Error deleting time slot")
    })?;

    if result.rows_affected() > 0 {
        Ok(HttpResponse::Ok().json("Временной слот успешно удален"))
    } else {
        Err(error::ErrorNotFound("Временной слот не найден"))
    }
}

/// Загрузка фотографии для места разгрузки
pub async fn upload_unload_place_photo(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    mut payload: Multipart,
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
    
    let place_id = path.into_inner();

    log::info!("Uploading photo for unload place ID: {}", place_id);

    // Получаем ID текущего пользователя
    let user_row = sqlx::query!(
        r#"
        SELECT id
        FROM users
        WHERE username = $1
        "#,
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

    // Проверяем существование места
    let place_exists = sqlx::query!(
        r#"
        SELECT id
        FROM unload_places
        WHERE id = $1
        "#,
        place_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check unload place: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if place_exists.is_none() {
        return Err(error::ErrorNotFound("Место разгрузки не найдено"));
    }

    // Создаем директорию для загрузок, если её нет
    fs::create_dir_all(UPLOAD_DIR).map_err(|e| {
        log::error!("Failed to create upload directory: {}", e);
        error::ErrorInternalServerError("Failed to create upload directory")
    })?;

    let mut uploaded_files = Vec::new();

    // Обрабатываем загруженные файлы
    while let Some(item) = payload.next().await {
        let mut field = item.map_err(|e| {
            log::error!("Error reading multipart: {}", e);
            error::ErrorBadRequest("Error reading multipart")
        })?;

        let content_disposition = field.content_disposition();
        let filename = content_disposition.get_filename().unwrap_or("unknown").to_string();
        
        // Генерируем уникальное имя файла
        let file_extension = Path::new(&filename)
            .extension()
            .and_then(|e| e.to_str())
            .unwrap_or("jpg");
        
        let unique_filename = format!("{}_{}.{}", Uuid::new_v4(), place_id, file_extension);
        let filepath = Path::new(UPLOAD_DIR).join(&unique_filename);
        let file_url = format!("/uploads/unload_places/{}", unique_filename);

        let mut file_size = 0;
        let mut file_data = Vec::new();

        // Читаем данные файла
        while let Some(chunk) = field.next().await {
            let data = chunk.map_err(|e| {
                log::error!("Error reading chunk: {}", e);
                error::ErrorBadRequest("Error reading chunk")
            })?;
            file_size += data.len();
            if file_size > MAX_FILE_SIZE {
                return Err(error::ErrorBadRequest("File too large. Max 10MB"));
            }
            file_data.extend_from_slice(&data);
        }

        // Сохраняем файл
        fs::write(&filepath, &file_data).map_err(|e| {
            log::error!("Failed to write file: {}", e);
            error::ErrorInternalServerError("Failed to write file")
        })?;

        // Определяем MIME тип
        let mime_type = mime_guess::from_path(&filename)
            .first()
            .map(|m| m.to_string())
            .unwrap_or_else(|| "application/octet-stream".to_string());

        uploaded_files.push((file_url, filename, file_size as i32, mime_type));
    }

    // Сохраняем информацию о фотографиях в БД
    let mut inserted_ids = Vec::new();
    
    for (file_url, filename, file_size, mime_type) in uploaded_files {
        // Проверяем, есть ли уже фотографии у этого места
        let photo_count = sqlx::query!(
            r#"
            SELECT COUNT(*) as count
            FROM unload_place_photos
            WHERE unload_place_id = $1
            "#,
            place_id
        )
        .fetch_one(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to count photos: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

        let is_main = photo_count.count.unwrap_or(0) == 0; // Первая фотография - главная

        let result = sqlx::query!(
            r#"
            INSERT INTO unload_place_photos (unload_place_id, photo_url, file_name, file_size, mime_type, is_main, uploaded_by)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            RETURNING id
            "#,
            place_id,
            file_url,
            filename,
            file_size,
            mime_type,
            is_main,
            current_user_id
        )
        .fetch_one(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to insert photo record: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

        inserted_ids.push(result.id);
    }

    Ok(HttpResponse::Ok().json(json!({
        "message": "Фотографии успешно загружены",
        "photo_ids": inserted_ids
    })))
}

/// Удаление фотографии
pub async fn delete_unload_place_photo(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (place_id, photo_id)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let (place_id, photo_id) = path.into_inner();

    log::info!("Deleting photo ID: {} for place ID: {}", photo_id, place_id);

    // Получаем информацию о фотографии для удаления файла
    let photo = sqlx::query!(
        r#"
        SELECT photo_url, is_main
        FROM unload_place_photos
        WHERE id = $1 AND unload_place_id = $2
        "#,
        photo_id,
        place_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch photo: {}", e);
        error::ErrorInternalServerError("Error fetching photo")
    })?;

    let photo = match photo {
        Some(p) => p,
        None => return Err(error::ErrorNotFound("Фотография не найдена")),
    };

    // Удаляем файл
    let file_path = format!("./{}", photo.photo_url);
    if Path::new(&file_path).exists() {
        let _ = fs::remove_file(file_path);
    }

    // Удаляем запись из БД
    let result = sqlx::query!(
        r#"
        DELETE FROM unload_place_photos
        WHERE id = $1 AND unload_place_id = $2
        "#,
        photo_id,
        place_id
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to delete photo: {}", e);
        error::ErrorInternalServerError("Error deleting photo")
    })?;

    if result.rows_affected() == 0 {
        return Err(error::ErrorNotFound("Фотография не найдена"));
    }

    // Если удалили главную фотографию, делаем следующую главной
    if photo.is_main.unwrap_or(false) {
        // Находим ID следующей фотографии для установки главной
        let next_photo = sqlx::query!(
            r#"
            SELECT id
            FROM unload_place_photos
            WHERE unload_place_id = $1 AND id != $2
            ORDER BY uploaded_at
            LIMIT 1
            "#,
            place_id,
            photo_id
        )
        .fetch_optional(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to find next photo: {}", e);
            error::ErrorInternalServerError("Error finding next photo")
        })?;

        if let Some(next) = next_photo {
            sqlx::query!(
                r#"
                UPDATE unload_place_photos
                SET is_main = true
                WHERE id = $1
                "#,
                next.id
            )
            .execute(pool.get_ref())
            .await
            .map_err(|e| {
                log::error!("Failed to set next main photo: {}", e);
                error::ErrorInternalServerError("Error setting next main photo")
            })?;
        }
    }

    Ok(HttpResponse::Ok().json("Фотография успешно удалена"))
}

/// Установка главной фотографии
pub async fn set_main_unload_place_photo(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (place_id, photo_id)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let (place_id, photo_id) = path.into_inner();

    log::info!("Setting main photo ID: {} for place ID: {}", photo_id, place_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Сбрасываем флаг is_main для всех фотографий этого места
    sqlx::query!(
        r#"
        UPDATE unload_place_photos
        SET is_main = false
        WHERE unload_place_id = $1
        "#,
        place_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to reset main photo: {}", e);
        error::ErrorInternalServerError("Error resetting main photo")
    })?;

    // Устанавливаем новую главную фотографию
    let result = sqlx::query!(
        r#"
        UPDATE unload_place_photos
        SET is_main = true
        WHERE id = $1 AND unload_place_id = $2
        RETURNING id
        "#,
        photo_id,
        place_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to set main photo: {}", e);
        error::ErrorInternalServerError("Error setting main photo")
    })?;

    if result.is_none() {
        return Err(error::ErrorNotFound("Фотография не найдена"));
    }

    // Фиксируем транзакцию
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json("Главная фотография успешно установлена"))
}