// handlers/unique_cars.rs
use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};

use crate::models::unique_cars::{UniqueCar, NewUniqueCar, UniqueCarWithRelations, CarOwnerInfo};
use crate::auth::decode_token;

/// Получение информации о владельце для фильтрации машин
async fn get_car_owner_info(
    pool: &web::Data<PgPool>,
    username: &str,
) -> Result<CarOwnerInfo, Error> {
    let user_info = sqlx::query!(
        r#"
        SELECT 
            u.id as user_id,
            u.organization_id,
            u.company_id,
            CASE WHEN o.id IS NOT NULL THEN true ELSE false END as has_organization,
            CASE WHEN c.id IS NOT NULL THEN true ELSE false END as has_company
        FROM users u
        LEFT JOIN organizations o ON u.organization_id = o.id
        LEFT JOIN companies c ON u.company_id = c.id
        WHERE u.username = $1
        "#,
        username
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user info: {}", e);
        error::ErrorInternalServerError("Error fetching user info")
    })?;

    Ok(CarOwnerInfo {
        has_organization: user_info.has_organization.unwrap_or(false),
        has_company: user_info.has_company.unwrap_or(false),
        organization_id: Some(user_info.organization_id),
        company_id: Some(user_info.company_id),
        user_id: user_info.user_id,
    })
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCarByNumberRequest {
    pub number: String,
    pub mark: String,
    pub update_data: NewUniqueCar,
}

