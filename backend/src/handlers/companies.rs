use actix_web::{web, HttpResponse, HttpRequest, Error, error, Responder};
use sqlx::PgPool;
use serde_json::json;
use log;

use crate::models::companies::{
    Company, CompanyWithUsers, NewCompany, CompanyUser, 
    UpdateCompanyUsersRequest, UpdateCompanyUnloadPlacesRequest,
    CompanyUnloadPlace, CompanyWithUsersExtended, CompanyTable,
    UpdateCompanyTablesRequest
};
use crate::auth::decode_token;

/// Получить все компании
pub async fn get_all_companies(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        Company,
        "SELECT id, name FROM companies ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(companies) => HttpResponse::Ok().json(companies),
        Err(e) => {
            log::error!("Failed to fetch companies: {}", e);
            HttpResponse::InternalServerError().json("Error fetching companies")
        }
    }
}

/// Получить компании с количеством пользователей
pub async fn get_companies_with_users(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        CompanyWithUsers,
        r#"
        SELECT c.id, c.name, COUNT(u.id) as "user_count!"
        FROM companies c
        LEFT JOIN users u ON u.company_id = c.id
        GROUP BY c.id
        ORDER BY c.name
        "#
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(companies) => HttpResponse::Ok().json(companies),
        Err(e) => {
            log::error!("Failed to fetch companies with users: {}", e);
            HttpResponse::InternalServerError().json("Error fetching companies")
        }
    }
}

/// Получить компании с расширенной информацией (включая места разгрузки)
pub async fn get_companies_with_users_extended(pool: web::Data<PgPool>) -> impl Responder {
    // Сначала получаем базовые данные компаний
    let companies_result = sqlx::query!(
        r#"
        SELECT 
            c.id,
            c.name,
            COUNT(u.id) as user_count
        FROM companies c
        LEFT JOIN users u ON u.company_id = c.id
        GROUP BY c.id, c.name
        ORDER BY c.name
        "#,
    )
    .fetch_all(pool.get_ref())
    .await;

    match companies_result {
        Ok(companies) => {
            let mut result = Vec::new();
            
            for company in companies {
                // Для каждой компании получаем места разгрузки
                let places = sqlx::query_as!(
                    CompanyUnloadPlace,
                    r#"
                    SELECT up.id, up.name, up.description
                    FROM unload_places up
                    JOIN companies_unload_places cup ON up.id = cup.unload_place_id
                    WHERE cup.company_id = $1
                    ORDER BY up.name
                    "#,
                    company.id
                )
                .fetch_all(pool.get_ref())
                .await
                .unwrap_or_default(); // В случае ошибки возвращаем пустой вектор
                
                result.push(serde_json::json!({
                    "id": company.id,
                    "name": company.name,
                    "user_count": company.user_count,
                    "unload_places": places
                }));
            }
            
            HttpResponse::Ok().json(result)
        },
        Err(e) => {
            log::error!("Failed to fetch companies with extended info: {}", e);
            HttpResponse::InternalServerError().json("Error fetching companies")
        }
    }
}

/// Получение таблиц компании
pub async fn get_company_tables(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let company_id = path.into_inner();
    
    match sqlx::query_as!(
        CompanyTable,
        r#"
        SELECT st.id, st.name, st.display_name, st.table_type
        FROM system_tables st
        JOIN companies_tables ct ON st.id = ct.table_id
        WHERE ct.company_id = $1 AND st.is_active = true
        ORDER BY st.display_name
        "#,
        company_id
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(tables) => HttpResponse::Ok().json(tables),
        Err(e) => {
            log::error!("Failed to fetch company tables: {}", e);
            HttpResponse::InternalServerError().json("Error fetching company tables")
        }
    }
}

