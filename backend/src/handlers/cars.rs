use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool};
use serde_json::json;
use log;
use serde::{Deserialize};
use chrono::{NaiveDate, NaiveTime};

use crate::auth::decode_token;

#[derive(Debug, Deserialize)]
pub struct CarData {
    pub car_number: String,
    pub car_brand: String,
    pub unload_place: Option<String>,
    pub entry_date_from: Option<String>,
    pub entry_time_from: Option<String>,
    pub entry_date_to: Option<String>,
    pub entry_time_to: Option<String>,
    pub unload_places: Vec<i32>,
}

/// Создание автомобиля
pub async fn create_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    car_data: web::Json<CarData>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    // Конвертируем строки в NaiveDate и NaiveTime
    let entry_date_from: Option<NaiveDate> = car_data.entry_date_from.as_ref()
        .and_then(|s| NaiveDate::parse_from_str(s, "%Y-%m-%d").ok());
    
    let entry_time_from: Option<NaiveTime> = car_data.entry_time_from.as_ref()
        .and_then(|s| NaiveTime::parse_from_str(s, "%H:%M:%S").ok());
    
    let entry_date_to: Option<NaiveDate> = car_data.entry_date_to.as_ref()
        .and_then(|s| NaiveDate::parse_from_str(s, "%Y-%m-%d").ok());
    
    let entry_time_to: Option<NaiveTime> = car_data.entry_time_to.as_ref()
        .and_then(|s| NaiveTime::parse_from_str(s, "%H:%M:%S").ok());

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Создаем автомобиль (attachment_id будет установлен позже при создании вложения)
    let car_result = sqlx::query!(
        r#"
        INSERT INTO cars (
            car_number,
            car_brand,
            unload_place,
            entry_date_from,
            entry_time_from,
            entry_date_to,
            entry_time_to,
            status
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, 0)
        RETURNING id
        "#,
        car_data.car_number,
        car_data.car_brand,
        car_data.unload_place.as_deref(),
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

    // Создаем связи с местами разгрузки
    for &place_id in &car_data.unload_places {
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

    // Фиксируем транзакцию
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Car created successfully",
        "car_id": car_id
    })))
}

/// Получение активных машин для ВСЕХ таблиц с типом cars
pub async fn get_active_cars_for_tables(
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

    log::info!("Getting active cars for ALL tables with type cars");

    #[derive(Debug, serde::Serialize)]
    struct TableCar {
        id: i32,
        car_number: String,
        car_brand: String,
        organization: Option<String>,
        unload_place: Option<String>,
        unload_places: Vec<String>,
        entry_date_to: Option<chrono::NaiveDate>,
        entry_time_from: Option<chrono::NaiveTime>,
        entry_time_to: Option<chrono::NaiveTime>,
        status: i32,
        application_id: Option<i32>,
    }

    // Получаем ВСЕ активные машины из согласованных заявок
    let rows = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.car_number,
            c.car_brand,
            c.unload_place, 
            COALESCE(o.name, co.name) as organization,
            c.entry_date_to,
            c.entry_time_from,
            c.entry_time_to,
            c.status,
            app.id as application_id
        FROM cars c
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies co ON app.company_id = co.id
        WHERE c.status = 1
        AND app.confirmation = 'Согласовано'
        AND app.status IN ('В работе', 'Завершено')
        AND LOWER(TRIM(c.car_number)) != 'по факту'
        ORDER BY c.car_number
        "#
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch active cars: {}", e);
        error::ErrorInternalServerError("Error fetching active cars")
    })?;

    log::info!("Found {} active cars (excluding 'по факту' numbers)", rows.len());

    // Для каждой машины получаем места разгрузки отдельно
    let mut cars: Vec<TableCar> = Vec::new();
    
    for row in rows {
        let places_rows = sqlx::query!(
            r#"
            SELECT up.name
            FROM car_unload_places cup
            JOIN unload_places up ON cup.unload_place_id = up.id
            WHERE cup.car_id = $1
            ORDER BY cup.order_index
            "#,
            row.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());
        
        let unload_places: Vec<String> = places_rows.iter()
            .filter_map(|p| Some(p.name.clone()))
            .collect();
        
        cars.push(TableCar {
            id: row.id,
            car_number: row.car_number,
            car_brand: row.car_brand,
            unload_place: row.unload_place,
            organization: row.organization,
            unload_places,
            entry_date_to: Some(row.entry_date_to),
            entry_time_from: Some(row.entry_time_from),
            entry_time_to: Some(row.entry_time_to),
            status: row.status.unwrap_or(0),
            application_id: Some(row.application_id),
        });
    }

    Ok(HttpResponse::Ok().json(cars))
}

