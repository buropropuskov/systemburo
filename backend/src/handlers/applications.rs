use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;

use crate::models::applications::{Application, ApplicationWithCars, ApplicationSubmitRequest};
use crate::models::cars::{CarWithUnloadPlaces, Car};
use crate::models::unload_places::CarUnloadPlace;

use crate::auth::decode_token;

/// Обновленный обработчик отправки заявки (v2)
pub async fn submit_application_v2(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<ApplicationSubmitRequest>,
) -> Result<HttpResponse, Error> {
    // проверка токена
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // базовая валидация
    if form.application.organization.is_none()
        || form.application.responsible_person.is_none()
        || form.application.contact_phone.is_none()
        || form.application.entry_date_from.is_none()
        || form.application.entry_date_to.is_none()
        || form.application.entry_time_from.is_none()
        || form.application.entry_time_to.is_none()
    {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "Missing required fields"
        })));
    }

    // вставка заявки
    let application_id = sqlx::query!(
        "INSERT INTO applications (organization, responsible_person, contact_phone, 
         entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
         VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id",
        form.application.organization.as_ref().unwrap(),
        form.application.responsible_person.as_ref().unwrap(),
        form.application.contact_phone.as_ref().unwrap(),
        form.application.entry_date_from.unwrap(),
        form.application.entry_date_to.unwrap(),
        form.application.entry_time_from.unwrap(),
        form.application.entry_time_to.unwrap()
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Error creating application: {}", e);
        error::ErrorInternalServerError("Error creating application")
    })?
    .id;

    // вставка машин
    for car_with_places in &form.cars {
        let car = &car_with_places.car;

        if car.car_number.is_none() || car.car_brand.is_none() {
            return Ok(HttpResponse::BadRequest().json(json!({
                "message": "Car number and brand are required"
            })));
        }

        let car_id = sqlx::query!(
            "INSERT INTO cars (application_id, car_number, car_brand, 
             entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
             VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id",
            application_id,
            car.car_number.as_ref().unwrap(),
            car.car_brand.as_ref().unwrap(),
            form.application.entry_date_from.unwrap(),
            form.application.entry_date_to.unwrap(),
            form.application.entry_time_from.unwrap(),
            form.application.entry_time_to.unwrap()
        )
        .fetch_one(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Error adding car: {}", e);
            error::ErrorInternalServerError("Error adding car")
        })?
        .id;

        for (index, unload_place) in car_with_places.unload_places.iter().enumerate() {
            sqlx::query!(
                "INSERT INTO car_unload_places (car_id, unload_place_id, order_index, planned_time, notes) 
                 VALUES ($1,$2,$3,$4,$5)",
                car_id,
                unload_place.unload_place_id,
                (index + 1) as i32,
                unload_place.planned_time,
                unload_place.notes
            )
            .execute(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Error adding unload place: {}", e);
                error::ErrorInternalServerError("Error adding unload place")
            })?;
        }
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application submitted successfully",
        "application_id": application_id
    })))
}

/// Старый обработчик отправки заявки
pub async fn submit_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<ApplicationSubmitRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    if form.application.organization.is_none()
        || form.application.responsible_person.is_none()
        || form.application.contact_phone.is_none()
        || form.application.entry_date_from.is_none()
        || form.application.entry_date_to.is_none()
        || form.application.entry_time_from.is_none()
        || form.application.entry_time_to.is_none()
    {
        return Err(error::ErrorBadRequest("Missing required fields in application"));
    }

    let application_id = sqlx::query!(
        "INSERT INTO applications (organization, responsible_person, contact_phone, 
         entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
         VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id",
        form.application.organization.as_ref().unwrap(),
        form.application.responsible_person.as_ref().unwrap(),
        form.application.contact_phone.as_ref().unwrap(),
        form.application.entry_date_from.unwrap(),
        form.application.entry_date_to.unwrap(),
        form.application.entry_time_from.unwrap(),
        form.application.entry_time_to.unwrap()
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Error creating application: {}", e);
        error::ErrorInternalServerError("Error creating application")
    })?
    .id;

    for car_with_places in &form.cars {
        let car = &car_with_places.car;

        if car.car_number.is_none() || car.car_brand.is_none() {
            return Err(error::ErrorBadRequest("Missing required fields for car"));
        }

        let car_id = sqlx::query!(
            "INSERT INTO cars (application_id, car_number, car_brand, 
             entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
             VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id",
            application_id,
            car.car_number.as_ref().unwrap(),
            car.car_brand.as_ref().unwrap(),
            form.application.entry_date_from.unwrap(),
            form.application.entry_date_to.unwrap(),
            form.application.entry_time_from.unwrap(),
            form.application.entry_time_to.unwrap()
        )
        .fetch_one(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Error adding car: {}", e);
            error::ErrorInternalServerError("Error adding car")
        })?
        .id;

        for (index, unload_place) in car_with_places.unload_places.iter().enumerate() {
            sqlx::query!(
                "INSERT INTO car_unload_places (car_id, unload_place_id, order_index, planned_time, notes) 
                 VALUES ($1,$2,$3,$4,$5)",
                car_id,
                unload_place.unload_place_id,
                (index + 1) as i32,
                unload_place.planned_time,
                unload_place.notes
            )
            .execute(&mut *transaction)
            .await
            .map_err(|e| {
                log::error!("Error adding unload place: {}", e);
                error::ErrorInternalServerError("Error adding unload place")
            })?;
        }
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application submitted successfully"
    })))
}

/// Обновление заявки
pub async fn update_application(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    application_data: web::Json<Application>
) -> Result<HttpResponse, Error> {
    let application_id = path.into_inner();

    sqlx::query!(
        "UPDATE applications 
         SET entry_date_from = $1, entry_date_to = $2, entry_time_from = $3, entry_time_to = $4
         WHERE id = $5",
        application_data.entry_date_from,
        application_data.entry_date_to,
        application_data.entry_time_from,
        application_data.entry_time_to,
        application_id
    )
    .execute(pool.get_ref())
    .await
    .map_err(|_| error::ErrorInternalServerError("Failed to update application"))?;

    Ok(HttpResponse::Ok().json("Application updated successfully"))
}
