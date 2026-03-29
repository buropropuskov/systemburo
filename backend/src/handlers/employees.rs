use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};
use chrono::{NaiveDate, NaiveDateTime, Utc};

use crate::auth::decode_token;
use crate::handlers::employees_history::add_employee_history_entry;
use crate::models::employees_history::{UpdateTerritoryStatusRequest, DeactivateEmployeeRequest, ActivateEmployeeRequest, RestoreEmployeeRequest};

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

#[derive(Debug, serde::Serialize)]
struct TableEmployee {
    id: i32,
    last_name: String,
    first_name: String,
    middle_name: Option<String>,
    organization: Option<String>,
    organization_id: Option<i32>,
    company: Option<String>,
    company_id: Option<i32>,
    entry_date_to: Option<NaiveDate>,
    pass_time: Option<String>,
    status: i32,
    territory_status: Option<i32>,
    territory_entry_time: Option<NaiveDateTime>,
    application_id: Option<i32>,
    target_tables: Option<Vec<i32>>,
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

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

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

/// Получение активных сотрудников для таблицы
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

    let rows = sqlx::query!(
        r#"
        SELECT 
            e.id,
            e.last_name,
            e.first_name,
            e.middle_name,
            e.position,
            e.passport_series_number,
            e.patent_number,
            e.other_permission,
            e.territory_status,
            e.territory_entry_time,
            ci.name as citizenship_name,
            o.name as organization,
            o.id as organization_id,
            c.name as company,
            c.id as company_id,
            a.entry_date_to,
            CONCAT(a.entry_time_from, ' - ', a.entry_time_to) as pass_time,
            e.status,
            app.id as application_id,
            (
                SELECT array_agg(table_id)
                FROM employee_target_tables
                WHERE employee_id = e.id
            ) as target_tables
        FROM employees e
        JOIN employee_target_tables ett ON e.id = ett.employee_id
        JOIN attachments a ON e.attachment_id = a.id
        JOIN applications app ON a.application_id = app.id
        LEFT JOIN citizenships ci ON e.citizenship_id = ci.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c ON app.company_id = c.id
        WHERE ett.table_id = $1
        AND e.status = 1
        AND app.confirmation = 'Согласовано'
        AND app.status IN ('В работе', 'Завершено')
        AND a.entry_date_to >= CURRENT_DATE
        GROUP BY e.id, e.last_name, e.first_name, e.middle_name, e.position, e.passport_series_number, e.patent_number, e.other_permission, e.territory_status, e.territory_entry_time, ci.name,
                 o.name, o.id, c.name, c.id, a.entry_date_to, a.entry_time_from, 
                 a.entry_time_to, e.status, app.id
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

    #[derive(Debug, Serialize)]
    struct TableEmployee {
        id: i32,
        last_name: String,
        first_name: String,
        middle_name: Option<String>,
        position: Option<String>,
        passport_series_number: Option<String>,
        patent_number: Option<String>,
        other_permission: Option<String>,
        citizenship_name: Option<String>,
        organization: Option<String>,
        organization_id: Option<i32>,
        company: Option<String>,
        company_id: Option<i32>,
        entry_date_to: Option<NaiveDate>,
        pass_time: Option<String>,
        status: Option<i32>,
        territory_status: Option<i32>,
        territory_entry_time: Option<NaiveDateTime>,
        application_id: i32,
        target_tables: Option<Vec<i32>>,
    }

    let employees: Vec<TableEmployee> = rows
        .into_iter()
        .map(|row| TableEmployee {
            id: row.id,
            last_name: row.last_name,
            first_name: row.first_name,
            middle_name: row.middle_name,
            position: row.position,
            passport_series_number: row.passport_series_number,
            patent_number: row.patent_number,
            other_permission: row.other_permission,
            citizenship_name: Some(row.citizenship_name),
            organization: Some(row.organization),
            organization_id: Some(row.organization_id),
            company: Some(row.company),
            company_id: Some(row.company_id),
            entry_date_to: row.entry_date_to,
            pass_time: row.pass_time,
            status: row.status,
            territory_status: row.territory_status,
            territory_entry_time: row.territory_entry_time,
            application_id: row.application_id,
            target_tables: row.target_tables,
        })
        .collect();

    Ok(HttpResponse::Ok().json(employees))
}

/// Обновление статуса территории (вход/выход)
pub async fn update_employee_territory_status(
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

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let employee_id = path.into_inner();
    let now = Utc::now();
    let action_type = if form.territory_status == 1 { "entry" } else if form.territory_status == 2 { "exit" } else { "unknown" };

    log::info!("Updating employee {} territory status to {} by user {:?}", employee_id, form.territory_status, form.user_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    let current_employee = sqlx::query!(
        "SELECT last_name, first_name, middle_name FROM employees WHERE id = $1",
        employee_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employee: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let employee = match current_employee {
        Some(e) => e,
        None => return Err(error::ErrorNotFound("Employee not found")),
    };

    sqlx::query!(
        r#"
        UPDATE employees 
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
        employee_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to update employee territory status: {}", e);
        error::ErrorInternalServerError("Error updating employee territory status")
    })?;

    let full_name = format!("{} {} {}", 
        employee.last_name,
        employee.first_name,
        employee.middle_name.as_deref().unwrap_or("")
    ).trim().to_string();

    let comment = if form.territory_status == 1 {
        format!("Сотрудник {} прошёл на территорию", full_name)
    } else if form.territory_status == 2 {
        format!("Сотрудник {} покинул территорию", full_name)
    } else {
        String::new()
    };

    add_employee_history_entry(
        &pool,
        employee_id,
        form.user_id,
        form.table_id,
        action_type,
        None,
        None,
        None,
        Some(&comment),
        None,
    ).await?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Employee territory status updated successfully",
        "territory_status": form.territory_status
    })))
}

