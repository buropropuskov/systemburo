use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};

use crate::models::unique_employees::{
    UniqueEmployee, NewUniqueEmployee, UniqueEmployeeWithRelations, EmployeeOwnerInfo
};
use crate::auth::decode_token;

/// Получение информации о владельце для фильтрации сотрудников
async fn get_employee_owner_info(
    pool: &web::Data<PgPool>,
    username: &str,
) -> Result<EmployeeOwnerInfo, Error> {
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

    Ok(EmployeeOwnerInfo {
        has_organization: user_info.has_organization.unwrap_or(false),
        has_company: user_info.has_company.unwrap_or(false),
        organization_id: Some(user_info.organization_id),
        company_id: Some(user_info.company_id),
        user_id: user_info.user_id,
    })
}

/// Получение сотрудников с фильтрацией по владельцу
pub async fn get_unique_employees(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<std::collections::HashMap<String, String>>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_employee_owner_info(&pool, &claims.sub).await?;
                        
                        let filter_type = query.get("filter_type").map(|s| s.as_str()).unwrap_or("user");
                        
                        log::info!("Fetching employees for user: {}, filter_type: {}", owner_info.user_id, filter_type);

                        // Определяем SQL запрос и параметры
                        let (sql, param): (&str, i32) = match filter_type {
                            "organization" if owner_info.has_organization => (
                                r#"
                                SELECT 
                                    ue.id,
                                    ue.last_name,
                                    ue.first_name,
                                    ue.middle_name,
                                    ue.organization_id,
                                    ue.company_id,
                                    ue.citizenship_id,
                                    ue.user_id,
                                    ue.position,
                                    ue.passport_series_number,
                                    ue.patent_number,
                                    ue.other_permission,
                                    ue.status,
                                    ue.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    cit.name as citizenship_name
                                FROM unique_employees ue
                                LEFT JOIN organizations o ON ue.organization_id = o.id
                                LEFT JOIN companies c ON ue.company_id = c.id
                                LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id
                                WHERE ue.organization_id = $1
                                ORDER BY ue.last_name, ue.first_name, ue.middle_name
                                "#,
                                owner_info.organization_id.unwrap_or(0)
                            ),
                            "company" if owner_info.has_company => (
                                r#"
                                SELECT 
                                    ue.id,
                                    ue.last_name,
                                    ue.first_name,
                                    ue.middle_name,
                                    ue.organization_id,
                                    ue.company_id,
                                    ue.citizenship_id,
                                    ue.user_id,
                                    ue.position,
                                    ue.passport_series_number,
                                    ue.patent_number,
                                    ue.other_permission,
                                    ue.status,
                                    ue.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    cit.name as citizenship_name
                                FROM unique_employees ue
                                LEFT JOIN organizations o ON ue.organization_id = o.id
                                LEFT JOIN companies c ON ue.company_id = c.id
                                LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id
                                WHERE ue.company_id = $1
                                ORDER BY ue.last_name, ue.first_name, ue.middle_name
                                "#,
                                owner_info.company_id.unwrap_or(0)
                            ),
                            "all" => (
                                r#"
                                SELECT 
                                    ue.id,
                                    ue.last_name,
                                    ue.first_name,
                                    ue.middle_name,
                                    ue.organization_id,
                                    ue.company_id,
                                    ue.citizenship_id,
                                    ue.user_id,
                                    ue.position,
                                    ue.passport_series_number,
                                    ue.patent_number,
                                    ue.other_permission,
                                    ue.status,
                                    ue.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    cit.name as citizenship_name
                                FROM unique_employees ue
                                LEFT JOIN organizations o ON ue.organization_id = o.id
                                LEFT JOIN companies c ON ue.company_id = c.id
                                LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id
                                WHERE ue.user_id = $1 OR ue.organization_id = $2 OR ue.company_id = $3
                                ORDER BY ue.last_name, ue.first_name, ue.middle_name
                                "#,
                                owner_info.user_id
                            ),
                            _ => (
                                r#"
                                SELECT 
                                    ue.id,
                                    ue.last_name,
                                    ue.first_name,
                                    ue.middle_name,
                                    ue.organization_id,
                                    ue.company_id,
                                    ue.citizenship_id,
                                    ue.user_id,
                                    ue.position,
                                    ue.passport_series_number,
                                    ue.patent_number,
                                    ue.other_permission,
                                    ue.status,
                                    ue.created_at,
                                    o.name as organization_name,
                                    c.name as company_name,
                                    cit.name as citizenship_name
                                FROM unique_employees ue
                                LEFT JOIN organizations o ON ue.organization_id = o.id
                                LEFT JOIN companies c ON ue.company_id = c.id
                                LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id
                                WHERE ue.user_id = $1
                                ORDER BY ue.last_name, ue.first_name, ue.middle_name
                                "#,
                                owner_info.user_id
                            )
                        };

                        let employees_rows = if filter_type == "all" {
                            sqlx::query(sql)
                                .bind(owner_info.user_id)
                                .bind(owner_info.organization_id)
                                .bind(owner_info.company_id)
                                .fetch_all(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to fetch unique employees: {}", e);
                                    error::ErrorInternalServerError("Error fetching employees")
                                })?
                        } else {
                            sqlx::query(sql)
                                .bind(param)
                                .fetch_all(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to fetch unique employees: {}", e);
                                    error::ErrorInternalServerError("Error fetching employees")
                                })?
                        };

                        log::info!("Fetched {} employees for user {}", employees_rows.len(), owner_info.user_id);

                        let employees: Vec<UniqueEmployeeWithRelations> = employees_rows.into_iter().map(|row| {
                            UniqueEmployeeWithRelations {
                                id: row.get("id"),
                                last_name: row.get("last_name"),
                                first_name: row.get("first_name"),
                                middle_name: row.get("middle_name"),
                                organization_id: row.get("organization_id"),
                                company_id: row.get("company_id"),
                                citizenship_id: row.get("citizenship_id"),
                                user_id: row.get("user_id"),
                                position: row.get("position"),
                                passport_series_number: row.get("passport_series_number"),
                                patent_number: row.get("patent_number"),
                                other_permission: row.get("other_permission"),
                                status: row.get::<Option<bool>, _>("status").unwrap_or(false),
                                created_at: row.get("created_at"),
                                organization_name: row.get("organization_name"),
                                company_name: row.get("company_name"),
                                citizenship_name: row.get("citizenship_name"),
                            }
                        }).collect();

                        Ok(HttpResponse::Ok().json(employees))
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

