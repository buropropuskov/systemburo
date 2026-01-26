// handlers/applications.rs
use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row, postgres::PgQueryResult};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};
use chrono::{DateTime, Utc, NaiveDateTime, NaiveDate, NaiveTime};

use crate::models::applications::{Application, ApplicationWithDetails, ApplicationFilter, ApplicationCreateRequest, ApplicationUpdateRequest};
use crate::auth::decode_token;

// Структура для полной заявки с вложениями
#[derive(Debug, Deserialize)]
pub struct CompleteApplicationRequest {
    pub message: Option<String>,
    pub organization: String,
    pub company: Option<String>,
    pub responsible_person: String,
    pub contact_phone: String,
    pub data_approval: bool,
    pub attachments: Vec<AttachmentData>,
}

#[derive(Debug, Deserialize)]
pub struct AttachmentData {
    pub attachment_type: String,
    pub attachment_name: String,
    pub attachment_display_name: String,
    pub entry_date_from: Option<String>,
    pub entry_date_to: Option<String>,
    pub entry_time_from: Option<String>,
    pub entry_time_to: Option<String>,
    pub data: AttachmentContentData,
}

#[derive(Debug, Deserialize)]
pub struct AttachmentContentData {
    pub vehicles: Option<Vec<VehicleData>>,
    pub employees: Option<Vec<EmployeeData>>,
    pub items: Option<Vec<ItemData>>,
}

#[derive(Debug, Deserialize)]
pub struct VehicleData {
    pub car_number: String,
    pub car_brand: String,
    pub unload_place: Option<String>,
    pub unload_places: Vec<i32>,
}

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

#[derive(Debug, Deserialize)]
pub struct ItemData {
    pub name: String,
    pub count: i32,
    pub order_index: i32,
}

// Структура для ответа
#[derive(Debug, Serialize)]
pub struct CompleteApplicationResponse {
    pub success: bool,
    pub message: String,
    pub application_id: i32,
    pub application_number: String,
}