/// Получение машин с фильтрацией по владельцу
pub async fn get_unique_cars(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_car_owner_info(&pool, &claims.sub).await?;
                        
                        let filter_type = query.get("filter_type").map(|s| s.as_str()).unwrap_or("user");
                        
                        log::info!("Fetching cars for user: {}, filter_type: {}", owner_info.user_id, filter_type);

                        // Определяем SQL запрос и параметры
                        let (sql, params): (&str, Vec<i32>) = match filter_type {
                            "organization" if owner_info.has_organization => (
                                r#"
                                SELECT 
                                    uc.id,
                                    uc.number,
                                    uc.mark,
                                    uc.organization_id,
                                    uc.company_id,
                                    uc.format_id,
                                    uc.user_id,
                                    uc.status,
                                    uc.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    lpf.name as format_name,
                                    u.username as user_name
                                FROM unique_cars uc
                                LEFT JOIN organizations o ON uc.organization_id = o.id
                                LEFT JOIN companies c ON uc.company_id = c.id
                                LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id
                                LEFT JOIN users u ON uc.user_id = u.id
                                WHERE uc.organization_id = $1
                                ORDER BY uc.number, uc.mark
                                "#,
                                vec![owner_info.organization_id.unwrap_or(0)]
                            ),
                            "company" if owner_info.has_company => (
                                r#"
                                SELECT 
                                    uc.id,
                                    uc.number,
                                    uc.mark,
                                    uc.organization_id,
                                    uc.company_id,
                                    uc.format_id,
                                    uc.user_id,
                                    uc.status,
                                    uc.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    lpf.name as format_name,
                                    u.username as user_name
                                FROM unique_cars uc
                                LEFT JOIN organizations o ON uc.organization_id = o.id
                                LEFT JOIN companies c ON uc.company_id = c.id
                                LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id
                                LEFT JOIN users u ON uc.user_id = u.id
                                WHERE uc.company_id = $1
                                ORDER BY uc.number, uc.mark
                                "#,
                                vec![owner_info.company_id.unwrap_or(0)]
                            ),
                            "all" => (
                                r#"
                                SELECT 
                                    uc.id,
                                    uc.number,
                                    uc.mark,
                                    uc.organization_id,
                                    uc.company_id,
                                    uc.format_id,
                                    uc.user_id,
                                    uc.status,
                                    uc.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    lpf.name as format_name,
                                    u.username as user_name
                                FROM unique_cars uc
                                LEFT JOIN organizations o ON uc.organization_id = o.id
                                LEFT JOIN companies c ON uc.company_id = c.id
                                LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id
                                LEFT JOIN users u ON uc.user_id = u.id
                                WHERE uc.user_id = $1 OR uc.organization_id = $2 OR uc.company_id = $3
                                ORDER BY uc.number, uc.mark
                                "#,
                                vec![
                                    owner_info.user_id,
                                    owner_info.organization_id.unwrap_or(0),
                                    owner_info.company_id.unwrap_or(0)
                                ]
                            ),
                            "all_system" => (
                                r#"
                                SELECT 
                                    uc.id,
                                    uc.number,
                                    uc.mark,
                                    uc.organization_id,
                                    uc.company_id,
                                    uc.format_id,
                                    uc.user_id,
                                    uc.status,
                                    uc.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    lpf.name as format_name,
                                    u.username as user_name
                                FROM unique_cars uc
                                LEFT JOIN organizations o ON uc.organization_id = o.id
                                LEFT JOIN companies c ON uc.company_id = c.id
                                LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id
                                LEFT JOIN users u ON uc.user_id = u.id
                                ORDER BY uc.number, uc.mark
                                "#,
                                vec![] // Пустой вектор параметров для all_system
                            ),
                            _ => (
                                r#"
                                SELECT 
                                    uc.id,
                                    uc.number,
                                    uc.mark,
                                    uc.organization_id,
                                    uc.company_id,
                                    uc.format_id,
                                    uc.user_id,
                                    uc.status,
                                    uc.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    lpf.name as format_name,
                                    u.username as user_name
                                FROM unique_cars uc
                                LEFT JOIN organizations o ON uc.organization_id = o.id
                                LEFT JOIN companies c ON uc.company_id = c.id
                                LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id
                                LEFT JOIN users u ON uc.user_id = u.id
                                WHERE uc.user_id = $1
                                ORDER BY uc.number, uc.mark
                                "#,
                                vec![owner_info.user_id]
                            )
                        };

                        // Выполняем запрос в зависимости от типа фильтра
                        let cars_rows = if filter_type == "all_system" {
                            // Для all_system выполняем запрос без параметров
                            sqlx::query(sql)
                                .fetch_all(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to fetch all system cars: {}", e);
                                    error::ErrorInternalServerError("Error fetching all system cars")
                                })?
                        } else {
                            // Для остальных фильтров выполняем запрос с параметрами
                            let mut query = sqlx::query(sql);
                            
                            // Динамически добавляем параметры
                            for param in params {
                                query = query.bind(param);
                            }
                            
                            query
                                .fetch_all(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to fetch unique cars: {}", e);
                                    error::ErrorInternalServerError("Error fetching cars")
                                })?
                        };

                        log::info!("Fetched {} cars for user {}", cars_rows.len(), owner_info.user_id);

                        let cars: Vec<UniqueCarWithRelations> = cars_rows.into_iter().map(|row| {
                            UniqueCarWithRelations {
                                id: row.get("id"),
                                number: row.get("number"),
                                mark: row.get("mark"),
                                organization_id: row.get("organization_id"),
                                company_id: row.get("company_id"),
                                format_id: row.get("format_id"),
                                user_id: row.get("user_id"),
                                status: row.get::<Option<bool>, _>("status").unwrap_or(false),
                                created_at: row.get("created_at"),
                                organization_name: row.get("organization_name"),
                                company_name: row.get("company_name"),
                                format_name: row.get("format_name"),
                                user_name: row.get("user_name"),
                            }
                        }).collect();

                        Ok(HttpResponse::Ok().json(cars))
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

