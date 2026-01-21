use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};

use crate::auth::decode_token;

#[derive(Debug, Deserialize)]
pub struct EmployeeData {
    pub last_name: String,
    pub first_name: String,
    pub middle_name: Option<String>,
    pub citizenship_id: i32,
    pub position: String,
    pub passport_series_number: String,
    pub patent_number: Option<String>,
    pub other_permission: Option<String>,
    pub target_tables: Vec<i32>,
}

/// Создание сотрудника
pub async fn create_employee(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    employee_data: web::Json<EmployeeData>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Создаем сотрудника (attachment_id будет установлен позже при создании вложения)
    let employee_result = sqlx::query!(
        r#"
        INSERT INTO employees (
            last_name,
            first_name,
            middle_name,
            citizenship_id,
            position,
            passport_series_number,
            patent_number,
            other_permission,
            status
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)
        RETURNING id
        "#,
        employee_data.last_name,
        employee_data.first_name,
        employee_data.middle_name.as_deref(),
        employee_data.citizenship_id,
        employee_data.position,
        employee_data.passport_series_number,
        employee_data.patent_number.as_deref(),
        employee_data.other_permission.as_deref()
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to create employee: {}", e);
        error::ErrorInternalServerError("Error creating employee")
    })?;

    let employee_id = employee_result.id;

    // Создаем связи с целевыми таблицами
    for &table_id in &employee_data.target_tables {
        sqlx::query!(
            r#"
            INSERT INTO employee_target_tables (employee_id, table_id, order_index)
            VALUES ($1, $2, 1)
            "#,
            employee_id,
            table_id
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to create employee target table: {}", e);
            error::ErrorInternalServerError("Error creating employee target table")
        })?;
    }

    // Фиксируем транзакцию
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Employee created successfully",
        "employee_id": employee_id
    })))
}