/// Получение всех заявок с фильтрами
pub async fn get_applications(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    filter: web::Query<ApplicationFilter>,
) -> Result<HttpResponse, Error> {
    // Проверка токена
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    log::info!("Getting applications with filters");

    let mut query = String::from(
        "SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
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
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || ru.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || ru.middle_name
                    ELSE ''
                END
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE 1=1"
    );

    let mut params: Vec<String> = Vec::new();
    let mut param_counter = 1;

    // Фильтр по поиску
    if let Some(ref search) = filter.search_query {
        if !search.is_empty() {
            query.push_str(&format!(" AND (
                a.application_number ILIKE ${} OR
                COALESCE(o.name, c.name, '') ILIKE ${} OR
                c.name ILIKE ${} OR
                a.message ILIKE ${} OR
                a.status ILIKE ${} OR
                a.confirmation ILIKE ${}
            )", param_counter, param_counter + 1, param_counter + 2, param_counter + 3, param_counter + 4, param_counter + 5));
            for _ in 0..6 {
                params.push(format!("%{}%", search));
            }
            param_counter += 6;
        }
    }

    // Фильтр по организации
    if let Some(org_id) = filter.organization_id {
        query.push_str(&format!(" AND a.organization_id = ${}", param_counter));
        params.push(org_id.to_string());
        param_counter += 1;
    }

    // Фильтр по компании
    if let Some(company_id) = filter.company_id {
        query.push_str(&format!(" AND a.company_id = ${}", param_counter));
        params.push(company_id.to_string());
        param_counter += 1;
    }

    // Фильтр по подтверждению
    if let Some(ref confirmation) = filter.confirmation {
        query.push_str(&format!(" AND a.confirmation = ${}", param_counter));
        params.push(confirmation.clone());
        param_counter += 1;
    }

    // Фильтр по статусу
    if let Some(ref status) = filter.status {
        query.push_str(&format!(" AND a.status = ${}", param_counter));
        params.push(status.clone());
        param_counter += 1;
    }

    // Фильтр по дате
    if let Some(date_from) = filter.date_from {
        query.push_str(&format!(" AND a.sending_datetime >= ${}", param_counter));
        params.push(date_from.and_hms_opt(0, 0, 0).unwrap().to_string());
        param_counter += 1;
    }

    if let Some(date_to) = filter.date_to {
        query.push_str(&format!(" AND a.sending_datetime <= ${}", param_counter));
        params.push(date_to.and_hms_opt(23, 59, 59).unwrap().to_string());
        param_counter += 1;
    }

    query.push_str(" ORDER BY a.sending_datetime DESC");

    log::debug!("SQL query: {}", query);
    log::debug!("Params: {:?}", params);

    let mut query_builder = sqlx::query(&query);
    
    for param in &params {
        query_builder = query_builder.bind(param);
    }

    let rows = query_builder
        .fetch_all(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to fetch applications: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

    let applications: Vec<ApplicationWithDetails> = rows.iter().map(|row| {
        // Получаем DateTime<Utc> из БД
        let sending_datetime: DateTime<Utc> = row.try_get("sending_datetime")
            .unwrap_or_else(|_| Utc::now());
        
        let reading_datetime: Option<DateTime<Utc>> = row.try_get("reading_datetime").ok();
        let confirmation_datetime: Option<DateTime<Utc>> = row.try_get("confirmation_datetime").ok();

        ApplicationWithDetails {
            id: row.try_get("id").unwrap_or(0),
            application_number: row.try_get("application_number").unwrap_or_default(),
            confirmation: row.try_get("confirmation").unwrap_or_default(),
            sending_datetime,
            reading_datetime,
            confirmation_datetime,
            organization_id: row.try_get("organization_id").unwrap_or(0),
            organization_name: row.try_get("organization_name").unwrap_or_default(),
            company_id: row.try_get("company_id").ok(),
            company_name: row.try_get("company_name").unwrap_or_default(),
            sender_user_id: row.try_get("sender_user_id").unwrap_or(0),
            sender_full_name: row.try_get("sender_full_name").ok(),
            sender_name: row.try_get("sender_name").unwrap_or_default(),
            message: row.try_get("message").ok(),
            status: row.try_get("status").unwrap_or_default(),
            responsible_user_id: row.try_get("responsible_user_id").ok(),
            responsible_full_name: row.try_get("responsible_full_name").ok(),
            responsible_name: row.try_get("responsible_name").unwrap_or_default(),
            responsible_comment: row.try_get("responsible_comment").ok(),
            data_approval: row.try_get("data_approval").unwrap_or(false),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(applications))
}

/// Создание новой заявки
pub async fn create_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<ApplicationCreateRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let username = &claims.sub;
    
    // Получаем ID пользователя из базы данных
    let user_row = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    if !form.data_approval {
        return Err(error::ErrorBadRequest("Data approval is required"));
    }

    log::info!("Creating new application for user: {}", user_id);

    let now_utc = Utc::now();
    let today_local = now_utc.date_naive();
    let date_part = today_local.format("%Y%m%d").to_string();
    
    let count_result = sqlx::query!(
        "SELECT COUNT(*) as count FROM applications WHERE DATE(sending_datetime AT TIME ZONE 'UTC') = $1",
        today_local
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to count applications: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application_number = format!("№ {}/{:03}", date_part, count_result.count.unwrap_or(0) + 1);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    let application_result = sqlx::query!(
        r#"
        INSERT INTO applications (
            application_number, 
            organization_id, 
            company_id,
            sender_user_id, 
            message, 
            data_approval,
            status,
            confirmation,
            sending_datetime
        )
        VALUES ($1, $2, $3, $4, $5, $6, 'Непрочитано', 'Согласование', $7)
        RETURNING id, application_number
        "#,
        application_number,
        form.organization_id,
        form.company_id,
        user_id,
        form.message,
        form.data_approval,
        now_utc
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to create application: {}", e);
        error::ErrorInternalServerError("Error creating application")
    })?;

    let application_id = application_result.id;

    // Получаем ответственных пользователей для организации и компании
    let mut responsible_users = Vec::new();
    let mut primary_responsible_id: Option<i32> = None;

    if let Some(org_id) = form.organization_id {
        // Получаем ответственных для организации
        let org_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary
            FROM organization_users
            WHERE organization_id = $1
            "#,
            org_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch organization responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching organization responsibles")
        })?;

        for row in org_responsibles {
            // Преобразуем Option<bool> в bool, по умолчанию false
            let is_primary = row.is_primary.unwrap_or(false);
            responsible_users.push((row.user_id, is_primary));
            if is_primary {
                primary_responsible_id = Some(row.user_id);
            }
        }
    }

    if let Some(company_id) = form.company_id {
        // Получаем ответственных для компании
        let company_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary
            FROM companies_users
            WHERE company_id = $1
            "#,
            company_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch company responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching company responsibles")
        })?;

        for row in company_responsibles {
            // Проверяем, не добавлен ли уже этот пользователь из организации
            let exists = responsible_users.iter().any(|&(user_id, _)| user_id == row.user_id);
            if !exists {
                // Преобразуем Option<bool> в bool, по умолчанию false
                let is_primary = row.is_primary.unwrap_or(false);
                responsible_users.push((row.user_id, is_primary));
                if is_primary && primary_responsible_id.is_none() {
                    primary_responsible_id = Some(row.user_id);
                }
            }
        }
    }

    // Обновляем поле responsible_user_id в заявке (главный ответственный)
    if let Some(primary_id) = primary_responsible_id {
        sqlx::query!(
            "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
            primary_id,
            application_id
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to update primary responsible: {}", e);
            error::ErrorInternalServerError("Error updating primary responsible")
        })?;
    }

    // Добавляем всех ответственных в новую таблицу
    for (user_id, is_primary) in responsible_users {
        sqlx::query!(
            r#"
            INSERT INTO application_responsible_users (application_id, user_id, is_primary)
            VALUES ($1, $2, $3)
            ON CONFLICT (application_id, user_id) DO NOTHING
            "#,
            application_id,
            user_id,
            is_primary
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to insert responsible user: {}", e);
            error::ErrorInternalServerError("Error inserting responsible user")
        })?;
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    log::info!("Successfully created application with ID: {}", application_result.id);

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application created successfully",
        "application_id": application_result.id,
        "application_number": application_result.application_number
    })))
}