/// Создание нового уникального сотрудника
pub async fn create_unique_employee(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<NewUniqueEmployee>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_employee_owner_info(&pool, &claims.sub).await?;

                        log::info!("Creating employee for user: {}, data: {:?}", owner_info.user_id, form);

                        // Проверка уникальности паспортных данных для пользователя
                        if let Some(passport) = &form.passport_series_number {
                            let user_existing_employee = sqlx::query!(
                                "SELECT id FROM unique_employees WHERE user_id = $1 AND passport_series_number = $2",
                                owner_info.user_id,
                                passport
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|e| {
                                log::error!("Failed to check employee uniqueness for user: {}", e);
                                error::ErrorInternalServerError("Error checking employee uniqueness")
                            })?;

                            if user_existing_employee.is_some() {
                                return Err(error::ErrorBadRequest("Сотрудник с такими паспортными данными уже привязан к вашему аккаунту"));
                            }
                        }

                        // Проверка уникальности паспортных данных для организации
                        if let Some(org_id) = form.organization_id {
                            if let Some(passport) = &form.passport_series_number {
                                let existing_employee = sqlx::query!(
                                    "SELECT id FROM unique_employees WHERE organization_id = $1 AND passport_series_number = $2",
                                    org_id,
                                    passport
                                )
                                .fetch_optional(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to check employee uniqueness for organization: {}", e);
                                    error::ErrorInternalServerError("Error checking employee uniqueness")
                                })?;

                                if existing_employee.is_some() {
                                    return Err(error::ErrorBadRequest("Сотрудник с такими паспортными данными уже существует в этой организации"));
                                }
                            }
                        }

                        // Проверка уникальности паспортных данных для компании
                        if let Some(company_id) = form.company_id {
                            if let Some(passport) = &form.passport_series_number {
                                let existing_employee = sqlx::query!(
                                    "SELECT id FROM unique_employees WHERE company_id = $1 AND passport_series_number = $2",
                                    company_id,
                                    passport
                                )
                                .fetch_optional(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to check employee uniqueness for company: {}", e);
                                    error::ErrorInternalServerError("Error checking employee uniqueness")
                                })?;

                                if existing_employee.is_some() {
                                    return Err(error::ErrorBadRequest("Сотрудник с такими паспортными данными уже существует в этой компании"));
                                }
                            }
                        }

                        // Убедимся, что user_id установлен
                        let user_id = form.user_id.unwrap_or(owner_info.user_id);

                        // Создаем сотрудника
                        let employee_result = sqlx::query!(
                            r#"
                            INSERT INTO unique_employees (
                                last_name, first_name, middle_name, citizenship_id, position,
                                passport_series_number, patent_number, other_permission,
                                organization_id, company_id, user_id, status
                            )
                            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, false)
                            RETURNING id, last_name, first_name, middle_name, citizenship_id, position,
                                      passport_series_number, patent_number, other_permission,
                                      organization_id, company_id, user_id, status, created_at
                            "#,
                            form.last_name,
                            form.first_name,
                            form.middle_name,
                            form.citizenship_id,
                            form.position,
                            form.passport_series_number,
                            form.patent_number,
                            form.other_permission,
                            form.organization_id,
                            form.company_id,
                            user_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create unique employee: {}", e);
                            error::ErrorInternalServerError("Error creating employee")
                        })?;

                        let employee = UniqueEmployee {
                            id: employee_result.id,
                            last_name: employee_result.last_name,
                            first_name: employee_result.first_name,
                            middle_name: employee_result.middle_name,
                            citizenship_id: employee_result.citizenship_id,
                            position: employee_result.position,
                            passport_series_number: employee_result.passport_series_number,
                            patent_number: employee_result.patent_number,
                            other_permission: employee_result.other_permission,
                            organization_id: employee_result.organization_id,
                            company_id: employee_result.company_id,
                            user_id: employee_result.user_id,
                            status: employee_result.status.unwrap_or(false),
                            created_at: employee_result.created_at,
                        };

                        log::info!("Successfully created employee: {:?}", employee);

                        Ok(HttpResponse::Ok().json(employee))
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

