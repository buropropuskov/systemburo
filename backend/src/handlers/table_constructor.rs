use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;
use chrono::{NaiveTime, Local, Datelike, Timelike};
use std::fs;
use std::path::Path;
use actix_multipart::Multipart;
use futures_util::stream::StreamExt as _;
use uuid::Uuid;

use crate::auth::decode_token;
use crate::models::table_constructor::{
    SystemTable, SystemTableTimeSlot, SystemTablePhoto, TableField,
    SystemTableWithDetails, SystemTableWithFields,
    CreateSystemTableRequest, UpdateSystemTableRequest,
    CreateTimeSlotRequest, UpdateTimeSlotRequest
};

const MAX_FILE_SIZE: usize = 10 * 1024 * 1024; // 10 MB
const UPLOAD_DIR: &str = "./uploads/system_tables";

/// Получение всех системных таблиц
pub async fn get_system_tables(
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

    log::info!("Getting all system tables");

    let rows = sqlx::query!(
        r#"
        SELECT 
            id, 
            name, 
            display_name, 
            table_type, 
            show_fact_table, 
            fact_table_hint, 
            instruction,
            map_link,
            status,
            status_comment,
            location_description,
            is_active, 
            created_at, 
            updated_at
        FROM system_tables 
        WHERE is_active = true
        ORDER BY display_name
        "#
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch system tables: {}", e);
        error::ErrorInternalServerError("Error fetching system tables")
    })?;

    // Получаем текущий день недели и время для определения статуса
    let now = Local::now();
    let current_day = now.weekday().num_days_from_monday() as i32; // 0=Пн, 6=Вс
    let current_time = now.time();
    
    let mut tables_with_details = Vec::new();
    
    for row in rows {
        // Получаем временные слоты для таблицы
        let slots_rows = sqlx::query!(
            r#"
            SELECT 
                id, 
                table_id, 
                day_of_week, 
                open_time, 
                close_time, 
                is_next_day, 
                is_active, 
                created_at, 
                updated_at
            FROM system_table_time_slots 
            WHERE table_id = $1
            ORDER BY day_of_week, open_time
            "#,
            row.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());
        
        let time_slots: Vec<SystemTableTimeSlot> = slots_rows.into_iter().map(|s| {
            SystemTableTimeSlot {
                id: s.id,
                table_id: s.table_id,
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
                table_id, 
                photo_url, 
                file_name, 
                file_size, 
                mime_type, 
                is_main, 
                uploaded_at, 
                uploaded_by
            FROM system_table_photos 
            WHERE table_id = $1
            ORDER BY is_main DESC, uploaded_at DESC
            "#,
            row.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());
        
        let photos: Vec<SystemTablePhoto> = photos_rows.into_iter().map(|p| {
            SystemTablePhoto {
                id: p.id,
                table_id: p.table_id,
                photo_url: p.photo_url,
                file_name: p.file_name,
                file_size: p.file_size,
                mime_type: p.mime_type,
                is_main: p.is_main.unwrap_or(false),
                uploaded_at: p.uploaded_at,
                uploaded_by: p.uploaded_by,
            }
        }).collect();
        
        // Получаем поля таблицы
        // Получаем поля таблицы
let fields_rows = sqlx::query!(
    r#"
    SELECT 
        id, 
        table_id, 
        field_name, 
        field_type, 
        display_order, 
        is_visible, 
        created_at
    FROM table_fields 
    WHERE table_id = $1
    ORDER BY display_order
    "#,
    row.id
)
.fetch_all(pool.get_ref())
.await
.unwrap_or_else(|_| Vec::new());

let fields: Vec<TableField> = fields_rows.into_iter().map(|row| {
    TableField {
        id: row.id,
        table_id: row.table_id.expect("REASON"),
        field_name: row.field_name,
        field_type: row.field_type,
        display_order: row.display_order.unwrap_or(0),
        is_visible: Some(row.is_visible.unwrap_or(true)),
        created_at: row.created_at,
    }
}).collect();
        
        // Определяем текущий статус (открыто/закрыто)
        let mut current_status = "closed".to_string();
        
        let table_status = row.status.as_deref().unwrap_or("active");
        
        if table_status == "active" {
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
        
        tables_with_details.push(SystemTableWithDetails {
            table: SystemTable {
                id: row.id,
                name: row.name,
                display_name: row.display_name,
                table_type: row.table_type,
                show_fact_table: row.show_fact_table,
                fact_table_hint: row.fact_table_hint,
                instruction: row.instruction,
                map_link: row.map_link,
                status: row.status,
                status_comment: row.status_comment,
                location_description: row.location_description,
                is_active: row.is_active,
                created_at: row.created_at,
                updated_at: row.updated_at,
            },
            fields,
            time_slots,
            photos,
            current_status,
        });
    }

    Ok(HttpResponse::Ok().json(tables_with_details))
}

/// Получение таблицы по ID с деталями
pub async fn get_system_table_by_id(
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

    let table_id = path.into_inner();

    log::info!("Getting system table by ID: {}", table_id);

    let row = sqlx::query!(
        r#"
        SELECT 
            id, 
            name, 
            display_name, 
            table_type, 
            show_fact_table, 
            fact_table_hint, 
            instruction,
            map_link,
            status,
            status_comment,
            location_description,
            is_active, 
            created_at, 
            updated_at
        FROM system_tables 
        WHERE id = $1 AND is_active = true
        "#,
        table_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch system table: {}", e);
        error::ErrorInternalServerError("Error fetching system table")
    })?;

    match row {
        Some(row) => {
            // Получаем временные слоты
            let slots_rows = sqlx::query!(
                r#"
                SELECT 
                    id, 
                    table_id, 
                    day_of_week, 
                    open_time, 
                    close_time, 
                    is_next_day, 
                    is_active, 
                    created_at, 
                    updated_at
                FROM system_table_time_slots 
                WHERE table_id = $1
                ORDER BY day_of_week, open_time
                "#,
                table_id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());
            
            let time_slots: Vec<SystemTableTimeSlot> = slots_rows.into_iter().map(|s| {
                SystemTableTimeSlot {
                    id: s.id,
                    table_id: s.table_id,
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
                    table_id, 
                    photo_url, 
                    file_name, 
                    file_size, 
                    mime_type, 
                    is_main, 
                    uploaded_at, 
                    uploaded_by
                FROM system_table_photos 
                WHERE table_id = $1
                ORDER BY is_main DESC, uploaded_at DESC
                "#,
                table_id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());
            
            let photos: Vec<SystemTablePhoto> = photos_rows.into_iter().map(|p| {
                SystemTablePhoto {
                    id: p.id,
                    table_id: p.table_id,
                    photo_url: p.photo_url,
                    file_name: p.file_name,
                    file_size: p.file_size,
                    mime_type: p.mime_type,
                    is_main: p.is_main.unwrap_or(false),
                    uploaded_at: p.uploaded_at,
                    uploaded_by: p.uploaded_by,
                }
            }).collect();
            
            // Получаем поля таблицы
let fields_rows = sqlx::query!(
    r#"
    SELECT 
        id, 
        table_id, 
        field_name, 
        field_type, 
        display_order, 
        is_visible, 
        created_at
    FROM table_fields 
    WHERE table_id = $1
    ORDER BY display_order
    "#,
    table_id
)
.fetch_all(pool.get_ref())
.await
.unwrap_or_else(|_| Vec::new());

let fields: Vec<TableField> = fields_rows.into_iter().map(|row| {
    TableField {
        id: row.id,
        table_id: row.table_id.expect("REASON"),
        field_name: row.field_name,
        field_type: row.field_type,
        display_order: row.display_order.unwrap_or(0),
        is_visible: Some(row.is_visible.unwrap_or(true)),
        created_at: row.created_at,
    }
}).collect();
            
            // Определяем текущий статус
            let now = Local::now();
            let current_day = now.weekday().num_days_from_monday() as i32;
            let current_time = now.time();
            
            let mut current_status = "closed".to_string();
            
            let table_status = row.status.as_deref().unwrap_or("active");
            
            if table_status == "active" {
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
            
            let table_with_details = SystemTableWithDetails {
                table: SystemTable {
                    id: row.id,
                    name: row.name,
                    display_name: row.display_name,
                    table_type: row.table_type,
                    show_fact_table: row.show_fact_table,
                    fact_table_hint: row.fact_table_hint,
                    instruction: row.instruction,
                    map_link: row.map_link,
                    status: row.status,
                    status_comment: row.status_comment,
                    location_description: row.location_description,
                    is_active: row.is_active,
                    created_at: row.created_at,
                    updated_at: row.updated_at,
                },
                fields,
                time_slots,
                photos,
                current_status,
            };
            
            Ok(HttpResponse::Ok().json(table_with_details))
        },
        None => Err(error::ErrorNotFound("Системная таблица не найдена")),
    }
}

/// Получение таблицы по имени
pub async fn get_system_table_by_name(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<String>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let table_name = path.into_inner();

    log::info!("Getting system table by name: {}", table_name);

    let row = sqlx::query!(
        r#"
        SELECT 
            id, 
            name, 
            display_name, 
            table_type, 
            show_fact_table, 
            fact_table_hint, 
            instruction,
            map_link,
            status,
            status_comment,
            location_description,
            is_active, 
            created_at, 
            updated_at
        FROM system_tables 
        WHERE name = $1 AND is_active = true
        "#,
        table_name
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch system table: {}", e);
        error::ErrorInternalServerError("Error fetching system table")
    })?;

    match row {
        Some(row) => {
            // Получаем временные слоты
            let slots_rows = sqlx::query!(
                r#"
                SELECT 
                    id, 
                    table_id, 
                    day_of_week, 
                    open_time, 
                    close_time, 
                    is_next_day, 
                    is_active, 
                    created_at, 
                    updated_at
                FROM system_table_time_slots 
                WHERE table_id = $1
                ORDER BY day_of_week, open_time
                "#,
                row.id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());
            
            let time_slots: Vec<SystemTableTimeSlot> = slots_rows.into_iter().map(|s| {
                SystemTableTimeSlot {
                    id: s.id,
                    table_id: s.table_id,
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
                    table_id, 
                    photo_url, 
                    file_name, 
                    file_size, 
                    mime_type, 
                    is_main, 
                    uploaded_at, 
                    uploaded_by
                FROM system_table_photos 
                WHERE table_id = $1
                ORDER BY is_main DESC, uploaded_at DESC
                "#,
                row.id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());
            
            let photos: Vec<SystemTablePhoto> = photos_rows.into_iter().map(|p| {
                SystemTablePhoto {
                    id: p.id,
                    table_id: p.table_id,
                    photo_url: p.photo_url,
                    file_name: p.file_name,
                    file_size: p.file_size,
                    mime_type: p.mime_type,
                    is_main: p.is_main.unwrap_or(false),
                    uploaded_at: p.uploaded_at,
                    uploaded_by: p.uploaded_by,
                }
            }).collect();
            
           // Получаем поля таблицы
let fields_rows = sqlx::query!(
    r#"
    SELECT 
        id, 
        table_id, 
        field_name, 
        field_type, 
        display_order, 
        is_visible, 
        created_at
    FROM table_fields 
    WHERE table_id = $1
    ORDER BY display_order
    "#,
    row.id
)
.fetch_all(pool.get_ref())
.await
.unwrap_or_else(|_| Vec::new());

let fields: Vec<TableField> = fields_rows.into_iter().map(|row| {
    TableField {
        id: row.id,
        table_id: row.table_id.expect("REASON"),
        field_name: row.field_name,
        field_type: row.field_type,
        display_order: row.display_order.unwrap_or(0),
        is_visible: Some(row.is_visible.unwrap_or(true)),
        created_at: row.created_at,
    }
}).collect();
            
            // Определяем текущий статус
            let now = Local::now();
            let current_day = now.weekday().num_days_from_monday() as i32;
            let current_time = now.time();
            
            let mut current_status = "closed".to_string();
            
            let table_status = row.status.as_deref().unwrap_or("active");
            
            if table_status == "active" {
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
            
            let table_with_details = SystemTableWithDetails {
                table: SystemTable {
                    id: row.id,
                    name: row.name,
                    display_name: row.display_name,
                    table_type: row.table_type,
                    show_fact_table: row.show_fact_table,
                    fact_table_hint: row.fact_table_hint,
                    instruction: row.instruction,
                    map_link: row.map_link,
                    status: row.status,
                    status_comment: row.status_comment,
                    location_description: row.location_description,
                    is_active: row.is_active,
                    created_at: row.created_at,
                    updated_at: row.updated_at,
                },
                fields,
                time_slots,
                photos,
                current_status,
            };
            
            Ok(HttpResponse::Ok().json(table_with_details))
        },
        None => Err(error::ErrorNotFound("Таблица не найдена")),
    }
}

/// Создание новой системной таблицы
pub async fn create_system_table(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    table_data: web::Json<CreateSystemTableRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Creating new system table: {}", table_data.name);

    // Проверяем, существует ли уже таблица с таким именем
    let existing = sqlx::query!(
        "SELECT id FROM system_tables WHERE name = $1",
        table_data.name
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check existing table: {}", e);
        error::ErrorInternalServerError("Error checking table existence")
    })?;

    if existing.is_some() {
        return Err(error::ErrorBadRequest("Таблица с таким именем уже существует"));
    }

    let status = table_data.status.as_deref().unwrap_or("active");

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Создаем основную таблицу
    let table_record = sqlx::query!(
        r#"
        INSERT INTO system_tables (
            name, 
            display_name, 
            table_type, 
            show_fact_table, 
            fact_table_hint, 
            instruction,
            map_link,
            status,
            status_comment,
            location_description,
            is_active
        ) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true) 
        RETURNING id
        "#,
        table_data.name,
        table_data.display_name,
        table_data.table_type,
        table_data.show_fact_table.unwrap_or(false),
        table_data.fact_table_hint,
        table_data.instruction,
        table_data.map_link,
        status,
        table_data.status_comment,
        table_data.location_description
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to create system table: {}", e);
        error::ErrorInternalServerError("Error creating system table")
    })?;

    // Создаем поля таблицы на основе типа
    let fields = get_default_fields_for_type(&table_data.table_type);
    
    for (index, field) in fields.iter().enumerate() {
        sqlx::query!(
            r#"
            INSERT INTO table_fields (table_id, field_name, field_type, display_order, is_visible) 
            VALUES ($1, $2, $3, $4, true)
            "#,
            table_record.id,
            field.name,
            field.field_type,
            index as i32
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to create table field: {}", e);
            error::ErrorInternalServerError("Error creating table fields")
        })?;
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "id": table_record.id,
        "message": "Системная таблица успешно создана"
    })))
}