/// Создание новой уникальной машины
pub async fn create_unique_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<NewUniqueCar>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_car_owner_info(&pool, &claims.sub).await?;

                        log::info!("Creating car for user: {}, data: {:?}", owner_info.user_id, form);

                        // Проверка уникальности номера и марки для пользователя
                        let user_existing_car = sqlx::query!(
                            "SELECT id FROM unique_cars WHERE user_id = $1 AND number = $2 AND mark = $3",
                            owner_info.user_id,
                            form.number,
                            form.mark
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to check car uniqueness for user: {}", e);
                            error::ErrorInternalServerError("Error checking car uniqueness")
                        })?;

                        if user_existing_car.is_some() {
                            return Err(error::ErrorBadRequest("Автомобиль уже привязан к вашему аккаунту"));
                        }

                        // Проверка уникальности номера и марки для организации
                        if let Some(org_id) = form.organization_id {
                            let existing_car = sqlx::query!(
                                "SELECT id FROM unique_cars WHERE organization_id = $1 AND number = $2 AND mark = $3",
                                org_id,
                                form.number,
                                form.mark
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|e| {
                                log::error!("Failed to check car uniqueness for organization: {}", e);
                                error::ErrorInternalServerError("Error checking car uniqueness")
                            })?;

                            if existing_car.is_some() {
                                return Err(error::ErrorBadRequest("Автомобиль с этим номером и маркой уже существует в этой организации"));
                            }
                        }

                        // Проверка уникальности номера и марки для компании
                        if let Some(company_id) = form.company_id {
                            let existing_car = sqlx::query!(
                                "SELECT id FROM unique_cars WHERE company_id = $1 AND number = $2 AND mark = $3",
                                company_id,
                                form.number,
                                form.mark
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|e| {
                                log::error!("Failed to check car uniqueness for company: {}", e);
                                error::ErrorInternalServerError("Error checking car uniqueness")
                            })?;

                            if existing_car.is_some() {
                                return Err(error::ErrorBadRequest("Автомобиль с этим номером и маркой уже существует в этой компании"));
                            }
                        }

                        // Убедимся, что user_id установлен
                        let user_id = form.user_id.unwrap_or(owner_info.user_id);

                        // Создаем машину
                        let car_result = sqlx::query!(
                            r#"
                            INSERT INTO unique_cars (number, mark, organization_id, company_id, format_id, user_id, status)
                            VALUES ($1, $2, $3, $4, $5, $6, false)
                            RETURNING id, number, mark, organization_id, company_id, format_id, user_id, status, created_at
                            "#,
                            form.number,
                            form.mark,
                            form.organization_id,
                            form.company_id,
                            form.format_id,
                            user_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create unique car: {}", e);
                            error::ErrorInternalServerError("Error creating car")
                        })?;

                        let car = UniqueCar {
                            id: car_result.id,
                            number: car_result.number,
                            mark: car_result.mark,
                            organization_id: car_result.organization_id,
                            company_id: car_result.company_id,
                            format_id: car_result.format_id,
                            user_id: car_result.user_id,
                            status: car_result.status.unwrap_or(false),
                            created_at: car_result.created_at,
                        };

                        log::info!("Successfully created car: {:?}", car);

                        Ok(HttpResponse::Ok().json(car))
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

