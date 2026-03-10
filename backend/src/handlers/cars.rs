use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool};
use serde_json::json;
use log;
use serde::{Deserialize};
use chrono::{NaiveDate, NaiveTime, NaiveDateTime, Utc};

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

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    // Получаем ID пользователя из токена
    let user_id = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        claims.sub
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|_| error::ErrorUnauthorized("User not found"))?
    .id;

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
            status,
            territory_status
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, 0, 0)
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

    // Добавляем запись в историю о создании автомобиля
    sqlx::query!(
        r#"
        INSERT INTO cars_history (
            car_id,
            user_id,
            action_type,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, NOW())
        "#,
        car_id,
        user_id,
        "create",
        format!("Автомобиль {} {} создан", car_data.car_number, car_data.car_brand)
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add car history entry: {}", e);
        error::ErrorInternalServerError("Error adding car history entry")
    })?;

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
        organization_id: Option<i32>,
        company: Option<String>,
        company_id: Option<i32>,
        unload_place: Option<String>,
        unload_places: Vec<String>,
        entry_date_to: Option<chrono::NaiveDate>,
        entry_time_from: Option<chrono::NaiveTime>,
        entry_time_to: Option<chrono::NaiveTime>,
        status: i32,
        application_id: Option<i32>,
        territory_status: Option<i32>,
        territory_entry_time: Option<chrono::NaiveDateTime>,
    }

    let rows = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.car_number,
            c.car_brand,
            c.unload_place,
            c.territory_status,
            c.territory_entry_time,
            o.name as organization,
            o.id as organization_id,
            c2.name as company,
            c2.id as company_id,
            c.entry_date_to,
            c.entry_time_from,
            c.entry_time_to,
            c.status,
            app.id as application_id
        FROM cars c
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c2 ON app.company_id = c2.id
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
            organization: Some(row.organization),
            organization_id: Some(row.organization_id),
            company: Some(row.company),
            company_id: Some(row.company_id),
            unload_place: row.unload_place,
            unload_places,
            entry_date_to: Some(row.entry_date_to),
            entry_time_from: Some(row.entry_time_from),
            entry_time_to: Some(row.entry_time_to),
            status: row.status.unwrap_or(0),
            application_id: Some(row.application_id),
            territory_status: row.territory_status,
            territory_entry_time: row.territory_entry_time,
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
        car_number: String,
        car_brand: String,
        organization: Option<String>,
        organization_id: Option<i32>,
        company: Option<String>,
        company_id: Option<i32>,
        unload_place: Option<String>,
        unload_places: Vec<String>,
        entry_date_to: Option<chrono::NaiveDate>,
        entry_time_from: Option<chrono::NaiveTime>,
        entry_time_to: Option<chrono::NaiveTime>,
        status: i32,
        application_id: Option<i32>,
        territory_status: Option<i32>,
        territory_entry_time: Option<chrono::NaiveDateTime>,
    }

    let rows = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.car_number,
            c.car_brand,
            c.unload_place,
            c.territory_status,
            c.territory_entry_time,
            o.name as organization,
            o.id as organization_id,
            c2.name as company,
            c2.id as company_id,
            c.entry_date_to,
            c.entry_time_from,
            c.entry_time_to,
            c.status,
            app.id as application_id
        FROM cars c
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c2 ON app.company_id = c2.id
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
                c.car_number,
                c.car_brand,
                c.unload_place,
                c.territory_status,
                c.territory_entry_time,
                o.name as organization,
                o.id as organization_id,
                c2.name as company,
                c2.id as company_id,
                c.entry_date_to,
                c.entry_time_from,
                c.entry_time_to,
                c.status,
                app.id as application_id
            FROM cars c
            JOIN attachments a ON c.attachment_id = a.id
            JOIN applications app ON a.application_id = app.id
            LEFT JOIN organizations o ON app.organization_id = o.id
            LEFT JOIN companies c2 ON app.company_id = c2.id
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
        
        let mut cars: Vec<FactCar> = Vec::new();
        
        for row in alt_rows {
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
            
            cars.push(FactCar {
                id: row.id,
                car_number: row.car_number,
                car_brand: row.car_brand,
                organization: Some(row.organization),
                organization_id: Some(row.organization_id),
                company: Some(row.company),
                company_id: Some(row.company_id),
                unload_place: row.unload_place,
                unload_places,
                entry_date_to: Some(row.entry_date_to),
                entry_time_from: Some(row.entry_time_from),
                entry_time_to: Some(row.entry_time_to),
                status: row.status.unwrap_or(0),
                application_id: Some(row.application_id),
                territory_status: row.territory_status,
                territory_entry_time: row.territory_entry_time,
            });
        }
        
        return Ok(HttpResponse::Ok().json(cars));
    }

    let mut cars: Vec<FactCar> = Vec::new();
    
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
        
        cars.push(FactCar {
            id: row.id,
            car_number: row.car_number,
            car_brand: row.car_brand,
            organization: Some(row.organization),
            organization_id: Some(row.organization_id),
            company: Some(row.company),
            company_id: Some(row.company_id),
            unload_place: row.unload_place,
            unload_places,
            entry_date_to: Some(row.entry_date_to),
            entry_time_from: Some(row.entry_time_from),
            entry_time_to: Some(row.entry_time_to),
            status: row.status.unwrap_or(0),
            application_id: Some(row.application_id),
            territory_status: row.territory_status,
            territory_entry_time: row.territory_entry_time,
        });
    }

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

/// Проверка активности автомобиля по номеру, марке, организации и компании
pub async fn check_active_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<CheckActiveCarQuery>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Checking active car: number={}, brand={}, org_id={:?}, company_id={:?}", 
               query.car_number, query.car_brand, query.organization_id, query.company_id);

    let now = Utc::now();
    let today = now.date_naive();
    let current_time = now.time();

    let car = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.car_number,
            c.car_brand,
            c.entry_date_to,
            c.entry_time_to,
            a.application_id,
            app.application_number,
            COALESCE(o.name, '') as organization_name,
            COALESCE(comp.name, '') as company_name
        FROM cars c
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies comp ON app.company_id = comp.id
        WHERE c.status = 1
        AND LOWER(TRIM(c.car_number)) = LOWER(TRIM($1))
        AND LOWER(TRIM(c.car_brand)) = LOWER(TRIM($2))
        AND (
            ($3::integer IS NULL AND app.organization_id IS NULL)
            OR app.organization_id = $3
        )
        AND (
            ($4::integer IS NULL AND app.company_id IS NULL)
            OR app.company_id = $4
        )
        AND (
            c.entry_date_to > $5
            OR (c.entry_date_to = $5 AND c.entry_time_to > $6)
        )
        LIMIT 1
        "#,
        query.car_number,
        query.car_brand,
        query.organization_id,
        query.company_id,
        today,
        current_time
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to check active car: {}", e);
        error::ErrorInternalServerError("Error checking active car")
    })?;

    if let Some(car) = car {
        Ok(HttpResponse::Ok().json(json!({
            "active": true,
            "car_id": car.id,
            "car_number": car.car_number,
            "car_brand": car.car_brand,
            "entry_date_to": car.entry_date_to,
            "entry_time_to": car.entry_time_to,
            "application_id": car.application_id,
            "application_number": car.application_number,
            "organization_name": car.organization_name,
            "company_name": car.company_name
        })))
    } else {
        Ok(HttpResponse::Ok().json(json!({
            "active": false
        })))
    }
}

#[derive(Debug, Deserialize)]
pub struct CheckActiveCarQuery {
    pub car_number: String,
    pub car_brand: String,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
}