/// Создание полной заявки с вложениями
pub async fn submit_complete_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<CompleteApplicationRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let username = &claims.sub;
    
    // Получаем ID пользователя из базы данных
    let user_row = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    if !form.data_approval {
        return Err(error::ErrorBadRequest("Data approval is required"));
    }

    if form.attachments.is_empty() {
        return Err(error::ErrorBadRequest("At least one attachment is required"));
    }

    log::info!("Creating complete application for user: {}", user_id);

    // Начинаем транзакцию
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // 1. Создаем заявку в таблице applications
    let now_utc = Utc::now();
    let today_local = now_utc.date_naive();
    let date_part = today_local.format("%Y%m%d").to_string();
    
    let count_result = sqlx::query!(
        "SELECT COUNT(*) as count FROM applications WHERE DATE(sending_datetime AT TIME ZONE 'UTC') = $1",
        today_local
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to count applications: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application_number = format!("№ {}/{:03}", date_part, count_result.count.unwrap_or(0) + 1);

    // Получаем ID организации по имени
    let organization_row = sqlx::query!(
        "SELECT id FROM organizations WHERE name = $1",
        form.organization
    )
    .fetch_optional(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch organization: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let organization_id = match organization_row {
        Some(row) => Some(row.id),
        None => None
    };

    // Получаем ID компании по имени (если указана)
    let company_id = if let Some(company_name) = &form.company {
        let company_row = sqlx::query!(
            "SELECT id FROM companies WHERE name = $1",
            company_name
        )
        .fetch_optional(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch company: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;
        
        company_row.map(|row| row.id)
    } else {
        None
    };

    let application_result = sqlx::query!(
        r#"
        INSERT INTO applications (
            application_number, 
            organization_id, 
            company_id,
            sender_user_id, 
            message, 
            data_approval,
            status,
            confirmation,
            sending_datetime
        )
        VALUES ($1, $2, $3, $4, $5, $6, 'Непрочитано', 'Согласование', $7)
        RETURNING id, application_number
        "#,
        application_number,
        organization_id,
        company_id,
        user_id,
        form.message,
        form.data_approval,
        now_utc
    )
    .fetch_one(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to create application: {}", e);
        error::ErrorInternalServerError("Error creating application")
    })?;

    let application_id = application_result.id;

    // Получаем ответственных пользователей для организации и компании
    let mut responsible_users = Vec::new();
    let mut primary_responsible_id: Option<i32> = None;

    if let Some(org_id) = organization_id {
        // Получаем ответственных для организации
        let org_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary
            FROM organization_users
            WHERE organization_id = $1
            "#,
            org_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch organization responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching organization responsibles")
        })?;

        for row in org_responsibles {
            // Преобразуем Option<bool> в bool, по умолчанию false
            let is_primary = row.is_primary.unwrap_or(false);
            responsible_users.push((row.user_id, is_primary));
            if is_primary {
                primary_responsible_id = Some(row.user_id);
            }
        }
    }

    if let Some(comp_id) = company_id {
        // Получаем ответственных для компании
        let company_responsibles = sqlx::query!(
            r#"
            SELECT user_id, is_primary
            FROM companies_users
            WHERE company_id = $1
            "#,
            comp_id
        )
        .fetch_all(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to fetch company responsibles: {}", e);
            error::ErrorInternalServerError("Error fetching company responsibles")
        })?;

        for row in company_responsibles {
            // Проверяем, не добавлен ли уже этот пользователь из организации
            let exists = responsible_users.iter().any(|&(user_id, _)| user_id == row.user_id);
            if !exists {
                // Преобразуем Option<bool> в bool, по умолчанию false
                let is_primary = row.is_primary.unwrap_or(false);
                responsible_users.push((row.user_id, is_primary));
                if is_primary && primary_responsible_id.is_none() {
                    primary_responsible_id = Some(row.user_id);
                }
            }
        }
    }

    // Обновляем поле responsible_user_id в заявке (главный ответственный)
    if let Some(primary_id) = primary_responsible_id {
        sqlx::query!(
            "UPDATE applications SET responsible_user_id = $1 WHERE id = $2",
            primary_id,
            application_id
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to update primary responsible: {}", e);
            error::ErrorInternalServerError("Error updating primary responsible")
        })?;
    }

    // Добавляем всех ответственных в новую таблицу
    for (user_id, is_primary) in responsible_users {
        sqlx::query!(
            r#"
            INSERT INTO application_responsible_users (application_id, user_id, is_primary)
            VALUES ($1, $2, $3)
            ON CONFLICT (application_id, user_id) DO NOTHING
            "#,
            application_id,
            user_id,
            is_primary
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to insert responsible user: {}", e);
            error::ErrorInternalServerError("Error inserting responsible user")
        })?;
    }

    // 2. Создаем вложения для заявки
    for attachment in &form.attachments {
        // Конвертируем строки дат и времени
        let entry_date_from: Option<NaiveDate> = attachment.entry_date_from.as_ref()
            .and_then(|s| NaiveDate::parse_from_str(s, "%Y-%m-%d").ok());
        
        let entry_date_to: Option<NaiveDate> = attachment.entry_date_to.as_ref()
            .and_then(|s| NaiveDate::parse_from_str(s, "%Y-%m-%d").ok());
        
        let entry_time_from: Option<NaiveTime> = attachment.entry_time_from.as_ref()
            .and_then(|s| NaiveTime::parse_from_str(s, "%H:%M:%S").ok());
        
        let entry_time_to: Option<NaiveTime> = attachment.entry_time_to.as_ref()
            .and_then(|s| NaiveTime::parse_from_str(s, "%H:%M:%S").ok());

        let attachment_result = sqlx::query!(
            r#"
            INSERT INTO attachments (
                application_id,
                attachment_type,
                attachment_name,
                attachment_display_name,
                entry_date_from,
                entry_date_to,
                entry_time_from,
                entry_time_to
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
            RETURNING id
            "#,
            application_id,
            attachment.attachment_type,
            attachment.attachment_name,
            attachment.attachment_display_name,
            entry_date_from,
            entry_date_to,
            entry_time_from,
            entry_time_to
        )
        .fetch_one(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Failed to create attachment: {}", e);
            error::ErrorInternalServerError("Error creating attachment")
        })?;

        let attachment_id = attachment_result.id;

        // 3. Создаем данные в зависимости от типа вложения
        match attachment.attachment_type.as_str() {
            "cars" => {
                if let Some(vehicles) = &attachment.data.vehicles {
                    for vehicle in vehicles {
                        let car_result = sqlx::query!(
                            r#"
                            INSERT INTO cars (
                                attachment_id,
                                car_number,
                                car_brand,
                                unload_place,
                                entry_date_from,
                                entry_time_from,
                                entry_date_to,
                                entry_time_to,
                                status
                            )
                            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)
                            RETURNING id
                            "#,
                            attachment_id,
                            vehicle.car_number,
                            vehicle.car_brand,
                            vehicle.unload_place.as_deref(),
                            entry_date_from,
                            entry_time_from,
                            entry_date_to,
                            entry_time_to
                        )
                        .fetch_one(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create car: {}", e);
                            error::ErrorInternalServerError("Error creating car")
                        })?;

                        let car_id = car_result.id;

                        // Создаем связи с местами разгрузки
                        for &place_id in &vehicle.unload_places {
                            sqlx::query!(
                                r#"
                                INSERT INTO car_unload_places (car_id, unload_place_id, order_index)
                                VALUES ($1, $2, 1)
                                "#,
                                car_id,
                                place_id
                            )
                            .execute(&mut *transaction)
                            .await
                            .map_err(|e| {
                                log::error!("Failed to create car unload place: {}", e);
                                error::ErrorInternalServerError("Error creating car unload place")
                            })?;
                        }
                    }
                }
            }
            "people" => {
                if let Some(employees) = &attachment.data.employees {
                    for employee in employees {
                        let employee_result = sqlx::query!(
                            r#"
                            INSERT INTO employees (
                                attachment_id,
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
                            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)
                            RETURNING id
                            "#,
                            attachment_id,
                            employee.last_name,
                            employee.first_name,
                            employee.middle_name.as_deref(),
                            employee.citizenship_id,
                            employee.position,
                            employee.passport_series_number,
                            employee.patent_number.as_deref(),
                            employee.other_permission.as_deref()
                        )
                        .fetch_one(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create employee: {}", e);
                            error::ErrorInternalServerError("Error creating employee")
                        })?;

                        let employee_id = employee_result.id;

                        // Создаем связи с целевыми таблицами
                        for &table_id in &employee.target_tables {
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
                    }
                }
            }
            "items" => {
                if let Some(items) = &attachment.data.items {
                    for item in items {
                        let now_utc_date = now_utc.date_naive();
                        
                        sqlx::query!(
                            r#"
                            INSERT INTO items (
                                attachment_id,
                                name,
                                count,
                                date_created
                            )
                            VALUES ($1, $2, $3, $4)
                            "#,
                            attachment_id,
                            item.name,
                            item.count,
                            now_utc_date
                        )
                        .execute(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to create item: {}", e);
                            error::ErrorInternalServerError("Error creating item")
                        })?;
                    }
                }
            }
            _ => {
                return Err(error::ErrorBadRequest("Invalid attachment type"));
            }
        }
    }

    // Фиксируем транзакцию
    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    log::info!("Successfully created complete application with ID: {}", application_id);

    let response = CompleteApplicationResponse {
        success: true,
        message: "Application created successfully".to_string(),
        application_id,
        application_number: application_result.application_number,
    };

    Ok(HttpResponse::Ok().json(response))
}