/// Обновление системной таблицы
pub async fn update_system_table(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    table_data: web::Json<UpdateSystemTableRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let table_id = path.into_inner();

    log::info!("Updating system table ID: {}", table_id);

    // Получаем текущие данные
    let current = sqlx::query!(
        r#"
        SELECT display_name, table_type, show_fact_table, fact_table_hint, instruction,
               map_link, status, status_comment, location_description
        FROM system_tables
        WHERE id = $1
        "#,
        table_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch system table: {}", e);
        error::ErrorInternalServerError("Error fetching system table")
    })?;

    let current = match current {
        Some(c) => c,
        None => return Err(error::ErrorNotFound("Системная таблица не найдена")),
    };

    // Строим динамический запрос
    let mut updates = Vec::new();
    let mut params: Vec<String> = Vec::new();
    let mut param_counter = 1;

    if let Some(display_name) = &table_data.display_name {
        updates.push(format!("display_name = ${}", param_counter));
        params.push(display_name.clone());
        param_counter += 1;
    }

    if let Some(table_type) = &table_data.table_type {
        updates.push(format!("table_type = ${}", param_counter));
        params.push(table_type.clone());
        param_counter += 1;
    }

    if let Some(show_fact_table) = table_data.show_fact_table {
        updates.push(format!("show_fact_table = ${}", param_counter));
        params.push(show_fact_table.to_string());
        param_counter += 1;
    }

    if let Some(fact_table_hint) = &table_data.fact_table_hint {
        updates.push(format!("fact_table_hint = ${}", param_counter));
        params.push(fact_table_hint.clone());
        param_counter += 1;
    }

    if let Some(instruction) = &table_data.instruction {
        updates.push(format!("instruction = ${}", param_counter));
        params.push(instruction.clone());
        param_counter += 1;
    }

    if let Some(map_link) = &table_data.map_link {
        updates.push(format!("map_link = ${}", param_counter));
        params.push(map_link.clone());
        param_counter += 1;
    }

    if let Some(status) = &table_data.status {
        updates.push(format!("status = ${}", param_counter));
        params.push(status.clone());
        param_counter += 1;
    }

    if let Some(status_comment) = &table_data.status_comment {
        updates.push(format!("status_comment = ${}", param_counter));
        params.push(status_comment.clone());
        param_counter += 1;
    }

    if let Some(location_description) = &table_data.location_description {
        updates.push(format!("location_description = ${}", param_counter));
        params.push(location_description.clone());
        param_counter += 1;
    }

    updates.push(format!("updated_at = NOW()"));

    if updates.is_empty() {
        return Ok(HttpResponse::Ok().json("Нет данных для обновления"));
    }

    let query = format!(
        "UPDATE system_tables SET {} WHERE id = ${}",
        updates.join(", "),
        param_counter
    );

    let mut query_builder = sqlx::query(&query);
    
    for param in &params {
        query_builder = query_builder.bind(param);
    }
    
    query_builder = query_builder.bind(table_id);

    let result = query_builder
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to update system table: {}", e);
            error::ErrorInternalServerError("Error updating system table")
        })?;

    if result.rows_affected() > 0 {
        Ok(HttpResponse::Ok().json("Системная таблица успешно обновлена"))
    } else {
        Err(error::ErrorNotFound("Системная таблица не найдена"))
    }
}