/// Создание нескольких уникальных машин
pub async fn create_unique_cars_batch(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    forms: web::Json<Vec<NewUniqueCar>>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_car_owner_info(&pool, &claims.sub).await?;

                        log::info!("Creating {} cars for user: {}", forms.len(), owner_info.user_id);

                        let mut created_cars = Vec::new();
                        let mut errors = Vec::new();

                        for form in forms.iter() {
                            // Проверка уникальности для каждой машины
                            let user_existing_car = sqlx::query!(
                                "SELECT id FROM unique_cars WHERE user_id = $1 AND number = $2 AND mark = $3",
                                owner_info.user_id,
                                form.number,
                                form.mark
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|e| {
                                log::error!("Failed to check car uniqueness for user: {}", e);
                                error::ErrorInternalServerError("Error checking car uniqueness")
                            })?;

                            if user_existing_car.is_some() {
                                errors.push(format!("Автомобиль {} {} уже привязан к вашему аккаунту", form.number, form.mark));
                                continue;
                            }

                            // Проверка для организации
                            if let Some(org_id) = form.organization_id {
                                let existing_car = sqlx::query!(
                                    "SELECT id FROM unique_cars WHERE organization_id = $1 AND number = $2 AND mark = $3",
                                    org_id,
                                    form.number,
                                    form.mark
                                )
                                .fetch_optional(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to check car uniqueness for organization: {}", e);
                                    error::ErrorInternalServerError("Error checking car uniqueness")
                                })?;

                                if existing_car.is_some() {
                                    errors.push(format!("Автомобиль {} {} уже существует в этой организации", form.number, form.mark));
                                    continue;
                                }
                            }

                            // Проверка для компании
                            if let Some(company_id) = form.company_id {
                                let existing_car = sqlx::query!(
                                    "SELECT id FROM unique_cars WHERE company_id = $1 AND number = $2 AND mark = $3",
                                    company_id,
                                    form.number,
                                    form.mark
                                )
                                .fetch_optional(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to check car uniqueness for company: {}", e);
                                    error::ErrorInternalServerError("Error checking car uniqueness")
                                })?;

                                if existing_car.is_some() {
                                    errors.push(format!("Автомобиль {} {} уже существует в этой компании", form.number, form.mark));
                                    continue;
                                }
                            }

                            // Убедимся, что user_id установлен
                            let user_id = form.user_id.unwrap_or(owner_info.user_id);

                            // Создаем машину
                            match sqlx::query!(
                                r#"
                                INSERT INTO unique_cars (number, mark, organization_id, company_id, format_id, user_id, status)
                                VALUES ($1, $2, $3, $4, $5, $6, false)
                                RETURNING id, number, mark, organization_id, company_id, format_id, user_id, status, created_at
                                "#,
                                form.number,
                                form.mark,
                                form.organization_id,
                                form.company_id,
                                form.format_id,
                                user_id
                            )
                            .fetch_one(pool.get_ref())
                            .await
                            {
                                Ok(car_result) => {
                                    let car = UniqueCar {
                                        id: car_result.id,
                                        number: car_result.number,
                                        mark: car_result.mark,
                                        organization_id: car_result.organization_id,
                                        company_id: car_result.company_id,
                                        format_id: car_result.format_id,
                                        user_id: car_result.user_id,
                                        status: car_result.status.unwrap_or(false),
                                        created_at: car_result.created_at,
                                    };
                                    created_cars.push(car);
                                }
                                Err(e) => {
                                    errors.push(format!("Ошибка при создании автомобиля {} {}: {}", form.number, form.mark, e));
                                }
                            }
                        }

                        let response = json!({
                            "created_cars": created_cars,
                            "errors": errors,
                            "success_count": created_cars.len(),
                            "error_count": errors.len()
                        });

                        if !errors.is_empty() {
                            log::warn!("Created {} cars with {} errors", created_cars.len(), errors.len());
                            Ok(HttpResponse::MultiStatus().json(response))
                        } else {
                            log::info!("Successfully created {} cars", created_cars.len());
                            Ok(HttpResponse::Ok().json(response))
                        }
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

