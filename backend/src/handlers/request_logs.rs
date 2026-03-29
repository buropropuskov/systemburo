// src/handlers/request_logs.rs

use actix_web::{web, HttpResponse, HttpRequest, Error, error};
use serde::{Deserialize, Serialize};
use sqlx::PgPool;
use chrono::{DateTime, Utc, Local, Duration};
use crate::auth::decode_token;
use serde_json::Value;

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct RequestLog {
    pub id: i64,
    pub user_id: Option<i32>,
    pub username: Option<String>,
    pub method: String,
    pub url: String,
    pub response_status: Option<i32>,
    pub duration_ms: Option<i32>,
    pub created_at: DateTime<Utc>,
}

#[derive(Deserialize)]
pub struct LogsQuery {
    pub user_id: Option<i32>,
    pub method: Option<String>,
    pub status: Option<i32>,
    pub from_date: Option<String>,
    pub to_date: Option<String>,
    pub search: Option<String>,
    pub page: Option<u32>,
    pub per_page: Option<u32>,
}

#[derive(Debug, Serialize)]
pub struct LogsUser {
    pub id: i32,
    pub username: String,
}

#[derive(Debug, Serialize)]
pub struct LogsStats {
    pub total: i64,
    pub today: i64,
    pub avg_duration: f64,
    pub error_rate: f64,
    pub requests_per_second: f64,
    pub requests_per_minute: f64,
}

#[derive(Debug, Serialize)]
pub struct RealtimeStats {
    pub last_second_count: i64,
    pub last_minute_count: i64,
}

#[derive(Debug, Serialize)]
pub struct TimelinePoint {
    pub timestamp: String,
    pub count: i64,
    pub avg_duration: f64,
}

#[derive(Deserialize)]
pub struct TimelineQuery {
    pub interval: Option<i64>,
    pub limit: Option<usize>,
    pub from_date: Option<String>,
    pub to_date: Option<String>,
}