/// Получение ответственных пользователей для заявки
pub async fn get_application_responsible_users(
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

    let application_id = path.into_inner();

    log::info!("Getting responsible users for application: {}", application_id);

    #[derive(Debug, sqlx::FromRow)]
    struct DbResponsibleUser {
        id: i32,
        username: String,
        last_name: Option<String>,
        first_name: Option<String>,
        middle_name: Option<String>,
        position: Option<String>,
        is_primary: bool,
    }

    let responsibles = sqlx::query_as!(
        DbResponsibleUser,
        r#"
        SELECT 
            u.id,
            u.username,
            u.last_name,
            u.first_name,
            u.middle_name,
            u.position,
            COALESCE(aru.is_primary, false) as "is_primary!"
        FROM application_responsible_users aru
        JOIN users u ON aru.user_id = u.id
        WHERE aru.application_id = $1
        ORDER BY aru.is_primary DESC, u.last_name, u.first_name
        "#,
        application_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch responsible users: {}", e);
        error::ErrorInternalServerError("Error fetching responsible users")
    })?;

    // Преобразуем в ResponsibleUserInfo
    use crate::models::applications::ResponsibleUserInfo;
    let responsibles_info: Vec<ResponsibleUserInfo> = responsibles.iter().map(|row| {
        ResponsibleUserInfo {
            id: row.id,
            username: row.username.clone(),
            last_name: row.last_name.clone(),
            first_name: row.first_name.clone(),
            middle_name: row.middle_name.clone(),
            position: row.position.clone(),
            is_primary: row.is_primary,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(responsibles_info))
}

/// Обновление заявки (подтверждение, статус и т.д.)
/// Обновление заявки (подтверждение, статус и т.д.)
pub async fn update_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    update_data: web::Json<ApplicationUpdateRequest>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let username = &claims.sub;
    
    // Получаем ID пользователя из базы данных
    let user_row = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };
    
    let application_id = path.into_inner();

    log::info!("Updating application {} by user {}", application_id, user_id);

    // Получаем текущее время в UTC
    let now_utc = Utc::now();

    // Строим динамический запрос с типизированными параметрами
    let mut query_parts: Vec<String> = Vec::new();
    let mut query = String::from("UPDATE applications SET ");
    let mut param_counter = 1;
    
    // Собираем параметры с правильными типами
    if let Some(ref confirmation) = update_data.confirmation {
        query_parts.push(format!("confirmation = ${}", param_counter));
        param_counter += 1;
        
        if confirmation == "Согласовано" || confirmation == "Не согласовано" {
            query_parts.push(format!("confirmation_datetime = ${}", param_counter));
            param_counter += 1;
        }
    }

    if let Some(ref status) = update_data.status {
        query_parts.push(format!("status = ${}", param_counter));
        param_counter += 1;
        
        if status == "В обработке" {
            query_parts.push(format!("reading_datetime = ${}", param_counter));
            param_counter += 1;
        }
    }

    if let Some(ref comment) = update_data.responsible_comment {
        query_parts.push(format!("responsible_comment = ${}", param_counter));
        param_counter += 1;
        query_parts.push(format!("responsible_user_id = ${}", param_counter));
        param_counter += 1;
    }

    if query_parts.is_empty() {
        return Err(error::ErrorBadRequest("No data to update"));
    }

    query.push_str(&query_parts.join(", "));
    query.push_str(&format!(" WHERE id = ${}", param_counter));

    log::debug!("Update query: {}", query);

    // Создаем query builder и добавляем параметры с правильными типами
    let mut query_builder = sqlx::query(&query);
    
    // Добавляем параметры в правильном порядке
    let mut param_index = 1;
    
    if let Some(ref confirmation) = update_data.confirmation {
        query_builder = query_builder.bind(confirmation);
        param_index += 1;
        
        if confirmation == "Согласовано" || confirmation == "Не согласовано" {
            query_builder = query_builder.bind(now_utc);
            param_index += 1;
        }
    }

    if let Some(ref status) = update_data.status {
        query_builder = query_builder.bind(status);
        param_index += 1;
        
        if status == "В обработке" {
            query_builder = query_builder.bind(now_utc);
            param_index += 1;
        }
    }

    if let Some(ref comment) = update_data.responsible_comment {
        query_builder = query_builder.bind(comment);
        param_index += 1;
        query_builder = query_builder.bind(user_id);
        param_index += 1;
    }

    // Добавляем ID заявки
    query_builder = query_builder.bind(application_id);

    let result = query_builder
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to update application: {:?}", e);
            error::ErrorInternalServerError(format!("Error updating application: {}", e))
        })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application updated successfully",
        "rows_affected": result.rows_affected()
    })))
}
/// Получение заявки по ID с расширенной информацией (включая ответственных)
pub async fn get_application_by_id(
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

    let application_id = path.into_inner();

    log::info!("Getting application by ID: {}", application_id);

    // Получаем основную информацию о заявке
    let application_row = sqlx::query!(
        r#"
        SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
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
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || ru.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || ru.middle_name
                    ELSE ''
                END
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE a.id = $1
        "#,
        application_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application = match application_row {
        Some(r) => {
            // Получаем DateTime<Utc> из БД
            let sending_datetime: DateTime<Utc> = r.sending_datetime;
            let reading_datetime: Option<DateTime<Utc>> = r.reading_datetime;
            let confirmation_datetime: Option<DateTime<Utc>> = r.confirmation_datetime;

            // Получаем список всех ответственных для этой заявки
            #[derive(Debug, sqlx::FromRow)]
            struct DbResponsibleUser {
                id: i32,
                username: String,
                last_name: Option<String>,
                first_name: Option<String>,
                middle_name: Option<String>,
                position: Option<String>,
                is_primary: bool,
            }

            let responsibles = sqlx::query_as!(
                DbResponsibleUser,
                r#"
                SELECT 
                    u.id,
                    u.username,
                    u.last_name,
                    u.first_name,
                    u.middle_name,
                    u.position,
                    COALESCE(aru.is_primary, false) as "is_primary!"
                FROM application_responsible_users aru
                JOIN users u ON aru.user_id = u.id
                WHERE aru.application_id = $1
                ORDER BY aru.is_primary DESC, u.last_name, u.first_name
                "#,
                application_id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());

            // Преобразуем в ResponsibleUserInfo
            use crate::models::applications::ResponsibleUserInfo;
            let responsibles_info: Vec<ResponsibleUserInfo> = responsibles.iter().map(|row| {
                ResponsibleUserInfo {
                    id: row.id,
                    username: row.username.clone(),
                    last_name: row.last_name.clone(),
                    first_name: row.first_name.clone(),
                    middle_name: row.middle_name.clone(),
                    position: row.position.clone(),
                    is_primary: row.is_primary,
                }
            }).collect();

            let application_with_details = ApplicationWithDetails {
                id: r.id,
                application_number: r.application_number,
                confirmation: r.confirmation,
                sending_datetime,
                reading_datetime,
                confirmation_datetime,
                organization_id: r.organization_id,
                organization_name: r.organization_name.unwrap_or_default(),
                company_id: r.company_id,
                company_name: r.company_name,
                sender_user_id: r.sender_user_id,
                sender_full_name: r.sender_full_name,
                sender_name: r.sender_name.unwrap_or_default(),
                message: r.message,
                status: r.status,
                responsible_user_id: r.responsible_user_id,
                responsible_full_name: r.responsible_full_name,
                responsible_name: r.responsible_name.unwrap_or_default(),
                responsible_comment: r.responsible_comment,
                data_approval: r.data_approval,
            };

            // Создаем объект с полной информацией о заявке
            let mut response = serde_json::to_value(application_with_details)
                .map_err(|e| {
                    log::error!("Failed to serialize application: {}", e);
                    error::ErrorInternalServerError("Error serializing application")
                })?;
            
            // Добавляем список ответственных в ответ
            if let serde_json::Value::Object(ref mut map) = response {
                map.insert("responsible_users".to_string(), serde_json::to_value(responsibles_info)
                    .map_err(|e| {
                        log::error!("Failed to serialize responsibles: {}", e);
                        error::ErrorInternalServerError("Error serializing responsibles")
                    })?);
            }

            response
        },
        None => return Err(error::ErrorNotFound("Application not found")),
    };

    Ok(HttpResponse::Ok().json(application))
}

/// Получение заявок текущего пользователя
pub async fn get_user_applications(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    filter: web::Query<ApplicationFilter>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let username = &claims.sub;

    log::info!("Getting applications for user: {}", username);

    // Получаем ID пользователя из базы данных
    let user_row = sqlx::query!(
        "SELECT id FROM users WHERE username = $1",
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let user_id = match user_row {
        Some(row) => row.id,
        None => return Err(error::ErrorUnauthorized("User not found")),
    };

    let mut query = String::from(
        "SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
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
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || ru.first_name
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || ru.middle_name
                    ELSE ''
                END
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE 1=1"
    );

    let mut params: Vec<String> = Vec::new();
    let mut param_counter = 1;

    // Этот endpoint возвращает ВСЕ заявки, фильтрация по вкладкам происходит на фронтенде
    // Это позволяет использовать один endpoint для всех вкладок

    if let Some(ref search) = filter.search_query {
        if !search.is_empty() {
            query.push_str(&format!(" AND (
                a.application_number ILIKE ${} OR
                COALESCE(o.name, c.name, '') ILIKE ${} OR
                c.name ILIKE ${} OR
                a.message ILIKE ${} OR
                a.status ILIKE ${} OR
                a.confirmation ILIKE ${} OR
                u.last_name ILIKE ${} OR
                u.first_name ILIKE ${} OR
                u.middle_name ILIKE ${} OR
                ru.last_name ILIKE ${} OR
                ru.first_name ILIKE ${} OR
                ru.middle_name ILIKE ${}
            )", param_counter, param_counter + 1, param_counter + 2, param_counter + 3, param_counter + 4, param_counter + 5, 
               param_counter + 6, param_counter + 7, param_counter + 8, param_counter + 9, param_counter + 10, param_counter + 11));
            for _ in 0..12 {
                params.push(format!("%{}%", search));
            }
            param_counter += 12;
        }
    }

    if let Some(ref confirmation) = filter.confirmation {
        query.push_str(&format!(" AND a.confirmation = ${}", param_counter));
        params.push(confirmation.clone());
        param_counter += 1;
    }

    if let Some(ref status) = filter.status {
        query.push_str(&format!(" AND a.status = ${}", param_counter));
        params.push(status.clone());
        param_counter += 1;
    }

    if let Some(date_from) = filter.date_from {
        query.push_str(&format!(" AND a.sending_datetime >= ${}", param_counter));
        params.push(date_from.and_hms_opt(0, 0, 0).unwrap().to_string());
        param_counter += 1;
    }

    if let Some(date_to) = filter.date_to {
        query.push_str(&format!(" AND a.sending_datetime <= ${}", param_counter));
        params.push(date_to.and_hms_opt(23, 59, 59).unwrap().to_string());
        param_counter += 1;
    }

    query.push_str(" ORDER BY a.sending_datetime DESC");

    log::debug!("SQL query for user applications: {}", query);

    let mut query_builder = sqlx::query(&query);
    
    for param in &params {
        query_builder = query_builder.bind(param);
    }

    let rows = query_builder
        .fetch_all(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to fetch user applications: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

    let applications: Vec<ApplicationWithDetails> = rows.iter().map(|row| {
        // Получаем DateTime<Utc> из БД
        let sending_datetime: DateTime<Utc> = row.try_get("sending_datetime")
            .unwrap_or_else(|_| Utc::now());
        
        let reading_datetime: Option<DateTime<Utc>> = row.try_get("reading_datetime").ok();
        let confirmation_datetime: Option<DateTime<Utc>> = row.try_get("confirmation_datetime").ok();

        ApplicationWithDetails {
            id: row.try_get("id").unwrap_or(0),
            application_number: row.try_get("application_number").unwrap_or_default(),
            confirmation: row.try_get("confirmation").unwrap_or_default(),
            sending_datetime,
            reading_datetime,
            confirmation_datetime,
            organization_id: row.try_get("organization_id").unwrap_or(0),
            organization_name: row.try_get("organization_name").unwrap_or_default(),
            company_id: row.try_get("company_id").ok(),
            company_name: row.try_get("company_name").unwrap_or_default(),
            sender_user_id: row.try_get("sender_user_id").unwrap_or(0),
            sender_full_name: row.try_get("sender_full_name").ok(),
            sender_name: row.try_get("sender_name").unwrap_or_default(),
            message: row.try_get("message").ok(),
            status: row.try_get("status").unwrap_or_default(),
            responsible_user_id: row.try_get("responsible_user_id").ok(),
            responsible_full_name: row.try_get("responsible_full_name").ok(),
            responsible_name: row.try_get("responsible_name").unwrap_or_default(),
            responsible_comment: row.try_get("responsible_comment").ok(),
            data_approval: row.try_get("data_approval").unwrap_or(false),
        }
    }).collect();

    Ok(HttpResponse::Ok().json(applications))
}

/// Получение вложений для заявки
pub async fn get_application_attachments(
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

    let application_id = path.into_inner();

    log::info!("Getting attachments for application: {}", application_id);

    #[derive(Debug, Serialize)]
    struct AttachmentInfo {
        id: i32,
        attachment_type: String,
        attachment_name: String,
        attachment_display_name: String,
        entry_date_from: Option<NaiveDate>,
        entry_date_to: Option<NaiveDate>,
        entry_time_from: Option<NaiveTime>,
        entry_time_to: Option<NaiveTime>,
        created_at: Option<NaiveDateTime>,
    }

    let rows = sqlx::query!(
        r#"
        SELECT 
            id,
            attachment_type,
            attachment_name,
            attachment_display_name,
            entry_date_from,
            entry_date_to,
            entry_time_from,
            entry_time_to,
            created_at
        FROM attachments 
        WHERE application_id = $1
        ORDER BY created_at
        "#,
        application_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch attachments: {}", e);
        error::ErrorInternalServerError("Error fetching attachments")
    })?;

    let attachments: Vec<AttachmentInfo> = rows.iter().map(|row| {
        AttachmentInfo {
            id: row.id,
            attachment_type: row.attachment_type.clone(),
            attachment_name: row.attachment_name.clone(),
            attachment_display_name: row.attachment_display_name.clone().unwrap_or_default(),
            entry_date_from: row.entry_date_from,
            entry_date_to: row.entry_date_to,
            entry_time_from: row.entry_time_from,
            entry_time_to: row.entry_time_to,
            created_at: row.created_at,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(attachments))
}

/// Получение автомобилей для вложения
pub async fn get_attachment_cars(
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

    let attachment_id = path.into_inner();

    #[derive(Debug, Serialize)]
    struct UnloadPlaceInfo {
        id: i32,
        name: String,
        description: Option<String>,
    }

    #[derive(Debug, Serialize)]
    struct CarWithPlaces {
        id: i32,
        car_number: String,
        car_brand: String,
        unload_place: Option<String>,
        entry_date_from: Option<NaiveDate>,
        entry_time_from: Option<NaiveTime>,
        entry_date_to: Option<NaiveDate>,
        entry_time_to: Option<NaiveTime>,
        unload_places: Vec<UnloadPlaceInfo>,
    }

    let cars = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.car_number,
            c.car_brand,
            c.unload_place,
            c.entry_date_from,
            c.entry_time_from,
            c.entry_date_to,
            c.entry_time_to
        FROM cars c
        WHERE c.attachment_id = $1
        "#,
        attachment_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch cars: {}", e);
        error::ErrorInternalServerError("Error fetching cars")
    })?;

    let mut car_with_places: Vec<CarWithPlaces> = Vec::new();

    for car in cars {
        let places = sqlx::query!(
            r#"
            SELECT up.id, up.name, up.description
            FROM car_unload_places cup
            JOIN unload_places up ON cup.unload_place_id = up.id
            WHERE cup.car_id = $1
            ORDER BY cup.order_index
            "#,
            car.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());

        let place_infos: Vec<UnloadPlaceInfo> = places.iter().map(|p| {
            UnloadPlaceInfo {
                id: p.id,
                name: p.name.clone(),
                description: p.description.clone(),
            }
        }).collect();

        car_with_places.push(CarWithPlaces {
            id: car.id,
            car_number: car.car_number,
            car_brand: car.car_brand,
            unload_place: car.unload_place,
            entry_date_from: Some(car.entry_date_from),
            entry_time_from: Some(car.entry_time_from),
            entry_date_to: Some(car.entry_date_to),
            entry_time_to: Some(car.entry_time_to),
            unload_places: place_infos,
        });
    }

    Ok(HttpResponse::Ok().json(car_with_places))
}

/// Получение сотрудников для вложения
pub async fn get_attachment_employees(
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

    let attachment_id = path.into_inner();

    #[derive(Debug, Serialize)]
    struct TableInfo {
        id: i32,
        name: String,
        display_name: String,
    }

    #[derive(Debug, Serialize)]
    struct EmployeeWithTables {
        id: i32,
        last_name: String,
        first_name: String,
        middle_name: Option<String>,
        position: Option<String>,
        citizenship_id: Option<i32>,
        passport_series_number: Option<String>,
        patent_number: Option<String>,
        other_permission: Option<String>,
        target_tables: Vec<TableInfo>,
    }

    let employees = sqlx::query!(
        r#"
        SELECT 
            e.id,
            e.last_name,
            e.first_name,
            e.middle_name,
            e.position,
            e.citizenship_id,
            e.passport_series_number,
            e.patent_number,
            e.other_permission
        FROM employees e
        WHERE e.attachment_id = $1
        "#,
        attachment_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch employees: {}", e);
        error::ErrorInternalServerError("Error fetching employees")
    })?;

    let mut employee_with_tables: Vec<EmployeeWithTables> = Vec::new();

    for employee in employees {
        let tables = sqlx::query!(
            r#"
            SELECT st.id, st.name, st.display_name
            FROM employee_target_tables ett
            JOIN system_tables st ON ett.table_id = st.id
            WHERE ett.employee_id = $1
            ORDER BY ett.order_index
            "#,
            employee.id
        )
        .fetch_all(pool.get_ref())
        .await
        .unwrap_or_else(|_| Vec::new());

        let table_infos: Vec<TableInfo> = tables.iter().map(|t| {
            TableInfo {
                id: t.id,
                name: t.name.clone(),
                display_name: t.display_name.clone(),
            }
        }).collect();

        employee_with_tables.push(EmployeeWithTables {
            id: employee.id,
            last_name: employee.last_name,
            first_name: employee.first_name,
            middle_name: employee.middle_name,
            position: employee.position,
            citizenship_id: employee.citizenship_id,
            passport_series_number: employee.passport_series_number,
            patent_number: employee.patent_number,
            other_permission: employee.other_permission,
            target_tables: table_infos,
        });
    }

    Ok(HttpResponse::Ok().json(employee_with_tables))
}

