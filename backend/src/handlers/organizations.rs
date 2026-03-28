use actix_web::{web, HttpResponse, HttpRequest, Error, error, Responder};
use sqlx::PgPool;
use serde_json::json;
use log;
use serde::Deserialize;
use serde::Serialize;

use crate::models::organizations::*;
use crate::auth::decode_token;

use crate::handlers::applications::update_responsible_users_for_entity;

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateOrganizationResponsibleRequest {
    pub responsible_person_id: Option<i32>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateOrganizationUnloadPlacesRequest {
    pub unload_place_ids: Vec<i32>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateOrganizationTablesRequest {
    pub table_ids: Vec<i32>,
}

/// Получение таблиц организации
pub async fn get_organization_tables(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let org_id = path.into_inner();
    
    match sqlx::query_as!(
        OrganizationTable,
        r#"
        SELECT st.id, st.name, st.display_name, st.table_type
        FROM system_tables st
        JOIN organization_tables ot ON st.id = ot.table_id
        WHERE ot.organization_id = $1 AND st.is_active = true
        ORDER BY st.display_name
        "#,
        org_id
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(tables) => HttpResponse::Ok().json(tables),
        Err(e) => {
            log::error!("Failed to fetch organization tables: {}", e);
            HttpResponse::InternalServerError().json("Error fetching organization tables")
        }
    }
}

/// Обновление таблиц организации
pub async fn update_organization_tables(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<UpdateOrganizationTablesRequest>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;
    
    let org_id = path.into_inner();
    
    // Начинаем транзакцию
    let mut tx = pool.begin().await.map_err(|e| {
        log::error!("Failed to begin transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    // Удаляем старые связи
    sqlx::query!(
        "DELETE FROM organization_tables WHERE organization_id = $1",
        org_id
    )
    .execute(&mut *tx)
    .await
    .map_err(|e| {
        log::error!("Failed to delete old organization tables: {}", e);
        error::ErrorInternalServerError("Error updating organization tables")
    })?;
    
    // Добавляем новые связи
    for table_id in &form.table_ids {
        sqlx::query!(
            "INSERT INTO organization_tables (organization_id, table_id) VALUES ($1, $2)",
            org_id,
            table_id
        )
        .execute(&mut *tx)
        .await
        .map_err(|e| {
            log::error!("Failed to insert organization table: {}", e);
            error::ErrorInternalServerError("Error updating organization tables")
        })?;
    }
    
    // Коммитим транзакцию
    tx.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    Ok(HttpResponse::Ok().json(json!({"message": "Organization tables updated successfully"})))
}

/// Получение ответственных пользователей организации
pub async fn get_organization_users(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(_claims) => {
                        let org_id = path.into_inner();

                        // В функции get_organization_users, замените запрос на:

let users = sqlx::query_as!(
    OrganizationUser,
    r#"
    SELECT 
        u.id,
        u.username,
        u.last_name,
        u.first_name,
        u.middle_name,
        u.position,
        ou.is_primary as "is_primary?",
        ou.required_approval as "required_approval?"
    FROM users u
    INNER JOIN organization_users ou ON u.id = ou.user_id
    WHERE ou.organization_id = $1
    ORDER BY ou.is_primary DESC, u.last_name, u.first_name
    "#,
    org_id
)
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch organization users: {}", e);
                            error::ErrorInternalServerError("Error fetching organization users")
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

/// Обновление ответственных пользователей организации с поддержкой обязательного согласования
pub async fn update_organization_users(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    req: HttpRequest,
    form: web::Json<UpdateOrganizationUsersRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(_claims) => {
                        let org_id = path.into_inner();

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

                        // Получаем старых пользователей для сравнения
                        let old_users = sqlx::query!(
                            "SELECT user_id, is_primary, required_approval FROM organization_users WHERE organization_id = $1",
                            org_id
                        )
                        .fetch_all(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch old organization users: {}", e);
                            error::ErrorInternalServerError("Error fetching old organization users")
                        })?;

                        // Векторы для хранения изменений
                        let mut removed_user_ids = Vec::new();
                        let mut added_users = Vec::new();
                        let mut new_user_ids = Vec::new();
                        
                        // Сначала получаем ID всех новых пользователей
                        for user_request in &form.users {
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
                                new_user_ids.push(user.id);
                            }
                        }
                        
                        // Определяем удаленных пользователей - тех, кто был в старом списке, но нет в новом
                        for old_user in &old_users {
                            if !new_user_ids.contains(&old_user.user_id) {
                                removed_user_ids.push(old_user.user_id);
                            }
                        }

                        // Удаляем старых пользователей организации
                        sqlx::query!(
                            "DELETE FROM organization_users WHERE organization_id = $1",
                            org_id
                        )
                        .execute(&mut *transaction)
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete old organization users: {}", e);
                            error::ErrorInternalServerError("Error updating organization users")
                        })?;

                        // Добавляем новых пользователей с поддержкой обязательного согласования
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
                                let is_primary = user_request.is_primary.unwrap_or(false);
                                let required_approval = user_request.required_approval.unwrap_or(false);
                                
                                sqlx::query!(
                                    "INSERT INTO organization_users (organization_id, user_id, is_primary, required_approval) VALUES ($1, $2, $3, $4)",
                                    org_id,
                                    user.id,
                                    is_primary,
                                    required_approval
                                )
                                .execute(&mut *transaction)
                                .await
                                .map_err(|e| {
                                    log::error!("Failed to insert organization user: {}", e);
                                    error::ErrorInternalServerError("Error updating organization users")
                                })?;
                                
                                // Проверяем, был ли этот пользователь в старом списке
                                let was_in_old = old_users.iter().any(|old_user| old_user.user_id == user.id);
                                if !was_in_old {
                                    // Новый пользователь
                                    added_users.push((user.id, is_primary, required_approval));
                                } else {
                                    // Пользователь был, проверяем изменился ли его статус is_primary или required_approval
                                    let old_is_primary = old_users.iter()
                                        .find(|old_user| old_user.user_id == user.id)
                                        .map(|old_user| old_user.is_primary.unwrap_or(false))
                                        .unwrap_or(false);
                                    
                                  
let old_required_approval = old_users.iter()
    .find(|old_user| old_user.user_id == user.id)
    .and_then(|old_user| Some(old_user.required_approval))
    .unwrap_or(false);
                                    
                                    if old_is_primary != is_primary || old_required_approval != required_approval {
                                        // Статус изменился - добавляем в список изменений
                                        added_users.push((user.id, is_primary, required_approval));
                                    }
                                }
                            } else {
                                log::warn!("User with username {} not found", user_request.username);
                            }
                        }

                        // Коммитим транзакцию организации
                        transaction.commit().await.map_err(|e| {
                            log::error!("Failed to commit transaction: {}", e);
                            error::ErrorInternalServerError("Database error")
                        })?;
                        
                        // Теперь обновляем заявки с новыми ответственными
                        // Создаем векторы с информацией об обязательном согласовании
                        let added_users_with_required: Vec<(i32, bool, bool)> = added_users.iter()
                            .map(|&(id, is_primary, required_approval)| (id, is_primary, required_approval))
                            .collect();
                        
                        // Вызываем обновленную функцию для обновления заявок
                        let update_result = crate::handlers::applications::update_responsible_users_for_entity_with_required(
                            pool.clone(),
                            req.clone(),
                            org_id,
                            "organization".to_string(),
                            removed_user_ids,
                            added_users_with_required,
                        ).await;
                        
                        match update_result {
                            Ok(response) => {
                                log::info!("Successfully updated organization users and related applications");
                                Ok(response)
                            }
                            Err(e) => {
                                log::error!("Failed to update applications: {}", e);
                                Ok(HttpResponse::Ok().json(json!({
                                    "message": "Organization users updated successfully",
                                    "warning": "Some applications may have outdated responsible users"
                                })))
                            }
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

pub async fn get_organization_unload_places(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let org_id = path.into_inner();
    
    match sqlx::query_as!(
        OrganizationUnloadPlace,
        r#"
        SELECT up.id, up.name, up.description
        FROM unload_places up
        JOIN organization_unload_places oup ON up.id = oup.unload_place_id
        WHERE oup.organization_id = $1 AND up.is_active = true
        ORDER BY up.name
        "#,
        org_id
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(places) => HttpResponse::Ok().json(places),
        Err(e) => {
            log::error!("Failed to fetch organization unload places: {}", e);
            HttpResponse::InternalServerError().json("Error fetching organization unload places")
        }
    }
}

/// Получить организации с расширенной информацией (включая места разгрузки)
pub async fn get_organizations_with_users_extended(pool: web::Data<PgPool>) -> impl Responder {
    // Сначала получаем базовые данные организаций
    let orgs_result = sqlx::query!(
        r#"
        SELECT 
            o.id,
            o.name,
            COUNT(u.id) as user_count
        FROM organizations o
        LEFT JOIN users u ON u.organization_id = o.id
        GROUP BY o.id, o.name
        ORDER BY o.name
        "#,
    )
    .fetch_all(pool.get_ref())
    .await;

    match orgs_result {
        Ok(orgs) => {
            let mut result = Vec::new();
            
            for org in orgs {
                // Для каждой организации получаем места разгрузки
                let places = sqlx::query_as!(
                    OrganizationUnloadPlace,
                    r#"
                    SELECT up.id, up.name, up.description
                    FROM unload_places up
                    JOIN organization_unload_places oup ON up.id = oup.unload_place_id
                    WHERE oup.organization_id = $1
                    ORDER BY up.name
                    "#,
                    org.id
                )
                .fetch_all(pool.get_ref())
                .await
                .unwrap_or_default(); // В случае ошибки возвращаем пустой вектор
                
                result.push(serde_json::json!({
                    "id": org.id,
                    "name": org.name,
                    "user_count": org.user_count,
                    "unload_places": places
                }));
            }
            
            HttpResponse::Ok().json(result)
        },
        Err(e) => {
            log::error!("Failed to fetch organizations with extended info: {}", e);
            HttpResponse::InternalServerError().json("Error fetching organizations")
        }
    }
}

/// Обновить места разгрузки организации
pub async fn update_organization_unload_places(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<UpdateOrganizationUnloadPlacesRequest>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;
    
    let org_id = path.into_inner();
    
    // Начинаем транзакцию
    let mut tx = pool.begin().await.map_err(|e| {
        log::error!("Failed to begin transaction: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;
    
    // Удаляем старые связи
    sqlx::query!(
        "DELETE FROM organization_unload_places WHERE organization_id = $1",
        org_id
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
            "INSERT INTO organization_unload_places (organization_id, unload_place_id) VALUES ($1, $2)",
            org_id,
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

/// Получение организации текущего пользователя
pub async fn get_organization(req: HttpRequest, pool: web::Data<PgPool>) -> impl Responder {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            "SELECT o.name as organization, u.organization_id 
                             FROM users u 
                             JOIN organizations o ON u.organization_id = o.id 
                             WHERE u.username = $1", 
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await;

                        match user {
                            Ok(user) => HttpResponse::Ok().json(json!({ 
                                "organization": user.organization,
                                "organization_id": user.organization_id
                            })),
                            Err(_) => HttpResponse::InternalServerError().json("Не удалось получить организацию"),
                        }
                    }
                    Err(_) => HttpResponse::Unauthorized().json("Invalid or missing token"),
                }
            } else {
                HttpResponse::Unauthorized().json("Invalid or missing token")
            }
        } else {
            HttpResponse::Unauthorized().json("Invalid or missing token")
        }
    } else {
        HttpResponse::Unauthorized().json("Missing Authorization header")
    }
}

/// Получить все организации
pub async fn get_all_organizations(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        OrganizationInfo,
        "SELECT id, name FROM organizations ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(orgs) => HttpResponse::Ok().json(orgs),
        Err(e) => {
            log::error!("Failed to fetch organizations: {}", e);
            HttpResponse::InternalServerError().json("Error fetching organizations")
        }
    }
}

/// Получить организации с количеством пользователей
pub async fn get_organizations_with_users(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        OrganizationWithUsers,
        r#"
        SELECT o.id, o.name, COUNT(u.id) as "user_count!"
        FROM organizations o
        LEFT JOIN users u ON u.organization_id = o.id
        GROUP BY o.id
        ORDER BY o.name
        "#
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(orgs) => HttpResponse::Ok().json(orgs),
        Err(e) => {
            log::error!("Failed to fetch organizations with users: {}", e);
            HttpResponse::InternalServerError().json("Error fetching organizations")
        }
    }
}

/// Создать новую организацию (требуются права buropropuskov)
pub async fn create_organization(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<NewOrganization>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let org = sqlx::query_as!(
        OrganizationInfo,
        "INSERT INTO organizations (name) VALUES ($1) RETURNING id, name",
        form.name
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to create organization: {}", e);
        error::ErrorInternalServerError("Error creating organization")
    })?;

    Ok(HttpResponse::Ok().json(org))
}

/// Обновить организацию
pub async fn update_organization(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<NewOrganization>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();
    let org = sqlx::query_as!(
        OrganizationInfo,
        "UPDATE organizations SET name = $1 WHERE id = $2 RETURNING id, name",
        form.name,
        id
    )
    .fetch_optional(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to update organization: {}", e);
        error::ErrorInternalServerError("Error updating organization")
    })?;

    match org {
        Some(org) => Ok(HttpResponse::Ok().json(org)),
        None => Err(error::ErrorNotFound("Organization not found")),
    }
}

/// Удалить организацию (нельзя удалить если есть пользователи)
pub async fn delete_organization(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();

    let user_count: i64 = sqlx::query!(
        "SELECT COUNT(*) as count FROM users WHERE organization_id = $1",
        id
    )
    .fetch_one(pool.get_ref())
    .await
    .map(|r| r.count.unwrap_or(0))
    .map_err(|_| error::ErrorInternalServerError("Error checking users"))?;

    if user_count > 0 {
        return Err(error::ErrorBadRequest("Cannot delete organization with users"));
    }

    sqlx::query!("DELETE FROM organizations WHERE id = $1", id)
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to delete organization: {}", e);
            error::ErrorInternalServerError("Error deleting organization")
        })?;

    Ok(HttpResponse::Ok().json(json!({"message": "Organization deleted"})))
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