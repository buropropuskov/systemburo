use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool};
use serde_json::json;
use log;
use chrono::{DateTime, Utc, NaiveDateTime};
use serde::{Serialize, Deserialize};

use crate::auth::decode_token;

#[derive(Debug, Serialize, Deserialize)]
pub struct CarHistoryItem {
    pub id: i32,
    pub car_id: i32,
    pub user_id: i32,
    pub user_name: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub action_type: String,
    pub field_name: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub created_at: NaiveDateTime,
    pub metadata: Option<serde_json::Value>,
    pub car_number: Option<String>,
    pub car_brand: Option<String>,
    pub organization: Option<String>,
    pub company: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct AddCarHistoryRequest {
    pub user_id: i32,
    pub action_type: String,
    pub field_name: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateTerritoryStatusRequest {
    pub territory_status: i32,
    pub user_id: i32,
}

#[derive(Debug, Deserialize)]
pub struct DeactivateCarRequest {
    pub status: i32,
    pub user_id: i32,
}

#[derive(Debug, Deserialize)]
pub struct ActivateCarRequest {
    pub user_id: i32,
}

#[derive(Debug, Deserialize)]
pub struct RestoreCarRequest {
    pub user_id: i32,
}

#[derive(Debug, Deserialize)]
pub struct UnifiedCarHistoryQuery {
    pub car_number: String,
    pub car_brand: String,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
}

/// Получение объединённой истории для всех машин с одинаковыми параметрами
pub async fn get_unified_car_history(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<UnifiedCarHistoryQuery>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Getting unified car history for: number={}, brand={}, org_id={:?}, company_id={:?}", 
               query.car_number, query.car_brand, query.organization_id, query.company_id);

    // Находим все машины с такими же параметрами
    let cars = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.car_number,
            c.car_brand
        FROM cars c
        JOIN attachments a ON c.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        WHERE LOWER(TRIM(c.car_number)) = LOWER(TRIM($1))
        AND LOWER(TRIM(c.car_brand)) = LOWER(TRIM($2))
        AND (
            ($3::integer IS NULL AND app.organization_id IS NULL)
            OR app.organization_id = $3
        )
        AND (
            ($4::integer IS NULL AND app.company_id IS NULL)
            OR app.company_id = $4
        )
        ORDER BY c.id
        "#,
        query.car_number,
        query.car_brand,
        query.organization_id,
        query.company_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch cars: {}", e);
        error::ErrorInternalServerError("Error fetching cars")
    })?;

    if cars.is_empty() {
        return Ok(HttpResponse::Ok().json(Vec::<serde_json::Value>::new()));
    }

    let car_ids: Vec<i32> = cars.iter().map(|car| car.id).collect();

    // Получаем всю историю для всех найденных машин
    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.car_id,
            h.user_id,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || u.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || u.middle_name
                    ELSE ''
                END
            ) as "user_name!",
            u.last_name,
            u.first_name,
            u.middle_name,
            h.action_type as "action_type!",
            h.field_name,
            h.old_value,
            h.new_value,
            h.comment,
            h.created_at as "created_at!",
            h.metadata as "metadata?",
            c.car_number,
            c.car_brand,
            COALESCE(o.name, '') as organization,
            COALESCE(c2.name, '') as company
        FROM cars_history h
        JOIN users u ON h.user_id = u.id
        JOIN cars c ON h.car_id = c.id
        LEFT JOIN attachments a ON c.attachment_id = a.id
        LEFT JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c2 ON app.company_id = c2.id
        WHERE h.car_id = ANY($1)
        ORDER BY h.created_at DESC
        "#,
        &car_ids[..]
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch unified car history: {}", e);
        error::ErrorInternalServerError("Error fetching unified car history")
    })?;

    let items: Vec<CarHistoryItem> = history.into_iter().map(|row| {
        CarHistoryItem {
            id: row.id,
            car_id: row.car_id,
            user_id: row.user_id,
            user_name: row.user_name,
            last_name: row.last_name,
            first_name: row.first_name,
            middle_name: row.middle_name,
            action_type: row.action_type,
            field_name: row.field_name,
            old_value: row.old_value,
            new_value: row.new_value,
            comment: row.comment,
            created_at: row.created_at,
            metadata: row.metadata,
            car_number: Some(row.car_number),
            car_brand: Some(row.car_brand),
            organization: row.organization,
            company: row.company,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Получение истории конкретного автомобиля (оригинальный метод)
pub async fn get_car_history(
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

    let car_id = path.into_inner();

    log::info!("Getting history for car: {}", car_id);

    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.car_id,
            h.user_id,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || u.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || u.middle_name
                    ELSE ''
                END
            ) as "user_name!",
            u.last_name,
            u.first_name,
            u.middle_name,
            h.action_type as "action_type!",
            h.field_name,
            h.old_value,
            h.new_value,
            h.comment,
            h.created_at as "created_at!",
            h.metadata as "metadata?"
        FROM cars_history h
        JOIN users u ON h.user_id = u.id
        WHERE h.car_id = $1
        ORDER BY h.created_at DESC
        "#,
        car_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch car history: {}", e);
        error::ErrorInternalServerError("Error fetching car history")
    })?;

    let items: Vec<CarHistoryItem> = history.into_iter().map(|row| {
        CarHistoryItem {
            id: row.id,
            car_id: row.car_id,
            user_id: row.user_id,
            user_name: row.user_name,
            last_name: row.last_name,
            first_name: row.first_name,
            middle_name: row.middle_name,
            action_type: row.action_type,
            field_name: row.field_name,
            old_value: row.old_value,
            new_value: row.new_value,
            comment: row.comment,
            created_at: row.created_at,
            metadata: row.metadata,
            car_number: None,
            car_brand: None,
            organization: None,
            company: None,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Добавление записи в историю автомобиля
pub async fn add_car_history_entry(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<AddCarHistoryRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let car_id = path.into_inner();

    log::info!("Adding history entry for car {} by user {}", car_id, form.user_id);

    sqlx::query!(
        r#"
        INSERT INTO cars_history (
            car_id,
            user_id,
            action_type,
            field_name,
            old_value,
            new_value,
            comment,
            metadata,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
        "#,
        car_id,
        form.user_id,
        form.action_type,
        form.field_name,
        form.old_value,
        form.new_value,
        form.comment,
        form.metadata
    )
    .execute(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to add car history entry: {}", e);
        error::ErrorInternalServerError("Error adding car history entry")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Car history entry added successfully"
    })))
}

/// Получение истории всех автомобилей (для общей таблицы)
pub async fn get_all_cars_history(
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

    log::info!("Getting all cars history");

    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.car_id,
            h.user_id,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || u.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || u.middle_name
                    ELSE ''
                END
            ) as "user_name!",
            h.action_type as "action_type!",
            h.comment,
            h.created_at as "created_at!",
            c.car_number,
            c.car_brand,
            COALESCE(o.name, '') as organization,
            COALESCE(c2.name, '') as company
        FROM cars_history h
        JOIN users u ON h.user_id = u.id
        JOIN cars c ON h.car_id = c.id
        LEFT JOIN attachments a ON c.attachment_id = a.id
        LEFT JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c2 ON app.company_id = c2.id
        WHERE h.action_type IN ('entry', 'exit')
        ORDER BY h.created_at DESC
        "#,
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch all cars history: {}", e);
        error::ErrorInternalServerError("Error fetching all cars history")
    })?;

    let items: Vec<serde_json::Value> = history.into_iter().map(|row| {
        json!({
            "id": row.id,
            "car_id": row.car_id,
            "user_id": row.user_id,
            "user_name": row.user_name,
            "action_type": row.action_type,
            "comment": row.comment,
            "created_at": row.created_at,
            "car_number": row.car_number,
            "car_brand": row.car_brand,
            "organization": row.organization,
            "company": row.company
        })
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Получение текущего статуса всех машин (на территории или нет)
pub async fn get_cars_current_status(
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

    log::info!("Getting cars current territory status");

    let statuses = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.territory_status,
            c.territory_entry_time,
            (
                SELECT created_at 
                FROM cars_history 
                WHERE car_id = c.id AND action_type = 'exit'
                ORDER BY created_at DESC 
                LIMIT 1
            ) as last_exit_time
        FROM cars c
        WHERE c.status = 1
        "#,
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch cars status: {}", e);
        error::ErrorInternalServerError("Error fetching cars status")
    })?;

    let items: Vec<serde_json::Value> = statuses.into_iter().map(|row| {
        json!({
            "car_id": row.id,
            "territory_status": row.territory_status.unwrap_or(0),
            "entry_time": row.territory_entry_time,
            "last_exit_time": row.last_exit_time
        })
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Обновление статуса нахождения на территории (въезд/выезд)
pub async fn update_car_territory_status(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<UpdateTerritoryStatusRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let car_id = path.into_inner();
    let now = Utc::now();
    let action_type = if form.territory_status == 1 { "entry" } else if form.territory_status == 2 { "exit" } else { "unknown" };

    log::info!("Updating car {} territory status to {} by user {}", car_id, form.territory_status, form.user_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем текущий статус машины
    let current_car = sqlx::query!(
        "SELECT car_number, car_brand, territory_status FROM cars WHERE id = $1",
        car_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch car: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let car = match current_car {
        Some(c) => c,
        None => return Err(error::ErrorNotFound("Car not found")),
    };

    // Обновляем статус в таблице cars
    sqlx::query!(
        r#"
        UPDATE cars 
        SET territory_status = $1,
            territory_entry_time = CASE 
                WHEN $2 = 1 THEN NOW()
                ELSE territory_entry_time
            END,
            updated_at = NOW()
        WHERE id = $3
        "#,
        form.territory_status,
        form.territory_status,
        car_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update car territory status: {}", e);
        error::ErrorInternalServerError("Error updating car territory status")
    })?;

    // Добавляем запись в историю
    let comment = if form.territory_status == 1 {
        format!("Автомобиль {} въехал на территорию", car.car_number)
    } else if form.territory_status == 2 {
        format!("Автомобиль {} выехал с территории", car.car_number)
    } else {
        String::new()
    };

    sqlx::query!(
        r#"
        INSERT INTO cars_history (
            car_id,
            user_id,
            action_type,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5)
        "#,
        car_id,
        form.user_id,
        action_type,
        comment,
        now.naive_utc()
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
        "message": "Car territory status updated successfully",
        "territory_status": form.territory_status
    })))
}

/// Деактивация автомобиля (удаление из таблицы)
pub async fn deactivate_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<DeactivateCarRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let car_id = path.into_inner();

    log::info!("Deactivating car {} by user {}", car_id, form.user_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем информацию о машине
    let current_car = sqlx::query!(
        "SELECT car_number, car_brand FROM cars WHERE id = $1",
        car_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch car: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let car = match current_car {
        Some(c) => c,
        None => return Err(error::ErrorNotFound("Car not found")),
    };

    // Обновляем статус
    sqlx::query!(
        "UPDATE cars SET status = $1, date_removed = CURRENT_DATE, updated_at = NOW() WHERE id = $2",
        form.status,
        car_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to deactivate car: {}", e);
        error::ErrorInternalServerError("Error deactivating car")
    })?;

    // Добавляем запись в историю
    sqlx::query!(
        r#"
        INSERT INTO cars_history (
            car_id,
            user_id,
            action_type,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5)
        "#,
        car_id,
        form.user_id,
        "delete",
        format!("Автомобиль {} {} удалён пользователем", car.car_number, car.car_brand),
        Utc::now().naive_utc()
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
        "message": "Car deactivated successfully"
    })))
}