// -----------------------------------------------------------------------------
// Получение списка логов с пагинацией и фильтрацией
// -----------------------------------------------------------------------------
pub async fn get_request_logs(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<LogsQuery>,
) -> Result<HttpResponse, Error> {
    // Проверка прав доступа: только manager или buropropuskov
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(token) {
                    Ok(claims) => {
                        let user_type = sqlx::query!(
                            "SELECT code FROM user_types WHERE id = $1",
                            claims.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?
                        .code;

                        if user_type != "manager" && user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                    }
                    Err(_) => return Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid auth header"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid auth header"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing Authorization header"));
    }

    let page = query.page.unwrap_or(1).max(1);
    let per_page = query.per_page.unwrap_or(20).min(100);
    let offset = (page - 1) * per_page;

    let mut sql = String::from(
        "SELECT id, user_id, username, method, url, response_status, duration_ms, created_at
         FROM request_logs WHERE 1=1"
    );
    let mut params: Vec<sqlx::types::JsonValue> = Vec::new();

    // Фильтры
    if let Some(user_id) = query.user_id {
        sql.push_str(" AND user_id = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(user_id)));
    }
    if let Some(method) = &query.method {
        sql.push_str(" AND method = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::String(method.clone()));
    }
    if let Some(status) = query.status {
        sql.push_str(" AND response_status = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(status)));
    }
    if let Some(from_date) = &query.from_date {
        sql.push_str(" AND created_at >= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = from_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid from_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(to_date) = &query.to_date {
        sql.push_str(" AND created_at <= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = to_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid to_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(search) = &query.search {
        sql.push_str(" AND (url ILIKE $");
        sql.push_str(&(params.len() + 1).to_string());
        sql.push_str(" OR username ILIKE $");
        sql.push_str(&(params.len() + 2).to_string());
        sql.push_str(")");
        let pattern = format!("%{}%", search);
        params.push(sqlx::types::JsonValue::String(pattern.clone()));
        params.push(sqlx::types::JsonValue::String(pattern));
    }

    sql.push_str(" ORDER BY created_at DESC LIMIT $");
    sql.push_str(&(params.len() + 1).to_string());
    params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(per_page as i64)));
    sql.push_str(" OFFSET $");
    sql.push_str(&(params.len() + 1).to_string());
    params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(offset as i64)));

    let mut query_builder = sqlx::query_as::<_, RequestLog>(&sql);
    for p in params {
        match p {
            sqlx::types::JsonValue::Number(n) => {
                if let Some(i) = n.as_i64() {
                    query_builder = query_builder.bind(i);
                } else if let Some(u) = n.as_u64() {
                    query_builder = query_builder.bind(u as i64);
                }
            }
            sqlx::types::JsonValue::String(s) => {
                query_builder = query_builder.bind(s);
            }
            _ => {}
        }
    }
    let logs = query_builder.fetch_all(pool.get_ref()).await.map_err(|e| {
        log::error!("Failed to fetch request logs: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    // Подсчёт общего количества для пагинации
    let mut count_sql = String::from("SELECT COUNT(*) FROM request_logs WHERE 1=1");
    let mut count_params: Vec<sqlx::types::JsonValue> = Vec::new();
    if let Some(user_id) = query.user_id {
        count_sql.push_str(" AND user_id = $");
        count_sql.push_str(&(count_params.len() + 1).to_string());
        count_params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(user_id)));
    }
    if let Some(method) = &query.method {
        count_sql.push_str(" AND method = $");
        count_sql.push_str(&(count_params.len() + 1).to_string());
        count_params.push(sqlx::types::JsonValue::String(method.clone()));
    }
    if let Some(status) = query.status {
        count_sql.push_str(" AND response_status = $");
        count_sql.push_str(&(count_params.len() + 1).to_string());
        count_params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(status)));
    }
    if let Some(from_date) = &query.from_date {
        count_sql.push_str(" AND created_at >= $");
        count_sql.push_str(&(count_params.len() + 1).to_string());
        let dt = from_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid from_date"))?;
        count_params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(to_date) = &query.to_date {
        count_sql.push_str(" AND created_at <= $");
        count_sql.push_str(&(count_params.len() + 1).to_string());
        let dt = to_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid to_date"))?;
        count_params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(search) = &query.search {
        count_sql.push_str(" AND (url ILIKE $");
        count_sql.push_str(&(count_params.len() + 1).to_string());
        count_sql.push_str(" OR username ILIKE $");
        count_sql.push_str(&(count_params.len() + 2).to_string());
        count_sql.push_str(")");
        let pattern = format!("%{}%", search);
        count_params.push(sqlx::types::JsonValue::String(pattern.clone()));
        count_params.push(sqlx::types::JsonValue::String(pattern));
    }

    let mut count_query = sqlx::query_scalar::<_, i64>(&count_sql);
    for p in count_params {
        match p {
            sqlx::types::JsonValue::Number(n) => {
                if let Some(i) = n.as_i64() {
                    count_query = count_query.bind(i);
                } else if let Some(u) = n.as_u64() {
                    count_query = count_query.bind(u as i64);
                }
            }
            sqlx::types::JsonValue::String(s) => {
                count_query = count_query.bind(s);
            }
            _ => {}
        }
    }
    let total = count_query.fetch_one(pool.get_ref()).await.unwrap_or(0);

    Ok(HttpResponse::Ok().json(serde_json::json!({
        "logs": logs,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": (total as f64 / per_page as f64).ceil() as u32
    })))
}

// -----------------------------------------------------------------------------
// Получение списка пользователей для фильтра
// -----------------------------------------------------------------------------
pub async fn get_logs_users(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    // Проверка прав доступа (аналогично)
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(token) {
                    Ok(claims) => {
                        let user_type = sqlx::query!(
                            "SELECT code FROM user_types WHERE id = $1",
                            claims.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?
                        .code;

                        if user_type != "manager" && user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                    }
                    Err(_) => return Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid auth header"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid auth header"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing Authorization header"));
    }

    let users = sqlx::query_as!(
        LogsUser,
        "SELECT DISTINCT u.id, u.username FROM users u 
         JOIN request_logs rl ON u.id = rl.user_id
         ORDER BY u.username"
    )
    .fetch_all(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch users for logs filter: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    Ok(HttpResponse::Ok().json(users))
}

// -----------------------------------------------------------------------------
// Получение статистики по запросам
// -----------------------------------------------------------------------------
pub async fn get_logs_stats(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<LogsQuery>,
) -> Result<HttpResponse, Error> {
    // Проверка прав доступа (аналогично)
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(token) {
                    Ok(claims) => {
                        let user_type = sqlx::query!(
                            "SELECT code FROM user_types WHERE id = $1",
                            claims.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?
                        .code;

                        if user_type != "manager" && user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                    }
                    Err(_) => return Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid auth header"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid auth header"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing Authorization header"));
    }

    // Строим базовый SQL с фильтрами
    let mut sql = String::from(
        "SELECT 
            COUNT(*) as total,
            COALESCE(AVG(duration_ms)::FLOAT8, 0) as avg_duration,
            COUNT(CASE WHEN response_status >= 400 THEN 1 END) as errors
         FROM request_logs WHERE 1=1"
    );
    let mut params: Vec<sqlx::types::JsonValue> = Vec::new();

    // Добавляем фильтры
    if let Some(user_id) = query.user_id {
        sql.push_str(" AND user_id = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(user_id)));
    }
    if let Some(method) = &query.method {
        sql.push_str(" AND method = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::String(method.clone()));
    }
    if let Some(status) = query.status {
        sql.push_str(" AND response_status = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(status)));
    }
    if let Some(from_date) = &query.from_date {
        sql.push_str(" AND created_at >= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = from_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid from_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(to_date) = &query.to_date {
        sql.push_str(" AND created_at <= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = to_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid to_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(search) = &query.search {
        sql.push_str(" AND (url ILIKE $");
        sql.push_str(&(params.len() + 1).to_string());
        sql.push_str(" OR username ILIKE $");
        sql.push_str(&(params.len() + 2).to_string());
        sql.push_str(")");
        let pattern = format!("%{}%", search);
        params.push(sqlx::types::JsonValue::String(pattern.clone()));
        params.push(sqlx::types::JsonValue::String(pattern));
    }

    let mut query_builder = sqlx::query_as::<_, (i64, f64, i64)>(&sql);
    for p in params {
        match p {
            sqlx::types::JsonValue::Number(n) => {
                if let Some(i) = n.as_i64() {
                    query_builder = query_builder.bind(i);
                } else if let Some(u) = n.as_u64() {
                    query_builder = query_builder.bind(u as i64);
                }
            }
            sqlx::types::JsonValue::String(s) => {
                query_builder = query_builder.bind(s);
            }
            _ => {}
        }
    }
    let (total, avg_duration, errors) = query_builder.fetch_one(pool.get_ref()).await
        .map_err(|e| {
            log::error!("Failed to fetch stats: {}", e);
            error::ErrorInternalServerError("Database error")
        })?;

    // Подсчёт запросов за сегодня
    let today_start = Local::now().date_naive().and_hms_opt(0, 0, 0).unwrap();
    let today_start_utc = today_start.and_local_timezone(Utc).unwrap();
    let today_count = sqlx::query_scalar!(
        "SELECT COUNT(*) FROM request_logs WHERE created_at >= $1",
        today_start_utc
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch today count: {}", e);
        error::ErrorInternalServerError("Database error")
    })?
    .unwrap_or(0);

    // Вычисляем запросы в секунду и минуту на основе временного диапазона
    let mut seconds_span = 1.0;
    let min_max_result = sqlx::query!(
        "SELECT MIN(created_at) as min_ts, MAX(created_at) as max_ts FROM request_logs"
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch min/max timestamps: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    if let (Some(min_ts), Some(max_ts)) = (min_max_result.min_ts, min_max_result.max_ts) {
        let diff = max_ts - min_ts;
        let total_secs = diff.num_seconds() as f64;
        if total_secs > 0.0 {
            seconds_span = total_secs;
        }
    }

    let requests_per_second = total as f64 / seconds_span;
    let requests_per_minute = requests_per_second * 60.0;

    let error_rate = if total > 0 {
        (errors as f64 / total as f64) * 100.0
    } else {
        0.0
    };

    Ok(HttpResponse::Ok().json(LogsStats {
        total,
        today: today_count,
        avg_duration,
        error_rate: (error_rate * 100.0).round() / 100.0,
        requests_per_second,
        requests_per_minute,
    }))
}

// -----------------------------------------------------------------------------
// Получение реальной статистики (последние 1 сек и 1 мин)
// -----------------------------------------------------------------------------
pub async fn get_realtime_stats(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    // Проверка прав доступа (аналогично)
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(token) {
                    Ok(claims) => {
                        let user_type = sqlx::query!(
                            "SELECT code FROM user_types WHERE id = $1",
                            claims.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?
                        .code;

                        if user_type != "manager" && user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                    }
                    Err(_) => return Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid auth header"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid auth header"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing Authorization header"));
    }

    let now = Utc::now();
    let one_sec_ago = now - Duration::seconds(1);
    let one_min_ago = now - Duration::minutes(1);

    let last_second = sqlx::query_scalar!(
        "SELECT COUNT(*) FROM request_logs WHERE created_at >= $1",
        one_sec_ago
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch last second count: {}", e);
        error::ErrorInternalServerError("Database error")
    })?
    .unwrap_or(0);

    let last_minute = sqlx::query_scalar!(
        "SELECT COUNT(*) FROM request_logs WHERE created_at >= $1",
        one_min_ago
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch last minute count: {}", e);
        error::ErrorInternalServerError("Database error")
    })?
    .unwrap_or(0);

    Ok(HttpResponse::Ok().json(RealtimeStats {
        last_second_count: last_second,
        last_minute_count: last_minute,
    }))
}

// -----------------------------------------------------------------------------
// Получение временной шкалы (исправленная)
// -----------------------------------------------------------------------------
pub async fn get_request_timeline(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<TimelineQuery>,
) -> Result<HttpResponse, Error> {
    // Проверка прав доступа (аналогично)
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(token) {
                    Ok(claims) => {
                        let user_type = sqlx::query!(
                            "SELECT code FROM user_types WHERE id = $1",
                            claims.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?
                        .code;

                        if user_type != "manager" && user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                    }
                    Err(_) => return Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid auth header"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid auth header"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing Authorization header"));
    }

    let interval = query.interval.unwrap_or(60);
    let limit = query.limit.unwrap_or(24);

    let mut sql = String::from(
        r#"
        SELECT
            to_timestamp(floor(extract(epoch from created_at) / $1) * $1) as bucket,
            COUNT(*) as count,
            COALESCE(AVG(duration_ms)::FLOAT8, 0) as avg_duration
        FROM request_logs
        WHERE 1=1
        "#
    );
    let mut params: Vec<sqlx::types::JsonValue> = Vec::new();
    params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(interval as i64)));

    if let Some(from_date) = &query.from_date {
        sql.push_str(" AND created_at >= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = from_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid from_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(to_date) = &query.to_date {
        sql.push_str(" AND created_at <= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = to_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid to_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }

    sql.push_str(" GROUP BY bucket ORDER BY bucket DESC LIMIT $");
    sql.push_str(&(params.len() + 1).to_string());
    params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(limit as i64)));

    let mut query_builder = sqlx::query_as::<_, (DateTime<Utc>, i64, f64)>(&sql);
    for p in params {
        match p {
            sqlx::types::JsonValue::Number(n) => {
                if let Some(i) = n.as_i64() {
                    query_builder = query_builder.bind(i);
                } else if let Some(u) = n.as_u64() {
                    query_builder = query_builder.bind(u as i64);
                }
            }
            sqlx::types::JsonValue::String(s) => {
                query_builder = query_builder.bind(s);
            }
            _ => {}
        }
    }
    let rows = query_builder.fetch_all(pool.get_ref()).await.map_err(|e| {
        log::error!("Failed to fetch timeline: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let timeline: Vec<TimelinePoint> = rows
        .into_iter()
        .map(|(ts, cnt, avg)| TimelinePoint {
            timestamp: ts.to_rfc3339(),
            count: cnt,
            avg_duration: avg,
        })
        .collect();

    Ok(HttpResponse::Ok().json(timeline))
}

// -----------------------------------------------------------------------------
// Экспорт логов в текстовый файл (TSV)
// -----------------------------------------------------------------------------
pub async fn export_logs_txt(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    query: web::Query<LogsQuery>,
) -> Result<HttpResponse, Error> {
    // Проверка прав доступа (аналогично)
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(token) {
                    Ok(claims) => {
                        let user_type = sqlx::query!(
                            "SELECT code FROM user_types WHERE id = $1",
                            claims.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?
                        .code;

                        if user_type != "manager" && user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                    }
                    Err(_) => return Err(error::ErrorUnauthorized("Invalid token")),
                }
            } else {
                return Err(error::ErrorUnauthorized("Invalid auth header"));
            }
        } else {
            return Err(error::ErrorUnauthorized("Invalid auth header"));
        }
    } else {
        return Err(error::ErrorUnauthorized("Missing Authorization header"));
    }

    let mut sql = String::from(
        "SELECT id, user_id, username, method, url, response_status, duration_ms, created_at
         FROM request_logs WHERE 1=1"
    );
    let mut params: Vec<sqlx::types::JsonValue> = Vec::new();

    if let Some(user_id) = query.user_id {
        sql.push_str(" AND user_id = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(user_id)));
    }
    if let Some(method) = &query.method {
        sql.push_str(" AND method = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::String(method.clone()));
    }
    if let Some(status) = query.status {
        sql.push_str(" AND response_status = $");
        sql.push_str(&(params.len() + 1).to_string());
        params.push(sqlx::types::JsonValue::Number(serde_json::Number::from(status)));
    }
    if let Some(from_date) = &query.from_date {
        sql.push_str(" AND created_at >= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = from_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid from_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(to_date) = &query.to_date {
        sql.push_str(" AND created_at <= $");
        sql.push_str(&(params.len() + 1).to_string());
        let dt = to_date.parse::<DateTime<Utc>>().map_err(|_| error::ErrorBadRequest("Invalid to_date"))?;
        params.push(sqlx::types::JsonValue::String(dt.to_rfc3339()));
    }
    if let Some(search) = &query.search {
        sql.push_str(" AND (url ILIKE $");
        sql.push_str(&(params.len() + 1).to_string());
        sql.push_str(" OR username ILIKE $");
        sql.push_str(&(params.len() + 2).to_string());
        sql.push_str(")");
        let pattern = format!("%{}%", search);
        params.push(sqlx::types::JsonValue::String(pattern.clone()));
        params.push(sqlx::types::JsonValue::String(pattern));
    }

    sql.push_str(" ORDER BY created_at DESC");

    let mut query_builder = sqlx::query_as::<_, RequestLog>(&sql);
    for p in params {
        match p {
            sqlx::types::JsonValue::Number(n) => {
                if let Some(i) = n.as_i64() {
                    query_builder = query_builder.bind(i);
                } else if let Some(u) = n.as_u64() {
                    query_builder = query_builder.bind(u as i64);
                }
            }
            sqlx::types::JsonValue::String(s) => {
                query_builder = query_builder.bind(s);
            }
            _ => {}
        }
    }
    let logs = query_builder.fetch_all(pool.get_ref()).await.map_err(|e| {
        log::error!("Failed to fetch logs for export: {}", e);
        error::ErrorInternalServerError("Database error")
    })?;

    let mut content = String::new();
    content.push_str("ID\tПользователь\tДата и время (МСК)\tМетод\tURL\tСтатус\tДлительность (мс)\n");
    for log in logs {
        let created = log.created_at.with_timezone(&chrono::Local).format("%Y-%m-%d %H:%M:%S");
        content.push_str(&format!(
            "{}\t{}\t{}\t{}\t{}\t{}\t{}\n",
            log.id,
            log.username.unwrap_or_else(|| "Неавторизован".to_string()),
            created,
            log.method,
            log.url,
            log.response_status.unwrap_or(0),
            log.duration_ms.unwrap_or(0)
        ));
    }

    Ok(HttpResponse::Ok()
        .content_type("text/plain; charset=utf-8")
        .body(content))
}

pub async fn get_stats_for_broadcast(
    pool: &PgPool,
) -> Result<(LogsStats, RealtimeStats), sqlx::Error> {
    let row = sqlx::query!(
        r#"
        SELECT 
            COUNT(*) as total,
            COALESCE(AVG(duration_ms)::FLOAT8, 0) as avg_duration,
            COUNT(CASE WHEN response_status >= 400 THEN 1 END) as errors
        FROM request_logs
        "#
    )
    .fetch_one(pool)
    .await?;

    let total = row.total.unwrap_or(0);
    let avg_duration = row.avg_duration.unwrap_or(0.0);
    let errors = row.errors.unwrap_or(0);
    let error_rate = if total > 0 { (errors as f64 / total as f64) * 100.0 } else { 0.0 };

    let today_start = chrono::Local::now().date_naive().and_hms_opt(0, 0, 0).unwrap();
    let today_start_utc = today_start.and_local_timezone(chrono::Utc).unwrap();
    let today = sqlx::query_scalar!(
        "SELECT COUNT(*) FROM request_logs WHERE created_at >= $1",
        today_start_utc
    )
    .fetch_one(pool)
    .await?
    .unwrap_or(0);

    let now = chrono::Utc::now();
    let one_sec_ago = now - chrono::Duration::seconds(1);
    let one_min_ago = now - chrono::Duration::minutes(1);

    let last_second = sqlx::query_scalar!(
        "SELECT COUNT(*) FROM request_logs WHERE created_at >= $1",
        one_sec_ago
    )
    .fetch_one(pool)
    .await?
    .unwrap_or(0);

    let last_minute = sqlx::query_scalar!(
        "SELECT COUNT(*) FROM request_logs WHERE created_at >= $1",
        one_min_ago
    )
    .fetch_one(pool)
    .await?
    .unwrap_or(0);

    Ok((
        LogsStats {
            total,
            today,
            avg_duration,
            error_rate,
            requests_per_second: 0.0,
            requests_per_minute: 0.0,
        },
        RealtimeStats {
            last_second_count: last_second,
            last_minute_count: last_minute,
        }
    ))
}

pub async fn get_timeline_for_broadcast(
    pool: &PgPool,
) -> Result<Vec<TimelinePoint>, sqlx::Error> {
    let interval = 60;
    let limit = 24;

    let rows = sqlx::query!(
        r#"
        SELECT
            to_timestamp(floor(extract(epoch from created_at) / $1) * $1) as bucket,
            COUNT(*) as count,
            COALESCE(AVG(duration_ms)::FLOAT8, 0) as avg_duration
        FROM request_logs
        GROUP BY bucket
        ORDER BY bucket DESC
        LIMIT $2
        "#,
        interval as i64,
        limit as i64,
    )
    .fetch_all(pool)
    .await?;

    let mut timeline = Vec::new();
    for row in rows {
        if let Some(ts) = row.bucket {
            timeline.push(TimelinePoint {
                timestamp: ts.to_rfc3339(),
                count: row.count.unwrap_or(0),
                avg_duration: row.avg_duration.unwrap_or(0.0),
            });
        }
    }
    timeline.reverse();
    Ok(timeline)
}