/// Удаление системной таблицы (мягкое удаление)
pub async fn delete_system_table(
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

    let table_id = path.into_inner();

    log::info!("Deleting system table ID: {}", table_id);

    // Проверяем, используется ли таблица в организациях
    let org_count = sqlx::query!(
        r#"
        SELECT COUNT(*) as count
        FROM organization_tables
        WHERE table_id = $1
        "#,
        table_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check organization tables: {}", e);
        error::ErrorInternalServerError("Error checking organization dependencies")
    })?;

    // Проверяем, используется ли таблица в компаниях
    let company_count = sqlx::query!(
        r#"
        SELECT COUNT(*) as count
        FROM companies_tables
        WHERE table_id = $1
        "#,
        table_id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check company tables: {}", e);
        error::ErrorInternalServerError("Error checking company dependencies")
    })?;

    // Если таблица привязана к организациям или компаниям, не удаляем
    if org_count.count.unwrap_or(0) > 0 || company_count.count.unwrap_or(0) > 0 {
        let mut error_message = String::from("Невозможно удалить таблицу, так как она привязана к: ");
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

    // Мягкое удаление
    let result = sqlx::query!(
        "UPDATE system_tables SET is_active = false WHERE id = $1",
        table_id
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to delete system table: {}", e);
        error::ErrorInternalServerError("Error deleting system table")
    })?;

    if result.rows_affected() > 0 {
        Ok(HttpResponse::Ok().json("Системная таблица успешно удалена"))
    } else {
        Err(error::ErrorNotFound("Системная таблица не найдена"))
    }
}