/// Получение ТМЦ для вложения
pub async fn get_attachment_items(
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

    let attachment_id = path.into_inner();

    #[derive(Debug, Serialize)]
    struct ItemInfo {
        id: i32,
        name: String,
        count: i32,
        date_created: Option<NaiveDate>,
    }

    let items = sqlx::query!(
        r#"
        SELECT id, name, count, date_created
        FROM items
        WHERE attachment_id = $1
        ORDER BY id
        "#,
        attachment_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch items: {}", e);
        error::ErrorInternalServerError("Error fetching items")
    })?;

    let item_infos: Vec<ItemInfo> = items.iter().map(|row| {
        ItemInfo {
            id: row.id,
            name: row.name.clone(),
            count: row.count,
            date_created: row.date_created,
        }
    }).collect();

    Ok(HttpResponse::Ok().json(item_infos))
}

/// Получение информации о текущем пользователе
/* pub async fn get_current_user(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    let username = &claims.sub;

    #[derive(Debug, Serialize)]
    struct UserInfo {
        id: i32,
        username: String,
        last_name: Option<String>,
        first_name: Option<String>,
        middle_name: Option<String>,
        position: Option<String>,
    }

    let user = sqlx::query_as!(
        UserInfo,
        r#"
        SELECT id, username, last_name, first_name, middle_name, position
        FROM users
        WHERE username = $1
        "#,
        username
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    match user {
        Some(user) => Ok(HttpResponse::Ok().json(user)),
        None => Err(error::ErrorUnauthorized("User not found")),
    }
} */

