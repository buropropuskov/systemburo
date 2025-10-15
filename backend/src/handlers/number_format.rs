use actix_web::{web, HttpResponse, Responder};
use sqlx::PgPool;
use log;

use crate::models::number_format::{
    LicensePlateFormat, LicensePlateFormatCell, LicensePlateFormatWithCells,
    CreateLicensePlateFormatRequest, UpdateLicensePlateFormatRequest
};

/// Получение всех форматов номеров с их клетками
pub async fn get_license_plate_formats(pool: web::Data<PgPool>) -> impl Responder {
    // Сначала получаем все форматы
    let formats = match sqlx::query_as!(
        LicensePlateFormat,
        "SELECT id, name, country_code, icon, is_active, is_default, created_at 
         FROM license_plate_formats 
         WHERE is_active = true
         ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(formats) => formats,
        Err(e) => {
            log::error!("Failed to fetch license plate formats: {}", e);
            return HttpResponse::InternalServerError().json("Error fetching license plate formats");
        }
    };

    // Для каждого формата получаем его клетки
    let mut formats_with_cells = Vec::new();
    
    for format in formats {
        let cells = match sqlx::query_as!(
            LicensePlateFormatCell,
            "SELECT id, format_id, cell_order, cell_type, min_length, max_length, 
                    allowed_letters, alphabet_type, language, padding_char, padding_side, created_at
             FROM license_plate_format_cells 
             WHERE format_id = $1 
             ORDER BY cell_order",
            format.id
        )
        .fetch_all(pool.get_ref())
        .await {
            Ok(cells) => cells,
            Err(e) => {
                log::error!("Failed to fetch format cells for format {}: {}", format.id, e);
                continue; // Пропускаем формат если не удалось загрузить клетки
            }
        };

        formats_with_cells.push(LicensePlateFormatWithCells {
            format,
            cells,
        });
    }

    HttpResponse::Ok().json(formats_with_cells)
}

/// Создание нового формата номеров с клетками
pub async fn create_license_plate_format(
    pool: web::Data<PgPool>,
    format_data: web::Json<CreateLicensePlateFormatRequest>,
) -> impl Responder {
    let mut transaction = match pool.begin().await {
        Ok(transaction) => transaction,
        Err(e) => {
            log::error!("Failed to start transaction: {}", e);
            return HttpResponse::InternalServerError().json("Error starting transaction");
        }
    };

    // Если выбран как формат по умолчанию, снимаем этот статус у других форматов
    if format_data.is_default.unwrap_or(false) {
        if let Err(e) = sqlx::query!(
            "UPDATE license_plate_formats SET is_default = false WHERE is_default = true"
        )
        .execute(&mut *transaction)
        .await {
            log::error!("Failed to clear default formats: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error clearing default formats");
        }
    }

    // Создаем основной формат
    let format_record = match sqlx::query!(
        "INSERT INTO license_plate_formats (name, country_code, icon, is_active, is_default) 
         VALUES ($1, $2, $3, true, $4) 
         RETURNING id",
        format_data.name,
        format_data.country_code,
        format_data.icon,
        format_data.is_default.unwrap_or(false)
    )
    .fetch_one(&mut *transaction)
    .await {
        Ok(record) => record,
        Err(e) => {
            log::error!("Failed to create license plate format: {}", e);
            return HttpResponse::InternalServerError().json("Error creating license plate format");
        }
    };

    // Создаем клетки формата
    for cell_data in &format_data.cells {
        match sqlx::query!(
            "INSERT INTO license_plate_format_cells 
             (format_id, cell_order, cell_type, min_length, max_length, 
              allowed_letters, alphabet_type, language, padding_char, padding_side) 
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
            format_record.id,
            cell_data.cell_order,
            cell_data.cell_type,
            cell_data.min_length,
            cell_data.max_length,
            cell_data.allowed_letters,
            cell_data.alphabet_type,
            cell_data.language,
            cell_data.padding_char.as_deref().unwrap_or("0"),
            cell_data.padding_side.as_deref().unwrap_or("left")
        )
        .execute(&mut *transaction)
        .await {
            Ok(_) => {},
            Err(e) => {
                log::error!("Failed to create format cell: {}", e);
                let _ = transaction.rollback().await;
                return HttpResponse::InternalServerError().json("Error creating format cells");
            }
        }
    }

    if let Err(e) = transaction.commit().await {
        log::error!("Failed to commit transaction: {}", e);
        return HttpResponse::InternalServerError().json("Error committing transaction");
    }

    HttpResponse::Ok().json(serde_json::json!({
        "id": format_record.id,
        "message": "Формат номеров успешно создан"
    }))
}