/// Обновление существующего сотрудника
pub async fn update_unique_employee(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<NewUniqueEmployee>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let employee_id = path.into_inner();
                        let owner_info = get_employee_owner_info(&pool, &claims.sub).await?;

                        log::info!("Updating employee {} for user: {}, data: {:?}", employee_id, owner_info.user_id, form);

                        // Проверяем, существует ли сотрудник и принадлежит ли пользователю
                        let existing_employee = sqlx::query!(
                            "SELECT user_id, organization_id, company_id FROM unique_employees WHERE id = $1",
                            employee_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch employee: {}", e);
                            error::ErrorInternalServerError("Error fetching employee")
                        })?;

                        if existing_employee.is_none() {
                            return Err(error::ErrorNotFound("Employee not found"));
                        }

                        let employee = existing_employee.unwrap();

                        // Проверяем права на редактирование
                        let can_edit = employee.user_id == Some(owner_info.user_id) ||
                                      (employee.organization_id.is_some() && employee.organization_id == owner_info.organization_id) ||
                                      (employee.company_id.is_some() && employee.company_id == owner_info.company_id);

                        if !can_edit {
                            return Err(error::ErrorForbidden("You don't have permission to edit this employee"));
                        }

                        // Проверка уникальности паспортных данных для пользователя (исключая текущего сотрудника)
                        if let Some(passport) = &form.passport_series_number {
                            let user_existing_employee = sqlx::query!(
                                "SELECT id FROM unique_employees WHERE user_id = $1 AND passport_series_number = $2 AND id != $3",
                                owner_info.user_id,
                                passport,
                                employee_id
                            )
                            .fetch_optional(pool.get_ref())
                            .await
                            .map_err(|e| {
                                log::error!("Failed to check employee uniqueness for user: {}", e);
                                error::ErrorInternalServerError("Error checking employee uniqueness")
                            })?;

                            if user_existing_employee.is_some() {
                                return Err(error::ErrorBadRequest("Сотрудник с такими паспортными данными уже привязан к вашему аккаунту"));
                            }
                        }

                        // Проверка уникальности паспортных данных для организации (исключая текущего сотрудника)
                        if let Some(org_id) = form.organization_id {
                            if let Some(passport) = &form.passport_series_number {
                                let existing_employee = sqlx::query!(
                                    "SELECT id FROM unique_employees WHERE organization_id = $1 AND passport_series_number = $2 AND id != $3",
                                    org_id,
                                    passport,
                                    employee_id
                                )
                                .fetch_optional(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to check employee uniqueness for organization: {}", e);
                                    error::ErrorInternalServerError("Error checking employee uniqueness")
                                })?;

                                if existing_employee.is_some() {
                                    return Err(error::ErrorBadRequest("Сотрудник с такими паспортными данными уже существует в этой организации"));
                                }
                            }
                        }

                        // Проверка уникальности паспортных данных для компании (исключая текущего сотрудника)
                        if let Some(company_id) = form.company_id {
                            if let Some(passport) = &form.passport_series_number {
                                let existing_employee = sqlx::query!(
                                    "SELECT id FROM unique_employees WHERE company_id = $1 AND passport_series_number = $2 AND id != $3",
                                    company_id,
                                    passport,
                                    employee_id
                                )
                                .fetch_optional(pool.get_ref())
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to check employee uniqueness for company: {}", e);
                                    error::ErrorInternalServerError("Error checking employee uniqueness")
                                })?;

                                if existing_employee.is_some() {
                                    return Err(error::ErrorBadRequest("Сотрудник с такими паспортными данными уже существует в этой компании"));
                                }
                            }
                        }

                        // Убедимся, что user_id установлен
                        let user_id = form.user_id.unwrap_or(owner_info.user_id);

                        // Обновляем сотрудника
                        let employee_result = sqlx::query!(
                            r#"
                            UPDATE unique_employees 
                            SET last_name = $1, first_name = $2, middle_name = $3, citizenship_id = $4, 
                                position = $5, passport_series_number = $6, patent_number = $7, 
                                other_permission = $8, organization_id = $9, company_id = $10, user_id = $11
                            WHERE id = $12
                            RETURNING id, last_name, first_name, middle_name, citizenship_id, position,
                                      passport_series_number, patent_number, other_permission,
                                      organization_id, company_id, user_id, status, created_at
                            "#,
                            form.last_name,
                            form.first_name,
                            form.middle_name,
                            form.citizenship_id,
                            form.position,
                            form.passport_series_number,
                            form.patent_number,
                            form.other_permission,
                            form.organization_id,
                            form.company_id,
                            user_id,
                            employee_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to update unique employee: {}", e);
                            error::ErrorInternalServerError("Error updating employee")
                        })?;

                        let updated_employee = UniqueEmployee {
                            id: employee_result.id,
                            last_name: employee_result.last_name,
                            first_name: employee_result.first_name,
                            middle_name: employee_result.middle_name,
                            citizenship_id: employee_result.citizenship_id,
                            position: employee_result.position,
                            passport_series_number: employee_result.passport_series_number,
                            patent_number: employee_result.patent_number,
                            other_permission: employee_result.other_permission,
                            organization_id: employee_result.organization_id,
                            company_id: employee_result.company_id,
                            user_id: employee_result.user_id,
                            status: employee_result.status.unwrap_or(false),
                            created_at: employee_result.created_at,
                        };

                        log::info!("Successfully updated employee: {:?}", updated_employee);

                        Ok(HttpResponse::Ok().json(updated_employee))
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

