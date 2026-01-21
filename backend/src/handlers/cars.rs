use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};
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
        VALUES ($1, $2, $3, $4, $5, $6, $7, 1)
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