/// Получение заявки по ID с расширенной информацией (включая ответственных и вложения)
pub async fn get_application_details(
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

    let application_id = path.into_inner();

    log::info!("Getting application details by ID: {}", application_id);

    // Получаем основную информацию о заявке
    let application_row = sqlx::query!(
        r#"
        SELECT 
            a.*,
            COALESCE(o.name, c.name) as organization_name,
            c.name as company_name,
            CONCAT(
                COALESCE(u.last_name, ''), ' ',
                COALESCE(u.first_name, ''), ' ',
                COALESCE(u.middle_name, '')
            ) as sender_full_name,
            CONCAT(
                COALESCE(u.last_name, ''),
                CASE 
                    WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN 
                        ' ' || LEFT(u.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(u.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as sender_name,
            CONCAT(
                COALESCE(ru.last_name, ''), ' ',
                COALESCE(ru.first_name, ''), ' ',
                COALESCE(ru.middle_name, '')
            ) as responsible_full_name,
            CONCAT(
                COALESCE(ru.last_name, ''),
                CASE 
                    WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN 
                        ' ' || LEFT(ru.first_name, 1) || '.'
                    ELSE ''
                END,
                CASE 
                    WHEN ru.middle_name IS NOT NULL AND u.middle_name != '' THEN 
                        ' ' || LEFT(ru.middle_name, 1) || '.'
                    ELSE ''
                END
            ) as responsible_name
        FROM applications a
        LEFT JOIN organizations o ON a.organization_id = o.id
        LEFT JOIN companies c ON a.company_id = c.id
        LEFT JOIN users u ON a.sender_user_id = u.id
        LEFT JOIN users ru ON a.responsible_user_id = ru.id
        WHERE a.id = $1
        "#,
        application_id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch application: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let application = match application_row {
        Some(r) => {
            // Получаем список всех ответственных для этой заявки
            #[derive(Debug, Serialize)]
            struct ResponsibleUser {
                id: i32,
                username: String,
                last_name: Option<String>,
                first_name: Option<String>,
                middle_name: Option<String>,
                position: Option<String>,
                is_primary: bool,
            }

            let responsibles = sqlx::query_as!(
                ResponsibleUser,
                r#"
                SELECT 
                    u.id,
                    u.username,
                    u.last_name,
                    u.first_name,
                    u.middle_name,
                    u.position,
                    COALESCE(aru.is_primary, false) as "is_primary!"
                FROM application_responsible_users aru
                JOIN users u ON aru.user_id = u.id
                WHERE aru.application_id = $1
                ORDER BY aru.is_primary DESC, u.last_name, u.first_name
                "#,
                application_id
            )
            .fetch_all(pool.get_ref())
            .await
            .unwrap_or_else(|_| Vec::new());

            #[derive(Debug, Serialize)]
            struct ApplicationDetails {
                id: i32,
                application_number: String,
                confirmation: String,
                sending_datetime: DateTime<Utc>,
                reading_datetime: Option<DateTime<Utc>>,
                confirmation_datetime: Option<DateTime<Utc>>,
                organization_id: i32,
                organization_name: String,
                company_id: Option<i32>,
                company_name: String,
                sender_user_id: i32,
                sender_full_name: Option<String>,
                sender_name: String,
                message: Option<String>,
                status: String,
                responsible_user_id: Option<i32>,
                responsible_full_name: Option<String>,
                responsible_name: String,
                responsible_comment: Option<String>,
                data_approval: bool,
                responsible_users: Vec<ResponsibleUser>,
            }

            let details = ApplicationDetails {
                id: r.id,
                application_number: r.application_number,
                confirmation: r.confirmation,
                sending_datetime: r.sending_datetime,
                reading_datetime: r.reading_datetime,
                confirmation_datetime: r.confirmation_datetime,
                organization_id: r.organization_id,
                organization_name: r.organization_name.unwrap_or_default(),
                company_id: r.company_id,
                company_name: r.company_name,
                sender_user_id: r.sender_user_id,
                sender_full_name: r.sender_full_name,
                sender_name: r.sender_name.unwrap_or_default(),
                message: r.message,
                status: r.status,
                responsible_user_id: r.responsible_user_id,
                responsible_full_name: r.responsible_full_name,
                responsible_name: r.responsible_name.unwrap_or_default(),
                responsible_comment: r.responsible_comment,
                data_approval: r.data_approval,
                responsible_users: responsibles,
            };

            serde_json::to_value(details)
                .map_err(|e| {
                    log::error!("Failed to serialize application: {}", e);
                    error::ErrorInternalServerError("Error serializing application")
                })?
        },
        None => return Err(error::ErrorNotFound("Application not found")),
    };

    Ok(HttpResponse::Ok().json(application))
}



// Структура для обновления статуса машины
#[derive(Debug, Deserialize)]
pub struct UpdateCarStatusRequest {
    pub status: i32,
}

// Структура для обновления статуса сотрудника
#[derive(Debug, Deserialize)]
pub struct UpdateEmployeeStatusRequest {
    pub status: i32,
}

/// Обновление статуса машины
pub async fn update_car_status(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    update_data: web::Json<UpdateCarStatusRequest>,
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

    log::info!("Updating car {} status to {}", car_id, update_data.status);

    match sqlx::query!(
        "UPDATE cars SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
        update_data.status,
        car_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                Ok(HttpResponse::Ok().json(json!({
                    "success": true,
                    "message": "Car status updated successfully"
                })))
            } else {
                Ok(HttpResponse::NotFound().json(json!({
                    "success": false,
                    "message": "Car not found"
                })))
            }
        },
        Err(e) => {
            log::error!("Failed to update car status: {}", e);
            Err(error::ErrorInternalServerError("Error updating car status"))
        }
    }
}

/// Обновление статуса сотрудника
pub async fn update_employee_status(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    update_data: web::Json<UpdateEmployeeStatusRequest>,
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

    log::info!("Updating employee {} status to {}", employee_id, update_data.status);

    match sqlx::query!(
        "UPDATE employees SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
        update_data.status,
        employee_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                Ok(HttpResponse::Ok().json(json!({
                    "success": true,
                    "message": "Employee status updated successfully"
                })))
            } else {
                Ok(HttpResponse::NotFound().json(json!({
                    "success": false,
                    "message": "Employee not found"
                })))
            }
        },
        Err(e) => {
            log::error!("Failed to update employee status: {}", e);
            Err(error::ErrorInternalServerError("Error updating employee status"))
        }
    }
}

