use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::{PgPool, Row};
use serde_json::json;
use log;
use serde::{Deserialize, Serialize};

use crate::auth::decode_token;

#[derive(Debug, Deserialize)]
pub struct ItemData {
    pub name: String,
    pub count: i32,
    pub order_index: i32,
}

/// Создание ТМЦ
pub async fn create_item(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    item_data: web::Json<ItemData>,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let _claims = decode_token(token)
        .map_err(|_| error::ErrorUnauthorized("Invalid token"))?;

    // Создаем ТМЦ (attachment_id будет установлен позже при создании вложения)
    let item_result = sqlx::query!(
        r#"
        INSERT INTO items (
            name,
            count,
            date_created
        )
        VALUES ($1, $2, CURRENT_DATE)
        RETURNING id
        "#,
        item_data.name,
        item_data.count
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to create item: {}", e);
        error::ErrorInternalServerError("Error creating item")
    })?;

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Item created successfully",
        "item_id": item_result.id
    })))
}