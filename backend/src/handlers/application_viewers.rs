use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use sqlx::PgPool;
use log;

use crate::auth::decode_token;

#[derive(Debug, serde::Serialize)]
pub struct ViewerWithUser {
    pub id: i32,
    pub user_id: i32,
    pub username: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub created_at: Option<chrono::NaiveDateTime>,
}

/// Получение списка просматривающих для заявки
pub async fn get_application_viewers(
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

    let viewers = sqlx::query_as!(
        ViewerWithUser,
        r#"
        SELECT 
            av.id,
            av.user_id,
            u.username,
            u.last_name,
            u.first_name,
            u.middle_name,
            u.position,
            av.created_at
        FROM application_viewers av
        JOIN users u ON av.user_id = u.id
        WHERE av.application_id = $1
        ORDER BY u.last_name, u.first_name
        "#,
        application_id
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch viewers: {}", e);
        error::ErrorInternalServerError("Error fetching viewers")
    })?;

    Ok(HttpResponse::Ok().json(viewers))
}