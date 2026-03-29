use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use serde_json::json;
use log;
use chrono::{Duration};

use crate::auth::decode_token;
use crate::models::employees_history::*;

use serde::Deserialize;

#[derive(Debug, Deserialize)]
pub struct UnifiedEmployeeHistoryQuery {
    pub last_name: String,
    pub first_name: String,
    pub middle_name: Option<String>,
    // организация и компания больше не нужны для поиска
}

/// Получение объединённой истории для всех сотрудников с одинаковыми ФИО
pub async fn get_unified_employee_history(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<UnifiedEmployeeHistoryQuery>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("=== Unified Employee History Request ===");
    log::info!("last_name: '{}'", query.last_name);
    log::info!("first_name: '{}'", query.first_name);
    log::info!("middle_name: {:?}", query.middle_name);

    let last_name_clean = query.last_name.trim().to_lowercase();
    let first_name_clean = query.first_name.trim().to_lowercase();
    let middle_name_clean = query.middle_name.as_ref().map(|m| m.trim().to_lowercase());

    // Находим всех сотрудников с таким ФИО (без учёта организации и компании)
    let employees = sqlx::query!(
        r#"
        SELECT e.id
        FROM employees e
        WHERE LOWER(TRIM(e.last_name)) = $1
          AND LOWER(TRIM(e.first_name)) = $2
          AND ($3::text IS NULL OR LOWER(TRIM(e.middle_name)) = $3)
        ORDER BY e.id
        "#,
        last_name_clean,
        first_name_clean,
        middle_name_clean,
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employees: {}", e);
        error::ErrorInternalServerError("Error fetching employees")
    })?;

    log::info!("Found {} employees with these params", employees.len());

    if employees.is_empty() {
        log::warn!("No employees found for unified history");
        return Ok(HttpResponse::Ok().json(Vec::<serde_json::Value>::new()));
    }

    let employee_ids: Vec<i32> = employees.iter().map(|e| e.id).collect();
    log::info!("Employee IDs: {:?}", employee_ids);

    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.employee_id,
            h.user_id,
            h.table_id,
            st.display_name as "table_name?",
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
            u.last_name as user_last_name,
            u.first_name as user_first_name,
            u.middle_name as user_middle_name,
            h.action_type as "action_type!",
            h.field_name,
            h.old_value,
            h.new_value,
            h.comment,
            h.created_at as "created_at!",
            h.metadata as "metadata?",
            e.last_name as employee_last_name,
            e.first_name as employee_first_name,
            e.middle_name as employee_middle_name,
            COALESCE(o.name, c.name) as organization,
            c.name as company
        FROM employees_history h
        LEFT JOIN users u ON h.user_id = u.id
        JOIN employees e ON h.employee_id = e.id
        LEFT JOIN system_tables st ON h.table_id = st.id
        LEFT JOIN attachments a ON e.attachment_id = a.id
        LEFT JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c ON app.company_id = c.id
        WHERE h.employee_id = ANY($1)
        ORDER BY h.created_at DESC
        "#,
        &employee_ids[..]
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch unified employee history: {}", e);
        error::ErrorInternalServerError("Error fetching unified employee history")
    })?;

    log::info!("Found {} history records total", history.len());

    let items: Vec<EmployeeHistoryItem> = history.into_iter().map(|row| {
        let msk_time = row.created_at + Duration::hours(3);
        let created_at_str = msk_time.format("%Y-%m-%dT%H:%M:%S+03:00").to_string();

        EmployeeHistoryItem {
            id: row.id,
            employee_id: row.employee_id,
            user_id: row.user_id,
            table_id: row.table_id,
            table_name: row.table_name,
            user_name: if row.user_name.is_empty() { "Система".to_string() } else { row.user_name },
            last_name: row.user_last_name,
            first_name: row.user_first_name,
            middle_name: row.user_middle_name,
            action_type: row.action_type,
            field_name: row.field_name,
            old_value: row.old_value,
            new_value: row.new_value,
            comment: row.comment,
            created_at: created_at_str,
            metadata: row.metadata,
            employee_last_name: Some(row.employee_last_name),
            employee_first_name: Some(row.employee_first_name),
            employee_middle_name: row.employee_middle_name,
            organization: row.organization,
            company: Some(row.company),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Получение истории конкретного сотрудника по ID
pub async fn get_employee_history(
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

    let employee_id = path.into_inner();

    log::info!("Getting history for employee: {}", employee_id);

    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.employee_id,
            h.user_id,
            h.table_id,
            st.display_name as "table_name?",
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
            u.last_name as user_last_name,
            u.first_name as user_first_name,
            u.middle_name as user_middle_name,
            h.action_type as "action_type!",
            h.field_name,
            h.old_value,
            h.new_value,
            h.comment,
            h.created_at as "created_at!",
            h.metadata as "metadata?"
        FROM employees_history h
        LEFT JOIN users u ON h.user_id = u.id
        LEFT JOIN system_tables st ON h.table_id = st.id
        WHERE h.employee_id = $1
        ORDER BY h.created_at DESC
        "#,
        employee_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employee history: {}", e);
        error::ErrorInternalServerError(format!("Database error: {}", e))
    })?;

    log::info!("Found {} history records for employee {}", history.len(), employee_id);

    let items: Vec<EmployeeHistoryItem> = history.into_iter().map(|row| {
        let msk_time = row.created_at + Duration::hours(3);
        let created_at_str = msk_time.format("%Y-%m-%dT%H:%M:%S+03:00").to_string();

        EmployeeHistoryItem {
            id: row.id,
            employee_id: row.employee_id,
            user_id: row.user_id,
            table_id: row.table_id,
            table_name: row.table_name,
            user_name: if row.user_name.is_empty() { "Система".to_string() } else { row.user_name },
            last_name: row.user_last_name,
            first_name: row.user_first_name,
            middle_name: row.user_middle_name,
            action_type: row.action_type,
            field_name: row.field_name,
            old_value: row.old_value,
            new_value: row.new_value,
            comment: row.comment,
            created_at: created_at_str,
            metadata: row.metadata,
            employee_last_name: None,
            employee_first_name: None,
            employee_middle_name: None,
            organization: None,
            company: None,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Добавление записи в историю сотрудника (внутреннее использование)
pub async fn add_employee_history_entry(
    pool: &PgPool,
    employee_id: i32,
    user_id: Option<i32>,
    table_id: Option<i32>,
    action_type: &str,
    field_name: Option<&str>,
    old_value: Option<&str>,
    new_value: Option<&str>,
    comment: Option<&str>,
    metadata: Option<serde_json::Value>,
) -> Result<(), Error> {
    sqlx::query!(
        r#"
        INSERT INTO employees_history (
            employee_id,
            user_id,
            table_id,
            action_type,
            field_name,
            old_value,
            new_value,
            comment,
            metadata,
            created_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
        "#,
        employee_id,
        user_id,
        table_id,
        action_type,
        field_name,
        old_value,
        new_value,
        comment,
        metadata
    )
    .execute(pool)
    .await
    .map_err(|e| {
        log::error!("Failed to add employee history entry: {}", e);
        error::ErrorInternalServerError("Error adding employee history entry")
    })?;

    Ok(())
}

/// Получение истории проходов (entry/exit) всех сотрудников (для общей таблицы)
pub async fn get_all_employees_entry_exit_history(
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

    log::info!("Getting all employees entry/exit history");

    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.employee_id,
            h.user_id,
            h.table_id,
            st.display_name as table_name,
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
            e.last_name,
            e.first_name,
            e.middle_name,
            COALESCE(o.name, c.name) as organization,
            c.name as company
        FROM employees_history h
        LEFT JOIN users u ON h.user_id = u.id
        JOIN employees e ON h.employee_id = e.id
        LEFT JOIN system_tables st ON h.table_id = st.id
        LEFT JOIN attachments a ON e.attachment_id = a.id
        LEFT JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c ON app.company_id = c.id
        WHERE h.action_type IN ('entry', 'exit')
        ORDER BY h.created_at DESC
        "#,
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch all employees entry/exit history: {}", e);
        error::ErrorInternalServerError("Error fetching history")
    })?;

    log::info!("Found {} all employees entry/exit records", history.len());

    let items: Vec<serde_json::Value> = history.into_iter().map(|row| {
        let msk_time = row.created_at + Duration::hours(3);
        let created_at_str = msk_time.format("%Y-%m-%dT%H:%M:%S+03:00").to_string();

        json!({
            "id": row.id,
            "employee_id": row.employee_id,
            "user_id": row.user_id,
            "table_id": row.table_id,
            "table_name": row.table_name,
            "user_name": if row.user_name.is_empty() { "Система".to_string() } else { row.user_name },
            "action_type": row.action_type,
            "comment": row.comment,
            "created_at": created_at_str,
            "last_name": row.last_name,
            "first_name": row.first_name,
            "middle_name": row.middle_name,
            "organization": row.organization,
            "company": row.company
        })
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Получение текущего статуса территории для всех активных сотрудников
pub async fn get_employees_current_status(
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

    log::info!("Getting employees current territory status");

    let statuses = sqlx::query!(
        r#"
        SELECT 
            e.id,
            e.territory_status,
            e.territory_entry_time,
            (
                SELECT created_at 
                FROM employees_history 
                WHERE employee_id = e.id AND action_type = 'exit'
                ORDER BY created_at DESC 
                LIMIT 1
            ) as last_exit_time
        FROM employees e
        WHERE e.status = 1
        "#,
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employees status: {}", e);
        error::ErrorInternalServerError("Error fetching employees status")
    })?;

    let items: Vec<serde_json::Value> = statuses.into_iter().map(|row| {
        let entry_time = row.territory_entry_time.map(|t| {
            (t + Duration::hours(3))
                .format("%Y-%m-%dT%H:%M:%S+03:00")
                .to_string()
        });
        let last_exit_time = row.last_exit_time.map(|t| {
            (t + Duration::hours(3))
                .format("%Y-%m-%dT%H:%M:%S+03:00")
                .to_string()
        });

        json!({
            "employee_id": row.id,
            "territory_status": row.territory_status.unwrap_or(0),
            "entry_time": entry_time,
            "last_exit_time": last_exit_time
        })
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}

/// Получение истории проходов для конкретной таблицы
pub async fn get_table_employees_history(
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

    log::info!("Getting employees entry/exit history for table {}", table_id);

    let history = sqlx::query!(
        r#"
        SELECT 
            h.id,
            h.employee_id,
            h.user_id,
            h.table_id,
            st.display_name as table_name,
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
            e.last_name,
            e.first_name,
            e.middle_name,
            COALESCE(o.name, c.name) as organization,
            c.name as company
        FROM employees_history h
        LEFT JOIN users u ON h.user_id = u.id
        JOIN employees e ON h.employee_id = e.id
        LEFT JOIN system_tables st ON h.table_id = st.id
        LEFT JOIN attachments a ON e.attachment_id = a.id
        LEFT JOIN applications app ON a.application_id = app.id
        LEFT JOIN organizations o ON app.organization_id = o.id
        LEFT JOIN companies c ON app.company_id = c.id
        WHERE h.action_type IN ('entry', 'exit')
          AND h.table_id = $1
        ORDER BY h.created_at DESC
        "#,
        table_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch table employees history: {}", e);
        error::ErrorInternalServerError("Error fetching history")
    })?;

    log::info!("Found {} records for table {}", history.len(), table_id);

    let items: Vec<serde_json::Value> = history.into_iter().map(|row| {
        let msk_time = row.created_at + Duration::hours(3);
        let created_at_str = msk_time.format("%Y-%m-%dT%H:%M:%S+03:00").to_string();

        json!({
            "id": row.id,
            "employee_id": row.employee_id,
            "user_id": row.user_id,
            "table_id": row.table_id,
            "table_name": row.table_name,
            "user_name": if row.user_name.is_empty() { "Система".to_string() } else { row.user_name },
            "action_type": row.action_type,
            "comment": row.comment,
            "created_at": created_at_str,
            "last_name": row.last_name,
            "first_name": row.first_name,
            "middle_name": row.middle_name,
            "organization": row.organization,
            "company": row.company
        })
    }).collect();

    Ok(HttpResponse::Ok().json(items))
}