/// Удаление сотрудника
pub async fn delete_unique_employee(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let employee_id = path.into_inner();
                        let owner_info = get_employee_owner_info(&pool, &claims.sub).await?;

                        // Проверяем права на удаление
                        let employee = sqlx::query!(
                            "SELECT user_id, organization_id, company_id FROM unique_employees WHERE id = $1",
                            employee_id
                        )
                        .fetch_optional(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch employee: {}", e);
                            error::ErrorInternalServerError("Error fetching employee")
                        })?;

                        if employee.is_none() {
                            return Err(error::ErrorNotFound("Employee not found"));
                        }

                        let employee_data = employee.unwrap();
                        let can_delete = employee_data.user_id == Some(owner_info.user_id) ||
                                       (employee_data.organization_id.is_some() && employee_data.organization_id == owner_info.organization_id) ||
                                       (employee_data.company_id.is_some() && employee_data.company_id == owner_info.company_id);

                        if !can_delete {
                            return Err(error::ErrorForbidden("You don't have permission to delete this employee"));
                        }

                        let result = sqlx::query!(
                            "DELETE FROM unique_employees WHERE id = $1",
                            employee_id
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete employee: {}", e);
                            error::ErrorInternalServerError("Error deleting employee")
                        })?;

                        if result.rows_affected() > 0 {
                            Ok(HttpResponse::Ok().json(json!({"message": "Employee deleted successfully"})))
                        } else {
                            Err(error::ErrorNotFound("Employee not found"))
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
pub async fn get_employee_ownership_info(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let owner_info = get_employee_owner_info(&pool, &claims.sub).await?;
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