/// Обновление статусов всех машин и сотрудников в заявке
pub async fn update_application_items_status(
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

    let application_id = path.into_inner();

    log::info!("Updating statuses for all items in application {}", application_id);

    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Получаем все вложения заявки
    let attachments = sqlx::query!(
        "SELECT id, attachment_type FROM attachments WHERE application_id = $1",
        application_id
    )
    .fetch_all(&mut *transaction)
    .await
    .map_err(|e| {
        log::error!("Failed to fetch attachments: {}", e);
        error::ErrorInternalServerError("Error fetching attachments")
    })?;

    for attachment in attachments {
        match attachment.attachment_type.as_str() {
            "cars" => {
                // Обновляем статусы машин
                match sqlx::query!(
                    "UPDATE cars SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = $1",
                    attachment.id
                )
                .execute(&mut *transaction)
                .await {
                    Ok(result) => {
                        log::info!("Updated {} cars for attachment {}", result.rows_affected(), attachment.id);
                    },
                    Err(e) => {
                        log::error!("Failed to update cars status: {}", e);
                    }
                }
            },
            "people" => {
                // Обновляем статусы сотрудников
                match sqlx::query!(
                    "UPDATE employees SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = $1",
                    attachment.id
                )
                .execute(&mut *transaction)
                .await {
                    Ok(result) => {
                        log::info!("Updated {} employees for attachment {}", result.rows_affected(), attachment.id);
                    },
                    Err(e) => {
                        log::error!("Failed to update employees status: {}", e);
                    }
                }
            },
            _ => {} // Для других типов вложений ничего не делаем
        }
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "All items statuses updated successfully"
    })))
}