/// Обновление существующей машины
pub async fn update_unique_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<NewUniqueCar>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let car_id = path.into_inner();
                        let owner_info = get_car_owner_info(&pool, &claims.sub).await?;

                        log::info!("Updating car {} for user: {}, data: {:?}", car_id, owner_info.user_id, form);

                        // Проверяем, существует ли машина и принадлежит ли пользователю
                        let existing_car = sqlx::query!(
                            "SELECT user_id, organization_id, company_id FROM unique_cars WHERE id = $1",
                            car_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch car: {}", e);
                            error::ErrorInternalServerError("Error fetching car")
                        })?;

                        if existing_car.is_none() {
                            return Err(error::ErrorNotFound("Car not found"));
                        }

                        let car = existing_car.unwrap();

                        // Проверяем права на редактирование
                        let can_edit = car.user_id == Some(owner_info.user_id) ||
                                      (car.organization_id.is_some() && car.organization_id == owner_info.organization_id) ||
                                      (car.company_id.is_some() && car.company_id == owner_info.company_id);

                        if !can_edit {
                            return Err(error::ErrorForbidden("You don't have permission to edit this car"));
                        }

                        // Проверка уникальности номера и марки для пользователя (исключая текущую машину)
                        let user_existing_car = sqlx::query!(
                            "SELECT id FROM unique_cars WHERE user_id = $1 AND number = $2 AND mark = $3 AND id != $4",
                            owner_info.user_id,
                            form.number,
                            form.mark,
                            car_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to check car uniqueness for user: {}", e);
                            error::ErrorInternalServerError("Error checking car uniqueness")
                        })?;

                        if user_existing_car.is_some() {
                            return Err(error::ErrorBadRequest("Автомобиль уже привязан к вашему аккаунту"));
                        }

                        // Проверка уникальности номера и марки для организации (исключая текущую машину)
                        if let Some(org_id) = form.organization_id {
                            let existing_car = sqlx::query!(
                                "SELECT id FROM unique_cars WHERE organization_id = $1 AND number = $2 AND mark = $3 AND id != $4",
                                org_id,
                                form.number,
                                form.mark,
                                car_id
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|e| {
                                log::error!("Failed to check car uniqueness for organization: {}", e);
                                error::ErrorInternalServerError("Error checking car uniqueness")
                            })?;

                            if existing_car.is_some() {
                                return Err(error::ErrorBadRequest("Автомобиль с этим номером и маркой уже существует в этой организации"));
                            }
                        }

                        // Проверка уникальности номера и марки для компании (исключая текущую машину)
                        if let Some(company_id) = form.company_id {
                            let existing_car = sqlx::query!(
                                "SELECT id FROM unique_cars WHERE company_id = $1 AND number = $2 AND mark = $3 AND id != $4",
                                company_id,
                                form.number,
                                form.mark,
                                car_id
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|e| {
                                log::error!("Failed to check car uniqueness for company: {}", e);
                                error::ErrorInternalServerError("Error checking car uniqueness")
                            })?;

                            if existing_car.is_some() {
                                return Err(error::ErrorBadRequest("Автомобиль с этим номером и маркой уже существует в этой компании"));
                            }
                        }

                        // Убедимся, что user_id установлен
                        let user_id = form.user_id.unwrap_or(owner_info.user_id);

                        // Обновляем машину
                        let car_result = sqlx::query!(
                            r#"
                            UPDATE unique_cars 
                            SET number = $1, mark = $2, organization_id = $3, company_id = $4, format_id = $5, user_id = $6
                            WHERE id = $7
                            RETURNING id, number, mark, organization_id, company_id, format_id, user_id, status, created_at
                            "#,
                            form.number,
                            form.mark,
                            form.organization_id,
                            form.company_id,
                            form.format_id,
                            user_id,
                            car_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to update unique car: {}", e);
                            error::ErrorInternalServerError("Error updating car")
                        })?;

                        let updated_car = UniqueCar {
                            id: car_result.id,
                            number: car_result.number,
                            mark: car_result.mark,
                            organization_id: car_result.organization_id,
                            company_id: car_result.company_id,
                            format_id: car_result.format_id,
                            user_id: car_result.user_id,
                            status: car_result.status.unwrap_or(false),
                            created_at: car_result.created_at,
                        };

                        log::info!("Successfully updated car: {:?}", updated_car);

                        Ok(HttpResponse::Ok().json(updated_car))
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