/// Получение машин "по факту"
pub async fn get_fact_cars_for_tables(
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

    log::info!("Getting fact cars for tables");

    #[derive(Debug, serde::Serialize)]
    struct FactCar {
        id: i32,
        organization: Option<String>,
        unload_place: Option<String>,
        entry_date_to: Option<chrono::NaiveDate>,
        entry_time_from: Option<chrono::NaiveTime>,
        entry_time_to: Option<chrono::NaiveTime>,
        status: i32,
        application_id: Option<i32>,
    }

    let rows = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.unload_place,
            COALESCE(o.name, co.name) as organization,
            c.entry_date_to,
            c.entry_time_from,
            c.entry_time_to,
            c.status,
            app.id as application_id
        FROM cars c
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies co ON app.company_id = co.id
        WHERE c.status = 1
        AND LOWER(TRIM(c.car_number)) = 'по факту'
        AND app.confirmation = 'Согласовано'
        AND app.status IN ('В работе', 'Завершено')
        ORDER BY organization, c.entry_date_to
        "#
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch fact cars: {}", e);
        error::ErrorInternalServerError("Error fetching fact cars")
    })?;

    log::info!("Found {} fact cars (with number 'по факту')", rows.len());

    // Если ничего не нашли, пробуем альтернативный вариант написания
    if rows.is_empty() {
        log::info!("Trying alternative search for 'по факту'...");
        
        let alt_rows = sqlx::query!(
            r#"
            SELECT 
                c.id,
                c.unload_place,
                COALESCE(o.name, co.name) as organization,
                c.entry_date_to,
                c.entry_time_from,
                c.entry_time_to,
                c.status,
                app.id as application_id
            FROM cars c
            JOIN attachments a ON c.attachment_id = a.id
            JOIN applications app ON a.application_id = app.id
            LEFT JOIN organizations o ON app.organization_id = o.id
            LEFT JOIN companies co ON app.company_id = co.id
            WHERE c.status = 1
            AND (
                c.car_number ILIKE '%по факту%' OR 
                c.car_number ILIKE '%пофакту%' OR
                c.car_number ILIKE '%факт%'
            )
            AND app.confirmation = 'Согласовано'
            AND app.status IN ('В работе', 'Завершено')
            ORDER BY organization, c.entry_date_to
            "#
        )
        .fetch_all(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to fetch alternative fact cars: {}", e);
            error::ErrorInternalServerError("Error fetching alternative fact cars")
        })?;
        
        log::info!("Alternative search found {} cars", alt_rows.len());
        
        let cars: Vec<FactCar> = alt_rows.into_iter().map(|row| {
            FactCar {
                id: row.id,
                organization: row.organization,
                unload_place: row.unload_place,
                entry_date_to: Some(row.entry_date_to),
                entry_time_from: Some(row.entry_time_from),
                entry_time_to: Some(row.entry_time_to),
                status: row.status.unwrap_or(0),
                application_id: Some(row.application_id),
            }
        }).collect();
        
        return Ok(HttpResponse::Ok().json(cars));
    }

    let cars: Vec<FactCar> = rows.into_iter().map(|row| {
        FactCar {
            id: row.id,
            organization: row.organization,
            unload_place: row.unload_place,
            entry_date_to: Some(row.entry_date_to),
            entry_time_from: Some(row.entry_time_from),
            entry_time_to: Some(row.entry_time_to),
            status: row.status.unwrap_or(0),
            application_id: Some(row.application_id),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(cars))
}

/// Получение связей всех активных машин с местами разгрузки
pub async fn get_car_unload_places(
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

    log::info!("Getting car unload places for all active cars");

    #[derive(Debug, serde::Serialize)]
    struct CarUnloadPlaceInfo {
        car_id: i32,
        unload_place_id: i32,
        unload_place_name: String,
    }

    let rows = sqlx::query!(
        r#"
        SELECT 
            cup.car_id,
            cup.unload_place_id,
            up.name as unload_place_name
        FROM car_unload_places cup
        JOIN unload_places up ON cup.unload_place_id = up.id
        JOIN cars c ON cup.car_id = c.id
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        WHERE c.status = 1
        AND app.confirmation = 'Согласовано'
        AND app.status IN ('В работе', 'Завершено')
        ORDER BY cup.car_id, cup.order_index
        "#
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch car unload places: {}", e);
        error::ErrorInternalServerError("Error fetching car unload places")
    })?;

    let places: Vec<CarUnloadPlaceInfo> = rows.into_iter().map(|row| {
        CarUnloadPlaceInfo {
            car_id: row.car_id,
            unload_place_id: row.unload_place_id,
            unload_place_name: row.unload_place_name,
        }
    }).collect();

    log::info!("Found {} car unload place records", places.len());

    Ok(HttpResponse::Ok().json(places))
}

/// Получение связей машин "по факту" с местами разгрузки
pub async fn get_fact_car_unload_places(
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

    log::info!("Getting fact car unload places");

    #[derive(Debug, serde::Serialize)]
    struct CarUnloadPlaceInfo {
        car_id: i32,
        unload_place_id: i32,
        unload_place_name: String,
    }

    let rows = sqlx::query!(
        r#"
        SELECT 
            cup.car_id,
            cup.unload_place_id,
            up.name as unload_place_name
        FROM car_unload_places cup
        JOIN unload_places up ON cup.unload_place_id = up.id
        JOIN cars c ON cup.car_id = c.id
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        WHERE c.status = 1
        AND app.confirmation = 'Согласовано'
        AND app.status IN ('В работе', 'Завершено')
        AND LOWER(TRIM(c.car_number)) = 'по факту'
        ORDER BY cup.car_id, cup.order_index
        "#
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch fact car unload places: {}", e);
        error::ErrorInternalServerError("Error fetching fact car unload places")
    })?;

    let places: Vec<CarUnloadPlaceInfo> = rows.into_iter().map(|row| {
        CarUnloadPlaceInfo {
            car_id: row.car_id,
            unload_place_id: row.unload_place_id,
            unload_place_name: row.unload_place_name,
        }
    }).collect();

    log::info!("Found {} fact car unload place records", places.len());

    Ok(HttpResponse::Ok().json(places))
}