/// Обновление таблиц компании
pub async fn update_company_tables(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<UpdateCompanyTablesRequest>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;
    
    let company_id = path.into_inner();
    
    // Начинаем транзакцию
    let mut tx = pool.begin().await.map_err(|e| {
        log::error!("Failed to begin transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    // Удаляем старые связи
    sqlx::query!(
        "DELETE FROM companies_tables WHERE company_id = $1",
        company_id
    )
    .execute(&mut *tx)
    .await
    .map_err(|e| {
        log::error!("Failed to delete old company tables: {}", e);
        error::ErrorInternalServerError("Error updating company tables")
    })?;
    
    // Добавляем новые связи
    for table_id in &form.table_ids {
        sqlx::query!(
            "INSERT INTO companies_tables (company_id, table_id) VALUES ($1, $2)",
            company_id,
            table_id
        )
        .execute(&mut *tx)
        .await
        .map_err(|e| {
            log::error!("Failed to insert company table: {}", e);
            error::ErrorInternalServerError("Error updating company tables")
        })?;
    }
    
    // Коммитим транзакцию
    tx.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    Ok(HttpResponse::Ok().json(json!({"message": "Company tables updated successfully"})))
}

/// Получение ответственных пользователей компании
pub async fn get_company_users(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(_claims) => {
                        let company_id = path.into_inner();

                        let users = sqlx::query_as!(
                            CompanyUser,
                            r#"
                            SELECT 
                                u.id,
                                u.username,
                                u.last_name,
                                u.first_name,
                                u.middle_name,
                                u.position,
                                cu.is_primary as "is_primary?"
                            FROM users u
                            INNER JOIN companies_users cu ON u.id = cu.user_id
                            WHERE cu.company_id = $1
                            ORDER BY cu.is_primary DESC, u.last_name, u.first_name
                            "#,
                            company_id
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch company users: {}", e);
                            error::ErrorInternalServerError("Error fetching company users")
                        })?;

                        Ok(HttpResponse::Ok().json(users))
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

/// Обновление ответственных пользователей компании
pub async fn update_company_users(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
    form: web::Json<UpdateCompanyUsersRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(_claims) => {
                        let company_id = path.into_inner();

                        // Проверяем, что только один пользователь назначен главным
                        let primary_users_count = form.users.iter()
                            .filter(|user| user.is_primary.unwrap_or(false))
                            .count();
                        
                        if primary_users_count > 1 {
                            return Err(error::ErrorBadRequest(
                                "Только один пользователь может быть главным ответственным"
                            ));
                        }

                        // Начинаем транзакцию
                        let mut transaction = pool.begin().await.map_err(|e| {
                            log::error!("Failed to begin transaction: {}", e);
                            error::ErrorInternalServerError("Database error")
                        })?;

                        // Удаляем старых пользователей компании
                        sqlx::query!(
                            "DELETE FROM companies_users WHERE company_id = $1",
                            company_id
                        )
                        .execute(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete old company users: {}", e);
                            error::ErrorInternalServerError("Error updating company users")
                        })?;

                        // Добавляем новых пользователей с указанием главного
                        for user_request in &form.users {
                            // Получаем ID пользователя по username
                            let user_result = sqlx::query!(
                                "SELECT id FROM users WHERE username = $1",
                                user_request.username
                            )
                            .fetch_optional(&mut *transaction)
                            .await
                            .map_err(|e| {
                                log::error!("Failed to find user by username: {}", e);
                                error::ErrorInternalServerError("Error finding user")
                            })?;

                            if let Some(user) = user_result {
                                sqlx::query!(
                                    "INSERT INTO companies_users (company_id, user_id, is_primary) VALUES ($1, $2, $3)",
                                    company_id,
                                    user.id,
                                    user_request.is_primary.unwrap_or(false)
                                )
                                .execute(&mut *transaction)
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to insert company user: {}", e);
                                    error::ErrorInternalServerError("Error updating company users")
                                })?;
                            } else {
                                log::warn!("User with username {} not found", user_request.username);
                            }
                        }

                        // Коммитим транзакцию
                        transaction.commit().await.map_err(|e| {
                            log::error!("Failed to commit transaction: {}", e);
                            error::ErrorInternalServerError("Database error")
                        })?;

                        Ok(HttpResponse::Ok().json(json!({"message": "Company users updated successfully"})))
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

/// Получение мест разгрузки компании
pub async fn get_company_unload_places(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let company_id = path.into_inner();
    
    match sqlx::query_as!(
        CompanyUnloadPlace,
        r#"
        SELECT up.id, up.name, up.description
        FROM unload_places up
        JOIN companies_unload_places cup ON up.id = cup.unload_place_id
        WHERE cup.company_id = $1 AND up.is_active = true
        ORDER BY up.name
        "#,
        company_id
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(places) => HttpResponse::Ok().json(places),
        Err(e) => {
            log::error!("Failed to fetch company unload places: {}", e);
            HttpResponse::InternalServerError().json("Error fetching company unload places")
        }
    }
}

/// Обновление мест разгрузки компании
pub async fn update_company_unload_places(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<UpdateCompanyUnloadPlacesRequest>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;
    
    let company_id = path.into_inner();
    
    // Начинаем транзакцию
    let mut tx = pool.begin().await.map_err(|e| {
        log::error!("Failed to begin transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    // Удаляем старые связи
    sqlx::query!(
        "DELETE FROM companies_unload_places WHERE company_id = $1",
        company_id
    )
    .execute(&mut *tx)
    .await
    .map_err(|e| {
        log::error!("Failed to delete old unload places: {}", e);
        error::ErrorInternalServerError("Error updating unload places")
    })?;
    
    // Добавляем новые связи
    for place_id in &form.unload_place_ids {
        sqlx::query!(
            "INSERT INTO companies_unload_places (company_id, unload_place_id) VALUES ($1, $2)",
            company_id,
            place_id
        )
        .execute(&mut *tx)
        .await
        .map_err(|e| {
            log::error!("Failed to insert unload place: {}", e);
            error::ErrorInternalServerError("Error updating unload places")
        })?;
    }
    
    // Коммитим транзакцию
    tx.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    Ok(HttpResponse::Ok().json(json!({"message": "Unload places updated successfully"})))
}

/// Создать новую компанию (требуются права buropropuskov)
pub async fn create_company(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<NewCompany>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let company = sqlx::query_as!(
        Company,
        "INSERT INTO companies (name) VALUES ($1) RETURNING id, name",
        form.name
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to create company: {}", e);
        error::ErrorInternalServerError("Error creating company")
    })?;

    Ok(HttpResponse::Ok().json(company))
}

/// Обновить компанию
pub async fn update_company(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<NewCompany>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();
    let company = sqlx::query_as!(
        Company,
        "UPDATE companies SET name = $1 WHERE id = $2 RETURNING id, name",
        form.name,
        id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to update company: {}", e);
        error::ErrorInternalServerError("Error updating company")
    })?;

    Ok(HttpResponse::Ok().json(company))
}

/// Удалить компанию (нельзя удалить если есть пользователи)
pub async fn delete_company(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();

    let user_count: i64 = sqlx::query!(
        "SELECT COUNT(*) as count FROM users WHERE company_id = $1",
        id
    )
    .fetch_one(pool.get_ref())
    .await
    .map(|r| r.count.unwrap_or(0))
    .map_err(|_| error::ErrorInternalServerError("Error checking users"))?;

    if user_count > 0 {
        return Err(error::ErrorBadRequest("Cannot delete company with users"));
    }

    sqlx::query!("DELETE FROM companies WHERE id = $1", id)
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to delete company: {}", e);
            error::ErrorInternalServerError("Error deleting company")
        })?;

    Ok(HttpResponse::Ok().json(json!({"message": "Company deleted"})))
}

/// Проверка прав (buropropuskov)
async fn check_admin_permissions(
    req: &HttpRequest,
    pool: &web::Data<PgPool>,
) -> Result<(), Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                            FROM users u
                            JOIN user_types ut ON u.type_id = ut.id
                            WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                        Ok(())
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