/// Активация автомобиля (ввод в работу)
pub async fn activate_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<ActivateCarRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let car_id = path.into_inner();

    log::info!("Activating car {} by user {}", car_id, form.user_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем информацию о машине
    let current_car = sqlx::query!(
        "SELECT car_number, car_brand FROM cars WHERE id = $1",
        car_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch car: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let car = match current_car {
        Some(c) => c,
        None => return Err(error::ErrorNotFound("Car not found")),
    };

    // Обновляем статус на активный (1)
    sqlx::query!(
        "UPDATE cars SET status = 1, date_removed = NULL, updated_at = NOW() WHERE id = $1",
        car_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to activate car: {}", e);
        error::ErrorInternalServerError("Error activating car")
    })?;

    // Добавляем запись в историю
    sqlx::query!(
        r#"
        INSERT INTO cars_history (
            car_id,
            user_id,
            action_type,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5)
        "#,
        car_id,
        form.user_id,
        "activate",
        format!("Автомобиль {} {} введён в работу", car.car_number, car.car_brand),
        Utc::now().naive_utc()
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add car history entry: {}", e);
        error::ErrorInternalServerError("Error adding car history entry")
    })?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Car activated successfully"
    })))
}

/// Восстановление автомобиля
pub async fn restore_car(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<RestoreCarRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let car_id = path.into_inner();

    log::info!("Restoring car {} by user {}", car_id, form.user_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем информацию о машине
    let current_car = sqlx::query!(
        "SELECT car_number, car_brand FROM cars WHERE id = $1",
        car_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch car: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let car = match current_car {
        Some(c) => c,
        None => return Err(error::ErrorNotFound("Car not found")),
    };

    // Восстанавливаем статус на активный (1) и убираем дату удаления
    sqlx::query!(
        "UPDATE cars SET status = 1, date_removed = NULL, updated_at = NOW() WHERE id = $1",
        car_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to restore car: {}", e);
        error::ErrorInternalServerError("Error restoring car")
    })?;

    // Добавляем запись в историю
    sqlx::query!(
        r#"
        INSERT INTO cars_history (
            car_id,
            user_id,
            action_type,
            comment,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5)
        "#,
        car_id,
        form.user_id,
        "restore",
        format!("Автомобиль {} {} восстановлен", car.car_number, car.car_brand),
        Utc::now().naive_utc()
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to add car history entry: {}", e);
        error::ErrorInternalServerError("Error adding car history entry")
    })?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Car restored successfully"
    })))
}