/// Обновление машины по номеру и марке
pub async fn update_unique_car_by_number(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<UpdateCarByNumberRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_car_owner_info(&pool, &claims.sub).await?;

                        log::info!("Updating car by number: {} for user: {}", form.number, owner_info.user_id);

                        // Находим машину по номеру и марке
                        let existing_car = sqlx::query!(
                            "SELECT id, user_id, organization_id, company_id FROM unique_cars WHERE number = $1 AND mark = $2",
                            form.number,
                            form.mark
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch car: {}", e);
                            error::ErrorInternalServerError("Error fetching car")
                        })?;

                        if existing_car.is_none() {
                            return Err(error::ErrorNotFound("Car not found"));
                        }

                        let car = existing_car.unwrap();

                        // Проверяем права на редактирование
                        let can_edit = car.user_id == Some(owner_info.user_id) ||
                                      (car.organization_id.is_some() && car.organization_id == owner_info.organization_id) ||
                                      (car.company_id.is_some() && car.company_id == owner_info.company_id);

                        if !can_edit {
                            return Err(error::ErrorForbidden("You don't have permission to edit this car"));
                        }

                        // Обновляем машину
                        let car_result = sqlx::query!(
                            r#"
                            UPDATE unique_cars 
                            SET number = $1, mark = $2, organization_id = $3, company_id = $4, format_id = $5, user_id = $6
                            WHERE id = $7
                            RETURNING id, number, mark, organization_id, company_id, format_id, user_id, status, created_at
                            "#,
                            form.update_data.number,
                            form.update_data.mark,
                            form.update_data.organization_id,
                            form.update_data.company_id,
                            form.update_data.format_id,
                            form.update_data.user_id.unwrap_or(owner_info.user_id),
                            car.id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to update unique car: {}", e);
                            error::ErrorInternalServerError("Error updating car")
                        })?;

                        let updated_car = UniqueCar {
                            id: car_result.id,
                            number: car_result.number,
                            mark: car_result.mark,
                            organization_id: car_result.organization_id,
                            company_id: car_result.company_id,
                            format_id: car_result.format_id,
                            user_id: car_result.user_id,
                            status: car_result.status.unwrap_or(false),
                            created_at: car_result.created_at,
                        };

                        log::info!("Successfully updated car: {:?}", updated_car);

                        Ok(HttpResponse::Ok().json(updated_car))
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

/// Удаление машины
pub async fn delete_unique_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let car_id = path.into_inner();
                        let owner_info = get_car_owner_info(&pool, &claims.sub).await?;

                        // Проверяем права на удаление
                        let car = sqlx::query!(
                            "SELECT user_id, organization_id, company_id FROM unique_cars WHERE id = $1",
                            car_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch car: {}", e);
                            error::ErrorInternalServerError("Error fetching car")
                        })?;

                        if car.is_none() {
                            return Err(error::ErrorNotFound("Car not found"));
                        }

                        let car_data = car.unwrap();
                        let can_delete = car_data.user_id == Some(owner_info.user_id) ||
                                       (car_data.organization_id.is_some() && car_data.organization_id == owner_info.organization_id) ||
                                       (car_data.company_id.is_some() && car_data.company_id == owner_info.company_id);

                        if !can_delete {
                            return Err(error::ErrorForbidden("You don't have permission to delete this car"));
                        }

                        let result = sqlx::query!(
                            "DELETE FROM unique_cars WHERE id = $1",
                            car_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete car: {}", e);
                            error::ErrorInternalServerError("Error deleting car")
                        })?;

                        if result.rows_affected() > 0 {
                            Ok(HttpResponse::Ok().json(json!({"message": "Car deleted successfully"})))
                        } else {
                            Err(error::ErrorNotFound("Car not found"))
                        }
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

/// Получение информации о владельце для интерфейса
pub async fn get_car_ownership_info(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_car_owner_info(&pool, &claims.sub).await?;
                        Ok(HttpResponse::Ok().json(owner_info))
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