// ==================== МЕТОДЫ ДЛЯ ВРЕМЕННЫХ СЛОТОВ ====================

/// Получение временных слотов таблицы
pub async fn get_system_table_time_slots(
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

    let table_id = path.into_inner();

    log::info!("Getting time slots for system table ID: {}", table_id);

    let rows = sqlx::query!(
        r#"
        SELECT 
            id, 
            table_id, 
            day_of_week, 
            open_time, 
            close_time, 
            is_next_day, 
            is_active, 
            created_at, 
            updated_at
        FROM system_table_time_slots 
        WHERE table_id = $1
        ORDER BY day_of_week, open_time
        "#,
        table_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch time slots: {}", e);
        error::ErrorInternalServerError("Error fetching time slots")
    })?;

    let slots: Vec<SystemTableTimeSlot> = rows.into_iter().map(|row| {
        SystemTableTimeSlot {
            id: row.id,
            table_id: row.table_id,
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

/// Добавление временного слота для таблицы
pub async fn add_system_table_time_slot(
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

    let table_id = path.into_inner();

    log::info!("Adding time slot for system table ID: {}", table_id);

    // Проверяем существование таблицы
    let table_exists = sqlx::query!(
        r#"
        SELECT id
        FROM system_tables
        WHERE id = $1 AND is_active = true
        "#,
        table_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check system table: {}", e);
        error::ErrorInternalServerError("Error checking system table")
    })?;

    if table_exists.is_none() {
        return Err(error::ErrorNotFound("Системная таблица не найдена"));
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
        INSERT INTO system_table_time_slots 
            (table_id, day_of_week, open_time, close_time, is_next_day, is_active, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        RETURNING id
        "#,
        table_id,
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
pub async fn update_system_table_time_slot(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (table_id, slot_id)
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

    let (table_id, slot_id) = path.into_inner();

    log::info!("Updating time slot ID: {} for table ID: {}", slot_id, table_id);

    // Получаем текущие данные слота
    let current = sqlx::query!(
        r#"
        SELECT day_of_week, open_time, close_time, is_next_day, is_active
        FROM system_table_time_slots
        WHERE id = $1 AND table_id = $2
        "#,
        slot_id,
        table_id
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
        UPDATE system_table_time_slots 
        SET day_of_week = $1, open_time = $2, close_time = $3, is_next_day = $4, is_active = $5, updated_at = NOW()
        WHERE id = $6 AND table_id = $7
        "#,
        day_of_week as i16,
        open_time,
        close_time,
        is_next_day,
        is_active,
        slot_id,
        table_id
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
pub async fn delete_system_table_time_slot(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (table_id, slot_id)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let (table_id, slot_id) = path.into_inner();

    log::info!("Deleting time slot ID: {} for table ID: {}", slot_id, table_id);

    let result = sqlx::query!(
        r#"
        DELETE FROM system_table_time_slots
        WHERE id = $1 AND table_id = $2
        "#,
        slot_id,
        table_id
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

// ==================== МЕТОДЫ ДЛЯ ФОТОГРАФИЙ ====================

/// Загрузка фотографии для таблицы
pub async fn upload_system_table_photo(
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
    
    let table_id = path.into_inner();

    log::info!("Uploading photo for system table ID: {}", table_id);

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

    // Проверяем существование таблицы
    let table_exists = sqlx::query!(
        r#"
        SELECT id
        FROM system_tables
        WHERE id = $1 AND is_active = true
        "#,
        table_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check system table: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if table_exists.is_none() {
        return Err(error::ErrorNotFound("Системная таблица не найдена"));
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
        
        let unique_filename = format!("{}_{}.{}", Uuid::new_v4(), table_id, file_extension);
        let filepath = Path::new(UPLOAD_DIR).join(&unique_filename);
        let file_url = format!("/uploads/system_tables/{}", unique_filename);

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
        // Проверяем, есть ли уже фотографии у этой таблицы
        let photo_count = sqlx::query!(
            r#"
            SELECT COUNT(*) as count
            FROM system_table_photos
            WHERE table_id = $1
            "#,
            table_id
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
            INSERT INTO system_table_photos (table_id, photo_url, file_name, file_size, mime_type, is_main, uploaded_by)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            RETURNING id
            "#,
            table_id,
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
pub async fn delete_system_table_photo(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (table_id, photo_id)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let (table_id, photo_id) = path.into_inner();

    log::info!("Deleting photo ID: {} for table ID: {}", photo_id, table_id);

    // Получаем информацию о фотографии для удаления файла
    let photo = sqlx::query!(
        r#"
        SELECT photo_url, is_main
        FROM system_table_photos
        WHERE id = $1 AND table_id = $2
        "#,
        photo_id,
        table_id
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
        DELETE FROM system_table_photos
        WHERE id = $1 AND table_id = $2
        "#,
        photo_id,
        table_id
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
            FROM system_table_photos
            WHERE table_id = $1 AND id != $2
            ORDER BY uploaded_at
            LIMIT 1
            "#,
            table_id,
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
                UPDATE system_table_photos
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
pub async fn set_main_system_table_photo(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<(i32, i32)>, // (table_id, photo_id)
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let (table_id, photo_id) = path.into_inner();

    log::info!("Setting main photo ID: {} for table ID: {}", photo_id, table_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Сбрасываем флаг is_main для всех фотографий этой таблицы
    sqlx::query!(
        r#"
        UPDATE system_table_photos
        SET is_main = false
        WHERE table_id = $1
        "#,
        table_id
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
        UPDATE system_table_photos
        SET is_main = true
        WHERE id = $1 AND table_id = $2
        RETURNING id
        "#,
        photo_id,
        table_id
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

/// Вспомогательная функция для получения полей по умолчанию для типа таблицы
fn get_default_fields_for_type(table_type: &str) -> Vec<DefaultField> {
    match table_type {
        "cars" => vec![
            DefaultField {
                name: "car_number".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "car_brand".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "organization".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "unload_place".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "valid_until".to_string(),
                field_type: "date".to_string(),
            },
            DefaultField {
                name: "time_range".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "status".to_string(),
                field_type: "text".to_string(),
            },
        ],
        "people" => vec![
            DefaultField {
                name: "organization".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "last_name".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "first_name".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "middle_name".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "valid_until".to_string(),
                field_type: "date".to_string(),
            },
            DefaultField {
                name: "pass_time".to_string(),
                field_type: "text".to_string(),
            },
        ],
        _ => vec![],
    }
}

/// Структура для полей по умолчанию
#[derive(Debug)]
struct DefaultField {
    name: String,
    field_type: String,
}