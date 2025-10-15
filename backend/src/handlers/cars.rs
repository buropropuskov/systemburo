use actix_web::{web, HttpResponse, Error, error, Responder};
use sqlx::PgPool;
use chrono::{Utc, NaiveDate};
use log;

use crate::models::cars::{CarWithUnloadPlaces, Car};
use crate::models::unload_places::CarUnloadPlace;
use crate::models::applications::ApplicationWithCars;


/// Получение всех заявок с машинами
pub async fn get_all_cars_for_account(pool: web::Data<PgPool>) -> impl Responder {
    let applications_result = sqlx::query!(
        r#"SELECT id, organization, responsible_person, contact_phone, 
                  entry_date_from, entry_date_to, entry_time_from, entry_time_to, submission_datetime
           FROM applications"#
    )
    .fetch_all(pool.get_ref())
    .await;

    match applications_result {
        Ok(application_data) => {
            let mut applications_with_cars = Vec::new();

            for app in application_data {
                let cars = sqlx::query!(
                    r#"SELECT id, application_id, car_number, car_brand, 
                              entry_date_from, entry_date_to, entry_time_from, entry_time_to, 
                              status, date_added, date_removed
                       FROM cars WHERE application_id = $1"#,
                    app.id
                )
                .fetch_all(pool.get_ref())
                .await
                .unwrap_or_else(|_| vec![]);

                let mut cars_with_places = Vec::new();
                for car in cars {
                    let unload_places = sqlx::query!(
                        r#"SELECT cup.id, cup.car_id, cup.unload_place_id, cup.order_index, 
                                  cup.planned_time, cup.notes, up.name as unload_place_name
                           FROM car_unload_places cup
                           JOIN unload_places up ON cup.unload_place_id = up.id
                           WHERE cup.car_id = $1
                           ORDER BY cup.order_index"#,
                        car.id
                    )
                    .fetch_all(pool.get_ref())
                    .await
                    .unwrap_or_else(|_| vec![]);

                    let car_model = Car {
                        id: Some(car.id),
                        application_id: Some(car.application_id),
                        car_number: Some(car.car_number),
                        car_brand: Some(car.car_brand),
                        unload_place: None,
                        entry_date_from: Some(car.entry_date_from),
                        entry_date_to: Some(car.entry_date_to),
                        entry_time_from: Some(car.entry_time_from),
                        entry_time_to: Some(car.entry_time_to),
                        status: car.status,
                        date_added: car.date_added,
                        date_removed: car.date_removed,
                        unload_places: Some(unload_places.into_iter().map(|up| CarUnloadPlace {
                            id: Some(up.id),
                            car_id: Some(up.car_id),
                            unload_place_id: up.unload_place_id,
                            unload_place_name: Some(up.unload_place_name),
                            order_index: up.order_index,
                            planned_time: up.planned_time,
                            notes: up.notes,
                        }).collect()),
                    };
                    cars_with_places.push(car_model);
                }

                applications_with_cars.push(ApplicationWithCars {
                    id: Some(app.id),
                    organization: Some(app.organization),
                    responsible_person: Some(app.responsible_person),
                    contact_phone: Some(app.contact_phone),
                    entry_date_from: Some(app.entry_date_from),
                    entry_date_to: Some(app.entry_date_to),
                    entry_time_from: Some(app.entry_time_from),
                    entry_time_to: Some(app.entry_time_to),
                    submission_datetime: app.submission_datetime,
                    cars: cars_with_places,
                });
            }

            HttpResponse::Ok().json(applications_with_cars)
        },
        Err(e) => {
            log::error!("Error fetching all cars: {}", e);
            HttpResponse::InternalServerError().json("Error fetching all cars for account")
        },
    }
}

