use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};
use chrono::NaiveDate;

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
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0)
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

/// Получение активных сотрудников для конкретной таблицы
/// Получение активных сотрудников для конкретной таблицы
pub async fn get_active_employees_for_table(
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

    log::info!("Getting active employees for table {}", table_id);

    #[derive(Debug, serde::Serialize)]
    struct TableEmployee {
        id: i32,
        last_name: String,
        first_name: String,
        middle_name: Option<String>,
        organization: Option<String>,
        entry_date_to: Option<NaiveDate>,
        pass_time: Option<String>,
        status: i32,
    }

    // Используем query! вместо query_as! для большей гибкости
    let rows = sqlx::query!(
        r#"
        SELECT 
            e.id,
            e.last_name,
            e.first_name,
            e.middle_name,
            COALESCE(o.name, co.name) as organization,
            a.entry_date_to,
            CONCAT(a.entry_time_from, ' - ', a.entry_time_to) as pass_time,
            e.status
        FROM employees e
        JOIN employee_target_tables ett ON e.id = ett.employee_id
        JOIN attachments a ON e.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies co ON app.company_id = co.id
        WHERE ett.table_id = $1
        AND e.status = 1
        AND app.confirmation = 'Согласовано'
        AND app.status IN ('В работе', 'Завершено')
        AND CURRENT_DATE BETWEEN a.entry_date_from AND a.entry_date_to
        GROUP BY e.id, e.last_name, e.first_name, e.middle_name, 
                 o.name, co.name, a.entry_date_to, a.entry_time_from, 
                 a.entry_time_to, e.status
        ORDER BY e.last_name, e.first_name
        "#,
        table_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch active employees: {}", e);
        error::ErrorInternalServerError("Error fetching active employees")
    })?;

    let employees: Vec<TableEmployee> = rows.iter().map(|row| {
        TableEmployee {
            id: row.id,
            last_name: row.last_name.clone(),
            first_name: row.first_name.clone(),
            middle_name: row.middle_name.clone(),
            organization: row.organization.clone(),
            entry_date_to: row.entry_date_to,
            pass_time: row.pass_time.clone(),
            status: row.status.unwrap_or(0),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(employees))
}