/// Деактивация сотрудника
pub async fn deactivate_employee(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<DeactivateEmployeeRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let employee_id = path.into_inner();

    log::info!("Deactivating employee {} by user {:?}", employee_id, form.user_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    let current_employee = sqlx::query!(
        "SELECT last_name, first_name, middle_name FROM employees WHERE id = $1",
        employee_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employee: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let employee = match current_employee {
        Some(e) => e,
        None => return Err(error::ErrorNotFound("Employee not found")),
    };

    sqlx::query!(
        "UPDATE employees SET status = $1, date_deleted = CURRENT_DATE, updated_at = NOW() WHERE id = $2",
        form.status,
        employee_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to deactivate employee: {}", e);
        error::ErrorInternalServerError("Error deactivating employee")
    })?;

    let full_name = format!("{} {} {}", 
        employee.last_name,
        employee.first_name,
        employee.middle_name.as_deref().unwrap_or("")
    ).trim().to_string();

    add_employee_history_entry(
        &pool,
        employee_id,
        form.user_id,
        None,
        "delete",
        None,
        None,
        None,
        Some(&format!("Сотрудник {} удалён пользователем", full_name)),
        None,
    ).await?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Employee deactivated successfully"
    })))
}

/// Активация сотрудника
pub async fn activate_employee(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<ActivateEmployeeRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let employee_id = path.into_inner();

    log::info!("Activating employee {} by user {:?}", employee_id, form.user_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    let current_employee = sqlx::query!(
        "SELECT last_name, first_name, middle_name FROM employees WHERE id = $1",
        employee_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employee: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let employee = match current_employee {
        Some(e) => e,
        None => return Err(error::ErrorNotFound("Employee not found")),
    };

    sqlx::query!(
        "UPDATE employees SET status = 1, date_deleted = NULL, updated_at = NOW() WHERE id = $1",
        employee_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to activate employee: {}", e);
        error::ErrorInternalServerError("Error activating employee")
    })?;

    let full_name = format!("{} {} {}", 
        employee.last_name,
        employee.first_name,
        employee.middle_name.as_deref().unwrap_or("")
    ).trim().to_string();

    add_employee_history_entry(
        &pool,
        employee_id,
        form.user_id,
        None,
        "activate",
        None,
        None,
        None,
        Some(&format!("Сотрудник {} введён в работу", full_name)),
        None,
    ).await?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Employee activated successfully"
    })))
}

/// Восстановление сотрудника
pub async fn restore_employee(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<RestoreEmployeeRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let employee_id = path.into_inner();

    log::info!("Restoring employee {} by user {:?}", employee_id, form.user_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    let current_employee = sqlx::query!(
        "SELECT last_name, first_name, middle_name FROM employees WHERE id = $1",
        employee_id
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employee: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let employee = match current_employee {
        Some(e) => e,
        None => return Err(error::ErrorNotFound("Employee not found")),
    };

    sqlx::query!(
        "UPDATE employees SET status = 1, date_deleted = NULL, updated_at = NOW() WHERE id = $1",
        employee_id
    )
    .execute(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to restore employee: {}", e);
        error::ErrorInternalServerError("Error restoring employee")
    })?;

    let full_name = format!("{} {} {}", 
        employee.last_name,
        employee.first_name,
        employee.middle_name.as_deref().unwrap_or("")
    ).trim().to_string();

    add_employee_history_entry(
        &pool,
        employee_id,
        form.user_id,
        None,
        "restore",
        None,
        None,
        None,
        Some(&format!("Сотрудник {} восстановлен", full_name)),
        None,
    ).await?;

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Employee restored successfully"
    })))
}