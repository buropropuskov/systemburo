use actix_web::{web, HttpResponse, Responder};
use sqlx::PgPool;
use log;

use crate::models::citizenship::{
    Citizenship, CreateCitizenshipRequest, UpdateCitizenshipRequest
};

/// Получение всех гражданств
pub async fn get_citizenships(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        Citizenship,
        "SELECT id, name, icon, is_active, is_default, patent_required, created_at, updated_at 
         FROM citizenships 
         ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(citizenships) => HttpResponse::Ok().json(citizenships),
        Err(e) => {
            log::error!("Failed to fetch citizenships: {}", e);
            HttpResponse::InternalServerError().json("Error fetching citizenships")
        }
    }
}

/// Создание нового гражданства
pub async fn create_citizenship(
    pool: web::Data<PgPool>,
    citizenship_data: web::Json<CreateCitizenshipRequest>,
) -> impl Responder {
    let mut transaction = match pool.begin().await {
        Ok(transaction) => transaction,
        Err(e) => {
            log::error!("Failed to start transaction: {}", e);
            return HttpResponse::InternalServerError().json("Error starting transaction");
        }
    };

    // Если выбран как гражданство по умолчанию, снимаем этот статус у других гражданств
    if citizenship_data.is_default.unwrap_or(false) {
        if let Err(e) = sqlx::query!(
            "UPDATE citizenships SET is_default = false WHERE is_default = true"
        )
        .execute(&mut *transaction)
        .await {
            log::error!("Failed to clear default citizenships: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error clearing default citizenships");
        }
    }

    // Создаем новое гражданство
    let citizenship_record = match sqlx::query!(
        "INSERT INTO citizenships (name, icon, is_default, patent_required) 
         VALUES ($1, $2, $3, $4) 
         RETURNING id",
        citizenship_data.name,
        citizenship_data.icon,
        citizenship_data.is_default.unwrap_or(false),
        citizenship_data.patent_required.unwrap_or(false)
    )
    .fetch_one(&mut *transaction)
    .await {
        Ok(record) => record,
        Err(e) => {
            log::error!("Failed to create citizenship: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error creating citizenship");
        }
    };

    if let Err(e) = transaction.commit().await {
        log::error!("Failed to commit transaction: {}", e);
        return HttpResponse::InternalServerError().json("Error committing transaction");
    }

    HttpResponse::Ok().json(serde_json::json!({
        "id": citizenship_record.id,
        "message": "Гражданство успешно создано"
    }))
}

/// Обновление гражданства
pub async fn update_citizenship(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    citizenship_data: web::Json<UpdateCitizenshipRequest>,
) -> impl Responder {
    let citizenship_id = path.into_inner();
    
    let mut transaction = match pool.begin().await {
        Ok(transaction) => transaction,
        Err(e) => {
            log::error!("Failed to start transaction: {}", e);
            return HttpResponse::InternalServerError().json("Error starting transaction");
        }
    };

    // Если выбран как гражданство по умолчанию, снимаем этот статус у других гражданств
    if citizenship_data.is_default.unwrap_or(false) {
        if let Err(e) = sqlx::query!(
            "UPDATE citizenships SET is_default = false WHERE is_default = true AND id != $1",
            citizenship_id
        )
        .execute(&mut *transaction)
        .await {
            log::error!("Failed to clear default citizenships: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error clearing default citizenships");
        }
    }

    // Обновляем гражданство
    match sqlx::query!(
        "UPDATE citizenships 
         SET name = $1, icon = $2, is_active = $3, is_default = $4, patent_required = $5
         WHERE id = $6",
        citizenship_data.name,
        citizenship_data.icon,
        citizenship_data.is_active.unwrap_or(true),
        citizenship_data.is_default.unwrap_or(false),
        citizenship_data.patent_required.unwrap_or(false),
        citizenship_id
    )
    .execute(&mut *transaction)
    .await {
        Ok(result) => {
            if result.rows_affected() == 0 {
                let _ = transaction.rollback().await;
                return HttpResponse::NotFound().json("Гражданство не найдено");
            }
        },
        Err(e) => {
            log::error!("Failed to update citizenship: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error updating citizenship");
        }
    }

    if let Err(e) = transaction.commit().await {
        log::error!("Failed to commit transaction: {}", e);
        return HttpResponse::InternalServerError().json("Error committing transaction");
    }

    HttpResponse::Ok().json("Гражданство успешно обновлено")
}

/// Удаление гражданства
pub async fn delete_citizenship(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let citizenship_id = path.into_inner();
    
    match sqlx::query!(
        "DELETE FROM citizenships WHERE id = $1",
        citizenship_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                HttpResponse::Ok().json("Гражданство успешно удалено")
            } else {
                HttpResponse::NotFound().json("Гражданство не найдено")
            }
        },
        Err(e) => {
            log::error!("Failed to delete citizenship: {}", e);
            HttpResponse::InternalServerError().json("Error deleting citizenship")
        }
    }
}

/// Очистка всех гражданств по умолчанию (вспомогательный метод)
pub async fn clear_default_citizenships(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query!(
        "UPDATE citizenships SET is_default = false WHERE is_default = true"
    )
    .execute(pool.get_ref())
    .await {
        Ok(_) => HttpResponse::Ok().json("Все гражданства по умолчанию сброшены"),
        Err(e) => {
            log::error!("Failed to clear default citizenships: {}", e);
            HttpResponse::InternalServerError().json("Error clearing default citizenships")
        }
    }
}