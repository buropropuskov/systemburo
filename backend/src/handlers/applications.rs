use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;
use serde::Deserialize;

use crate::models::applications::{Application, ApplicationWithCars, ApplicationSubmitRequest};
use crate::models::cars::{CarWithUnloadPlaces, Car};
use crate::models::unload_places::CarUnloadPlace;
use crate::auth::decode_token;

/// Получение информации о пользователе
async fn get_user_info(
    pool: &web::Data<PgPool>,
    username: &str,
) -> Result<UserInfo, Error> {
    let user_info = sqlx::query!(
        r#"
        SELECT id as user_id, organization_id, company_id
        FROM users 
        WHERE username = $1
        "#,
        username
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user info: {}", e);
        error::ErrorInternalServerError("Error fetching user info")
    })?;

    Ok(UserInfo {
        user_id: user_info.user_id,
        organization_id: Some(user_info.organization_id),
        company_id: Some(user_info.company_id),
    })
}

#[derive(Debug)]
struct UserInfo {
    user_id: i32,
    organization_id: Option<i32>,
    company_id: Option<i32>,
}

/// Структуры для заявок сотрудников
#[derive(Debug, Deserialize)]
pub struct EmployeeApplicationData {
    pub message: Option<String>,
    pub application: ApplicationData,
    pub employees: Vec<EmployeeApplication>,
}

#[derive(Debug, Deserialize)]
pub struct EmployeeApplication {
    pub employee: EmployeeData,
    pub target_tables: Vec<TargetTableData>,
}

#[derive(Debug, Deserialize)]
pub struct EmployeeData {
    pub last_name: String,
    pub first_name: String,
    pub middle_name: Option<String>,
    pub position: String,
    pub citizenship_id: i32,
    pub passport_series_number: String,
    pub patent_number: Option<String>,
    pub other_permission: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct TargetTableData {
    pub table_id: i32,
    pub order_index: i32,
}

use chrono::{NaiveDate, NaiveTime};

#[derive(Debug, Deserialize)]
pub struct ApplicationData {
    pub organization: String,
    pub responsible_person: String,
    pub contact_phone: String,
    pub entry_date_from: NaiveDate,
    pub entry_date_to: NaiveDate,
    pub entry_time_from: NaiveTime,
    pub entry_time_to: NaiveTime,
}

/// Отправка заявки для сотрудников
pub async fn submit_employee_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<EmployeeApplicationData>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user_info = get_user_info(&pool, &claims.sub).await?;

                        log::info!("Submitting employee application for user: {}", user_info.user_id);

                        // Начинаем транзакцию
                        let mut transaction = pool.begin().await.map_err(|e| {
                            log::error!("Failed to begin transaction: {}", e);
                            error::ErrorInternalServerError("Database error")
                        })?;

                        // Создаем заявку
                        let application_result = sqlx::query!(
                            r#"
                            INSERT INTO applications (
                                organization, responsible_person, contact_phone,
                                entry_date_from, entry_date_to, entry_time_from, entry_time_to,
                                message, user_id, application_type
                            )
                            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'employee')
                            RETURNING id
                            "#,
                            form.application.organization,
                            form.application.responsible_person,
                            form.application.contact_phone,
                            form.application.entry_date_from,
                            form.application.entry_date_to,
                            form.application.entry_time_from,
                            form.application.entry_time_to,
                            form.message,
                            user_info.user_id
                        )
                        .fetch_one(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create application: {}", e);
                            error::ErrorInternalServerError("Error creating application")
                        })?;

                        let application_id = application_result.id;

                        // Добавляем сотрудников в заявку
                        for (index, employee_app) in form.employees.iter().enumerate() {
                            let employee_result = sqlx::query!(
                                r#"
                                INSERT INTO application_employees (
                                    application_id, last_name, first_name, middle_name, position,
                                    citizenship_id, passport_series_number, patent_number, other_permission,
                                    order_index
                                )
                                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
                                RETURNING id
                                "#,
                                application_id,
                                employee_app.employee.last_name,
                                employee_app.employee.first_name,
                                employee_app.employee.middle_name,
                                employee_app.employee.position,
                                employee_app.employee.citizenship_id,
                                employee_app.employee.passport_series_number,
                                employee_app.employee.patent_number,
                                employee_app.employee.other_permission,
                                index as i32
                            )
                            .fetch_one(&mut *transaction)
                            .await
                            .map_err(|e| {
                                log::error!("Failed to create application employee: {}", e);
                                error::ErrorInternalServerError("Error creating application employee")
                            })?;

                            let application_employee_id = employee_result.id;

                            // Добавляем целевые таблицы для сотрудника
                            for target_table in &employee_app.target_tables {
                                sqlx::query!(
                                    r#"
                                    INSERT INTO employee_target_tables (
                                        application_employee_id, table_id, order_index
                                    )
                                    VALUES ($1, $2, $3)
                                    "#,
                                    application_employee_id,
                                    target_table.table_id,
                                    target_table.order_index
                                )
                                .execute(&mut *transaction)
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to create employee target table: {}", e);
                                    error::ErrorInternalServerError("Error creating employee target table")
                                })?;
                            }
                        }

                        // Коммитим транзакцию
                        transaction.commit().await.map_err(|e| {
                            log::error!("Failed to commit transaction: {}", e);
                            error::ErrorInternalServerError("Database error")
                        })?;

                        log::info!("Successfully submitted employee application with ID: {}", application_id);

                        Ok(HttpResponse::Ok().json(json!({
                            "message": "Employee application submitted successfully",
                            "application_id": application_id
                        })))
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

    let user_info = get_user_info(&pool, &claims.sub).await?;

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
         entry_date_from, entry_date_to, entry_time_from, entry_time_to, message, user_id, application_type) 
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'vehicle') RETURNING id",
        form.application.organization.as_ref().unwrap(),
        form.application.responsible_person.as_ref().unwrap(),
        form.application.contact_phone.as_ref().unwrap(),
        form.application.entry_date_from.as_ref().unwrap(),
        form.application.entry_date_to.as_ref().unwrap(),
        form.application.entry_time_from.as_ref().unwrap(),
        form.application.entry_time_to.as_ref().unwrap(),
        form.message,
        user_info.user_id
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
            form.application.entry_date_from.as_ref().unwrap(),
            form.application.entry_date_to.as_ref().unwrap(),
            form.application.entry_time_from.as_ref().unwrap(),
            form.application.entry_time_to.as_ref().unwrap()
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

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let user_info = get_user_info(&pool, &claims.sub).await?;

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
         entry_date_from, entry_date_to, entry_time_from, entry_time_to, user_id, application_type) 
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'vehicle') RETURNING id",
        form.application.organization.as_ref().unwrap(),
        form.application.responsible_person.as_ref().unwrap(),
        form.application.contact_phone.as_ref().unwrap(),
        form.application.entry_date_from.as_ref().unwrap(),
        form.application.entry_date_to.as_ref().unwrap(),
        form.application.entry_time_from.as_ref().unwrap(),
        form.application.entry_time_to.as_ref().unwrap(),
        user_info.user_id
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
            form.application.entry_date_from.as_ref().unwrap(),
            form.application.entry_date_to.as_ref().unwrap(),
            form.application.entry_time_from.as_ref().unwrap(),
            form.application.entry_time_to.as_ref().unwrap()
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