/// Обновление формата номеров
pub async fn update_license_plate_format(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    format_data: web::Json<UpdateLicensePlateFormatRequest>,
) -> impl Responder {
    let format_id = path.into_inner();
    
    let mut transaction = match pool.begin().await {
        Ok(transaction) => transaction,
        Err(e) => {
            log::error!("Failed to start transaction: {}", e);
            return HttpResponse::InternalServerError().json("Error starting transaction");
        }
    };

    // Если выбран как формат по умолчанию, снимаем этот статус у других форматов
    if format_data.is_default.unwrap_or(false) {
        if let Err(e) = sqlx::query!(
            "UPDATE license_plate_formats SET is_default = false WHERE is_default = true AND id != $1",
            format_id
        )
        .execute(&mut *transaction)
        .await {
            log::error!("Failed to clear default formats: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error clearing default formats");
        }
    }

    // Обновляем основной формат
    match sqlx::query!(
        "UPDATE license_plate_formats 
         SET name = $1, country_code = $2, icon = $3, is_default = $4
         WHERE id = $5",
        format_data.name,
        format_data.country_code,
        format_data.icon,
        format_data.is_default.unwrap_or(false),
        format_id
    )
    .execute(&mut *transaction)
    .await {
        Ok(result) => {
            if result.rows_affected() == 0 {
                let _ = transaction.rollback().await;
                return HttpResponse::NotFound().json("Формат номеров не найден");
            }
        },
        Err(e) => {
            log::error!("Failed to update license plate format: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error updating license plate format");
        }
    }

    // Удаляем старые клетки
    match sqlx::query!(
        "DELETE FROM license_plate_format_cells WHERE format_id = $1",
        format_id
    )
    .execute(&mut *transaction)
    .await {
        Ok(_) => {},
        Err(e) => {
            log::error!("Failed to delete old format cells: {}", e);
            let _ = transaction.rollback().await;
            return HttpResponse::InternalServerError().json("Error updating format cells");
        }
    }

    // Создаем новые клетки
    for cell_data in &format_data.cells {
        match sqlx::query!(
            "INSERT INTO license_plate_format_cells 
             (format_id, cell_order, cell_type, min_length, max_length, 
              allowed_letters, alphabet_type, language, padding_char, padding_side) 
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
            format_id,
            cell_data.cell_order,
            cell_data.cell_type,
            cell_data.min_length,
            cell_data.max_length,
            cell_data.allowed_letters,
            cell_data.alphabet_type,
            cell_data.language,
            cell_data.padding_char.as_deref().unwrap_or("0"),
            cell_data.padding_side.as_deref().unwrap_or("left")
        )
        .execute(&mut *transaction)
        .await {
            Ok(_) => {},
            Err(e) => {
                log::error!("Failed to create format cell: {}", e);
                let _ = transaction.rollback().await;
                return HttpResponse::InternalServerError().json("Error creating format cells");
            }
        }
    }

    if let Err(e) = transaction.commit().await {
        log::error!("Failed to commit transaction: {}", e);
        return HttpResponse::InternalServerError().json("Error committing transaction");
    }

    HttpResponse::Ok().json("Формат номеров успешно обновлен")
}

/// Удаление формата номеров
pub async fn delete_license_plate_format(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let format_id = path.into_inner();
    
    // Проверяем, используется ли формат в заявках (нужно добавить проверку когда будет таблица cars/applications)
    // Пока просто удаляем
    
    match sqlx::query!(
        "DELETE FROM license_plate_formats WHERE id = $1",
        format_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                HttpResponse::Ok().json("Формат номеров успешно удален")
            } else {
                HttpResponse::NotFound().json("Формат номеров не найден")
            }
        },
        Err(e) => {
            log::error!("Failed to delete license plate format: {}", e);
            HttpResponse::InternalServerError().json("Error deleting license plate format")
        }
    }
}