/// Получение активных машин для таблицы
pub async fn get_active_cars_for_table(pool: web::Data<PgPool>) -> impl Responder {
    let applications_result = sqlx::query!(
        r#"SELECT id, organization, responsible_person, contact_phone, 
                  entry_date_from, entry_date_to, entry_time_from, entry_time_to, submission_datetime 
           FROM applications"#
    )
    .fetch_all(pool.get_ref())
    .await;

    match applications_result {
        Ok(application_data) => {
            let mut applications_with_cars = Vec::new();

            for app in application_data {
                let cars = sqlx::query!(
                    r#"SELECT id, application_id, car_number, car_brand, 
                              entry_date_from, entry_date_to, entry_time_from, entry_time_to, 
                              status, date_added, date_removed
                       FROM cars WHERE application_id = $1 AND status = 1"#,
                    app.id
                )
                .fetch_all(pool.get_ref())
                .await
                .unwrap_or_else(|_| vec![]);

                let mut cars_with_places = Vec::new();
                for car in cars {
                    let unload_places = sqlx::query!(
                        r#"SELECT cup.id, cup.car_id, cup.unload_place_id, cup.order_index, 
                                  cup.planned_time, cup.notes, up.name as unload_place_name
                           FROM car_unload_places cup
                           JOIN unload_places up ON cup.unload_place_id = up.id
                           WHERE cup.car_id = $1
                           ORDER BY cup.order_index"#,
                        car.id
                    )
                    .fetch_all(pool.get_ref())
                    .await
                    .unwrap_or_else(|_| vec![]);

                    let car_model = Car {
                        id: Some(car.id),
                        application_id: Some(car.application_id),
                        car_number: Some(car.car_number),
                        car_brand: Some(car.car_brand),
                        unload_place: None,
                        entry_date_from: Some(car.entry_date_from),
                        entry_date_to: Some(car.entry_date_to),
                        entry_time_from: Some(car.entry_time_from),
                        entry_time_to: Some(car.entry_time_to),
                        status: car.status,
                        date_added: car.date_added,
                        date_removed: car.date_removed,
                        unload_places: Some(unload_places.into_iter().map(|up| CarUnloadPlace {
                            id: Some(up.id),
                            car_id: Some(up.car_id),
                            unload_place_id: up.unload_place_id,
                            unload_place_name: Some(up.unload_place_name),
                            order_index: up.order_index,
                            planned_time: up.planned_time,
                            notes: up.notes,
                        }).collect()),
                    };
                    cars_with_places.push(car_model);
                }

                applications_with_cars.push(ApplicationWithCars {
                    id: Some(app.id),
                    organization: Some(app.organization),
                    responsible_person: Some(app.responsible_person),
                    contact_phone: Some(app.contact_phone),
                    entry_date_from: Some(app.entry_date_from),
                    entry_date_to: Some(app.entry_date_to),
                    entry_time_from: Some(app.entry_time_from),
                    entry_time_to: Some(app.entry_time_to),
                    submission_datetime: app.submission_datetime,
                    cars: cars_with_places,
                });
            }

            HttpResponse::Ok().json(applications_with_cars)
        },
        Err(e) => {
            log::error!("Error fetching active cars: {}", e);
            HttpResponse::InternalServerError().json("Error fetching active cars for table")
        },
    }
}

/// Обновление данных машины
pub async fn update_car(
    pool: web::Data<PgPool>,
    path: web::Path<(i32, i32)>,
    car_data: web::Json<CarWithUnloadPlaces>
) -> Result<HttpResponse, Error> {
    let (application_id, car_id) = path.into_inner();
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    let date_removed: Option<NaiveDate> = if car_data.car.status == Some(0) {
        Some(Utc::now().date_naive())
    } else {
        None
    };

    sqlx::query!(
        "UPDATE cars 
         SET car_number = $1, 
             car_brand = $2, 
             entry_date_from = $3,
             entry_date_to = $4,
             entry_time_from = $5,
             entry_time_to = $6,
             status = $7, 
             date_removed = $8
         WHERE application_id = $9 AND id = $10",
        car_data.car.car_number,
        car_data.car.car_brand,
        car_data.car.entry_date_from,
        car_data.car.entry_date_to,
        car_data.car.entry_time_from,
        car_data.car.entry_time_to,
        car_data.car.status,
        date_removed,
        application_id,
        car_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|_| error::ErrorInternalServerError("Error updating car"))?;

    sqlx::query!("DELETE FROM car_unload_places WHERE car_id = $1", car_id)
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Error deleting old unload places: {}", e);
            error::ErrorInternalServerError("Error updating unload places")
        })?;

    for (index, unload_place) in car_data.unload_places.iter().enumerate() {
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
            error::ErrorInternalServerError("Error updating unload places")
        })?;
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json("Car updated successfully"))
}

/// Удаление машины (soft delete)
pub async fn delete_car(pool: web::Data<PgPool>, path: web::Path<(i32, i32)>) -> Result<HttpResponse, Error> {
    let (application_id, car_id) = path.into_inner();

    sqlx::query!(
        "UPDATE cars SET status = 0, date_removed = CURRENT_DATE WHERE application_id = $1 AND id = $2",
        application_id,
        car_id
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Error updating car status: {}", e);
        error::ErrorInternalServerError("Error updating car status")
    })?;

    Ok(HttpResponse::Ok().json("Car status updated to removed"))
}
