// handlers/attachments.rs
use actix_web::{web, HttpResponse, Responder};
use sqlx::PgPool;
use log;

use crate::models::attachments::{
    UniqueAttachment, CreateAttachmentRequest, UpdateAttachmentRequest
};

/// Получение всех активных вложений
pub async fn get_attachments(pool: web::Data<PgPool>) -> impl Responder {
    let attachments = match sqlx::query_as!(
        UniqueAttachment,
        "SELECT id, attachment_type, name, display_name, title, instruction, is_active, created_at, updated_at 
         FROM unique_attachments 
         WHERE is_active = true
         ORDER BY title, display_name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(attachments) => attachments,
        Err(e) => {
            log::error!("Failed to fetch attachments: {}", e);
            return HttpResponse::InternalServerError().json("Error fetching attachments");
        }
    };

    HttpResponse::Ok().json(attachments)
}

/// Получение вложения по ID
pub async fn get_attachment_by_id(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let attachment_id = path.into_inner();
    
    let attachment = match sqlx::query_as!(
        UniqueAttachment,
        "SELECT id, attachment_type, name, display_name, title, instruction, is_active, created_at, updated_at 
         FROM unique_attachments 
         WHERE id = $1 AND is_active = true",
        attachment_id
    )
    .fetch_optional(pool.get_ref())
    .await {
        Ok(Some(attachment)) => attachment,
        Ok(None) => {
            return HttpResponse::NotFound().json("Attachment not found");
        },
        Err(e) => {
            log::error!("Failed to fetch attachment: {}", e);
            return HttpResponse::InternalServerError().json("Error fetching attachment");
        }
    };

    HttpResponse::Ok().json(attachment)
}

/// Создание нового вложения
pub async fn create_attachment(
    pool: web::Data<PgPool>,
    attachment_data: web::Json<CreateAttachmentRequest>,
) -> impl Responder {
    // Проверяем, существует ли уже вложение с таким именем
    let existing_attachment = match sqlx::query!(
        "SELECT id FROM unique_attachments WHERE name = $1 AND is_active = true",
        attachment_data.name
    )
    .fetch_optional(pool.get_ref())
    .await {
        Ok(Some(_)) => {
            return HttpResponse::BadRequest().json("Attachment with this name already exists");
        },
        Ok(None) => {},
        Err(e) => {
            log::error!("Failed to check existing attachment: {}", e);
            return HttpResponse::InternalServerError().json("Error checking attachment existence");
        }
    };

    // Приводим title к верхнему регистру
    let title_uppercase = attachment_data.title.to_uppercase();

    let attachment_record = match sqlx::query!(
        "INSERT INTO unique_attachments (attachment_type, name, display_name, title, instruction, is_active) 
         VALUES ($1, $2, $3, $4, $5, true) 
         RETURNING id",
        attachment_data.attachment_type,
        attachment_data.name,
        attachment_data.display_name,
        title_uppercase,
        attachment_data.instruction
    )
    .fetch_one(pool.get_ref())
    .await {
        Ok(record) => record,
        Err(e) => {
            log::error!("Failed to create attachment: {}", e);
            return HttpResponse::InternalServerError().json("Error creating attachment");
        }
    };

    HttpResponse::Ok().json(serde_json::json!({
        "id": attachment_record.id,
        "message": "Вложение успешно создано"
    }))
}

/// Обновление вложения
pub async fn update_attachment(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    attachment_data: web::Json<UpdateAttachmentRequest>,
) -> impl Responder {
    let attachment_id = path.into_inner();
    
    // Проверяем существование вложения
    let existing_attachment = match sqlx::query!(
        "SELECT id FROM unique_attachments WHERE id = $1 AND is_active = true",
        attachment_id
    )
    .fetch_optional(pool.get_ref())
    .await {
        Ok(Some(_)) => {},
        Ok(None) => {
            return HttpResponse::NotFound().json("Attachment not found");
        },
        Err(e) => {
            log::error!("Failed to check attachment existence: {}", e);
            return HttpResponse::InternalServerError().json("Error checking attachment existence");
        }
    };

    // Приводим title к верхнему регистру
    let title_uppercase = attachment_data.title.to_uppercase();

    match sqlx::query!(
        "UPDATE unique_attachments 
         SET attachment_type = $1, name = $2, display_name = $3, title = $4, instruction = $5
         WHERE id = $6",
        attachment_data.attachment_type,
        attachment_data.name,
        attachment_data.display_name,
        title_uppercase,
        attachment_data.instruction,
        attachment_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() == 0 {
                return HttpResponse::NotFound().json("Вложение не найдено");
            }
        },
        Err(e) => {
            log::error!("Failed to update attachment: {}", e);
            return HttpResponse::InternalServerError().json("Error updating attachment");
        }
    }

    HttpResponse::Ok().json("Вложение успешно обновлено")
}

/// Удаление вложения (мягкое удаление)
pub async fn delete_attachment(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let attachment_id = path.into_inner();
    
    match sqlx::query!(
        "UPDATE unique_attachments SET is_active = false WHERE id = $1",
        attachment_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                HttpResponse::Ok().json("Вложение успешно удалено")
            } else {
                HttpResponse::NotFound().json("Вложение не найдено")
            }
        },
        Err(e) => {
            log::error!("Failed to delete attachment: {}", e);
            HttpResponse::InternalServerError().json("Error deleting attachment")
        }
    }
}

// Добавить новый endpoint для получения всех вложений (активных и архивных)
pub async fn get_all_attachments(pool: web::Data<PgPool>) -> impl Responder {
    let attachments = match sqlx::query_as!(
        UniqueAttachment,
        "SELECT id, attachment_type, name, display_name, title, instruction, is_active, created_at, updated_at 
         FROM unique_attachments 
         ORDER BY is_active DESC, title, display_name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(attachments) => attachments,
        Err(e) => {
            log::error!("Failed to fetch all attachments: {}", e);
            return HttpResponse::InternalServerError().json("Error fetching attachments");
        }
    };

    HttpResponse::Ok().json(attachments)
}

// Добавить endpoint для восстановления вложения
pub async fn restore_attachment(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let attachment_id = path.into_inner();
    
    match sqlx::query!(
        "UPDATE unique_attachments SET is_active = true WHERE id = $1",
        attachment_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                HttpResponse::Ok().json("Вложение успешно восстановлено")
            } else {
                HttpResponse::NotFound().json("Вложение не найдено")
            }
        },
        Err(e) => {
            log::error!("Failed to restore attachment: {}", e);
            HttpResponse::InternalServerError().json("Error restoring attachment")
        }
    }
}