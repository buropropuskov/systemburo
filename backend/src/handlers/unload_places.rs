use actix_web::{web, HttpResponse, Responder};
use sqlx::PgPool;
use log;
use serde::Deserialize;

use crate::models::unload_places::UnloadPlace;

#[derive(Debug, Deserialize)]
pub struct CreateUnloadPlaceRequest {
    pub name: String,
    pub description: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateUnloadPlaceRequest {
    pub name: String,
    pub description: Option<String>,
}

/// Получение всех доступных мест разгрузки
pub async fn get_unload_places(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        UnloadPlace,
        "SELECT id, name, description, is_active FROM unload_places ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(places) => HttpResponse::Ok().json(places),
        Err(e) => {
            log::error!("Failed to fetch unload places: {}", e);
            HttpResponse::InternalServerError().json("Error fetching unload places")
        }
    }
}

/// Создание нового места разгрузки
pub async fn create_unload_place(
    pool: web::Data<PgPool>,
    place_data: web::Json<CreateUnloadPlaceRequest>,
) -> impl Responder {
    match sqlx::query!(
        "INSERT INTO unload_places (name, description, is_active) VALUES ($1, $2, true) RETURNING id",
        place_data.name,
        place_data.description
    )
    .fetch_one(pool.get_ref())
    .await {
        Ok(record) => HttpResponse::Ok().json(serde_json::json!({
            "id": record.id,
            "message": "Место разгрузки успешно создано"
        })),
        Err(e) => {
            log::error!("Failed to create unload place: {}", e);
            HttpResponse::InternalServerError().json("Error creating unload place")
        }
    }
}

/// Обновление места разгрузки
pub async fn update_unload_place(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    place_data: web::Json<UpdateUnloadPlaceRequest>,
) -> impl Responder {
    let place_id = path.into_inner();
    
    match sqlx::query!(
        "UPDATE unload_places SET name = $1, description = $2 WHERE id = $3",
        place_data.name,
        place_data.description,
        place_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                HttpResponse::Ok().json("Место разгрузки успешно обновлено")
            } else {
                HttpResponse::NotFound().json("Место разгрузки не найдено")
            }
        },
        Err(e) => {
            log::error!("Failed to update unload place: {}", e);
            HttpResponse::InternalServerError().json("Error updating unload place")
        }
    }
}

/// Удаление места разгрузки (полное удаление с проверкой связей)
pub async fn delete_unload_place(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let place_id = path.into_inner();
    
    // Проверяем, используется ли место разгрузки в организациях
    let org_count: i64 = match sqlx::query!(
        "SELECT COUNT(*) as count FROM organization_unload_places WHERE unload_place_id = $1",
        place_id
    )
    .fetch_one(pool.get_ref())
    .await {
        Ok(record) => record.count.unwrap_or(0),
        Err(e) => {
            log::error!("Failed to check organization unload places: {}", e);
            return HttpResponse::InternalServerError().json("Error checking organization dependencies");
        }
    };
    
    // Проверяем, используется ли место разгрузки в компаниях
    let company_count: i64 = match sqlx::query!(
        "SELECT COUNT(*) as count FROM companies_unload_places WHERE unload_place_id = $1",
        place_id
    )
    .fetch_one(pool.get_ref())
    .await {
        Ok(record) => record.count.unwrap_or(0),
        Err(e) => {
            log::error!("Failed to check company unload places: {}", e);
            return HttpResponse::InternalServerError().json("Error checking company dependencies");
        }
    };
    
    // Если место разгрузки привязано к организациям или компаниям, не удаляем
    if org_count > 0 || company_count > 0 {
        let mut error_message = String::from("Невозможно удалить место разгрузки, так как оно привязано к: ");
        let mut parts = Vec::new();
        
        if org_count > 0 {
            parts.push(format!("организациям ({})", org_count));
        }
        if company_count > 0 {
            parts.push(format!("компаниям ({})", company_count));
        }
        
        error_message.push_str(&parts.join(" и "));
        return HttpResponse::BadRequest().json(serde_json::json!({
            "message": error_message
        }));
    }
    
    // Если связей нет, удаляем место разгрузки
    match sqlx::query!(
        "DELETE FROM unload_places WHERE id = $1",
        place_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                HttpResponse::Ok().json("Место разгрузки успешно удалено")
            } else {
                HttpResponse::NotFound().json("Место разгрузки не найдено")
            }
        },
        Err(e) => {
            log::error!("Failed to delete unload place: {}", e);
            HttpResponse::InternalServerError().json("Error deleting unload place")
        }
    }
}