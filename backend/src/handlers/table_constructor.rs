// handlers/table_constructor.rs
use actix_web::{web, HttpResponse, Responder};
use sqlx::PgPool;
use log;

use crate::models::table_constructor::{
    SystemTable, TableField, SystemTableWithFields,
    CreateSystemTableRequest, UpdateSystemTableRequest
};

/// Получение всех системных таблиц
pub async fn get_system_tables(pool: web::Data<PgPool>) -> impl Responder {
    let tables = match sqlx::query_as!(
        SystemTable,
        "SELECT id, name, display_name, table_type, show_fact_table, fact_table_hint, instruction, is_active, created_at, updated_at 
         FROM system_tables 
         WHERE is_active = true
         ORDER BY display_name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(tables) => tables,
        Err(e) => {
            log::error!("Failed to fetch system tables: {}", e);
            return HttpResponse::InternalServerError().json("Error fetching system tables");
        }
    };

    HttpResponse::Ok().json(tables)
}

/// Получение таблицы по имени
pub async fn get_system_table_by_name(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
) -> impl Responder {
    let table_name = path.into_inner();
    
    let table = match sqlx::query_as!(
        SystemTable,
        "SELECT id, name, display_name, table_type, show_fact_table, fact_table_hint, instruction, is_active, created_at, updated_at 
         FROM system_tables 
         WHERE name = $1 AND is_active = true",
        table_name
    )
    .fetch_optional(pool.get_ref())
    .await {
        Ok(Some(table)) => table,
        Ok(None) => {
            return HttpResponse::NotFound().json("Table not found");
        },
        Err(e) => {
            log::error!("Failed to fetch system table: {}", e);
            return HttpResponse::InternalServerError().json("Error fetching system table");
        }
    };

    HttpResponse::Ok().json(table)
}

/// Создание новой системной таблицы
pub async fn create_system_table(
    pool: web::Data<PgPool>,
    table_data: web::Json<CreateSystemTableRequest>,
) -> impl Responder {
    // Проверяем, существует ли уже таблица с таким именем
    let existing_table = match sqlx::query!(
        "SELECT id FROM system_tables WHERE name = $1 AND is_active = true",
        table_data.name
    )
    .fetch_optional(pool.get_ref())
    .await {
        Ok(Some(_)) => {
            return HttpResponse::BadRequest().json("Table with this name already exists");
        },
        Ok(None) => {},
        Err(e) => {
            log::error!("Failed to check existing table: {}", e);
            return HttpResponse::InternalServerError().json("Error checking table existence");
        }
    };

    let mut transaction = match pool.begin().await {
        Ok(transaction) => transaction,
        Err(e) => {
            log::error!("Failed to start transaction: {}", e);
            return HttpResponse::InternalServerError().json("Error starting transaction");
        }
    };

    // Создаем основную таблицу
    let table_record = match sqlx::query!(
        "INSERT INTO system_tables (name, display_name, table_type, show_fact_table, fact_table_hint, instruction, is_active) 
         VALUES ($1, $2, $3, $4, $5, $6, true) 
         RETURNING id",
        table_data.name,
        table_data.display_name,
        table_data.table_type,
        table_data.show_fact_table.unwrap_or(false),
        table_data.fact_table_hint,
        table_data.instruction
    )
    .fetch_one(&mut *transaction)
    .await {
        Ok(record) => record,
        Err(e) => {
            log::error!("Failed to create system table: {}", e);
            return HttpResponse::InternalServerError().json("Error creating system table");
        }
    };

    // Создаем поля таблицы на основе типа
    let fields = get_default_fields_for_type(&table_data.table_type);
    
    for (index, field) in fields.iter().enumerate() {
        match sqlx::query!(
            "INSERT INTO table_fields (table_id, field_name, field_type, display_order, is_visible) 
             VALUES ($1, $2, $3, $4, true)",
            table_record.id,
            field.name,
            field.field_type,
            index as i32
        )
        .execute(&mut *transaction)
        .await {
            Ok(_) => {},
            Err(e) => {
                log::error!("Failed to create table field: {}", e);
                let _ = transaction.rollback().await;
                return HttpResponse::InternalServerError().json("Error creating table fields");
            }
        }
    }

    if let Err(e) = transaction.commit().await {
        log::error!("Failed to commit transaction: {}", e);
        return HttpResponse::InternalServerError().json("Error committing transaction");
    }

    HttpResponse::Ok().json(serde_json::json!({
        "id": table_record.id,
        "message": "Системная таблица успешно создана"
    }))
}

/// Обновление системной таблицы
pub async fn update_system_table(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    table_data: web::Json<UpdateSystemTableRequest>,
) -> impl Responder {
    let table_id = path.into_inner();
    
    // Проверяем существование таблицы
    let existing_table = match sqlx::query!(
        "SELECT id FROM system_tables WHERE id = $1 AND is_active = true",
        table_id
    )
    .fetch_optional(pool.get_ref())
    .await {
        Ok(Some(_)) => {},
        Ok(None) => {
            return HttpResponse::NotFound().json("System table not found");
        },
        Err(e) => {
            log::error!("Failed to check table existence: {}", e);
            return HttpResponse::InternalServerError().json("Error checking table existence");
        }
    };

    match sqlx::query!(
        "UPDATE system_tables 
         SET display_name = $1, table_type = $2, show_fact_table = $3, fact_table_hint = $4, instruction = $5
         WHERE id = $6",
        table_data.display_name,
        table_data.table_type,
        table_data.show_fact_table.unwrap_or(false),
        table_data.fact_table_hint,
        table_data.instruction,
        table_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() == 0 {
                return HttpResponse::NotFound().json("Системная таблица не найдена");
            }
        },
        Err(e) => {
            log::error!("Failed to update system table: {}", e);
            return HttpResponse::InternalServerError().json("Error updating system table");
        }
    }

    HttpResponse::Ok().json("Системная таблица успешно обновлена")
}

/// Удаление системной таблицы (мягкое удаление)
pub async fn delete_system_table(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
) -> impl Responder {
    let table_id = path.into_inner();
    
    match sqlx::query!(
        "UPDATE system_tables SET is_active = false WHERE id = $1",
        table_id
    )
    .execute(pool.get_ref())
    .await {
        Ok(result) => {
            if result.rows_affected() > 0 {
                HttpResponse::Ok().json("Системная таблица успешно удалена")
            } else {
                HttpResponse::NotFound().json("Системная таблица не найдена")
            }
        },
        Err(e) => {
            log::error!("Failed to delete system table: {}", e);
            HttpResponse::InternalServerError().json("Error deleting system table")
        }
    }
}

/// Вспомогательная функция для получения полей по умолчанию для типа таблицы
fn get_default_fields_for_type(table_type: &str) -> Vec<DefaultField> {
    match table_type {
        "cars" => vec![
            DefaultField {
                name: "car_number".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "car_brand".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "organization".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "unload_place".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "valid_until".to_string(),
                field_type: "date".to_string(),
            },
            DefaultField {
                name: "time_range".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "status".to_string(),
                field_type: "text".to_string(),
            },
        ],
        "people" => vec![
            DefaultField {
                name: "organization".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "last_name".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "first_name".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "middle_name".to_string(),
                field_type: "text".to_string(),
            },
            DefaultField {
                name: "valid_until".to_string(),
                field_type: "date".to_string(),
            },
            DefaultField {
                name: "pass_time".to_string(),
                field_type: "text".to_string(),
            },
        ],
        _ => vec![],
    }
}

/// Структура для полей по умолчанию
#[derive(Debug)]
struct DefaultField {
    name: String,
    field_type: String,
}