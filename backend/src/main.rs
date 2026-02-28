#![allow(warnings)]

mod models;
mod routes;
mod handlers;
mod auth;
mod database;

use actix_web::{App, HttpServer, middleware::Logger, web, HttpResponse};
use actix_files as fs; // Добавить этот импорт
use actix_cors::Cors;
use std::sync::Arc;
use dashmap::DashMap;
use database::get_pool;
use std::time::{SystemTime, UNIX_EPOCH};
use actix_web::dev::{Service, ServiceRequest, ServiceResponse, Transform};
use std::future::{ready, Ready};
use std::task::{Context, Poll};
use std::pin::Pin;
use futures::future::LocalBoxFuture;

#[derive(Clone)]
struct RateLimiter {
    requests: Arc<DashMap<String, Vec<u64>>>,
    max_requests: usize,
    window_secs: u64,
}

impl RateLimiter {
    fn new(max_requests: usize, window_secs: u64) -> Self {
        Self {
            requests: Arc::new(DashMap::new()),
            max_requests,
            window_secs,
        }
    }

    fn is_allowed(&self, key: String) -> bool {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();

        let mut entry = self.requests.entry(key).or_insert_with(Vec::new);
        let timestamps = &mut *entry;

        timestamps.retain(|&ts| (now as i64 - ts as i64) < self.window_secs as i64);

        if timestamps.len() < self.max_requests {
            timestamps.push(now);
            true
        } else {
            false
        }
    }
}

// Создаем свою middleware структуру
pub struct RateLimitMiddleware;

impl<S, B> Transform<S, ServiceRequest> for RateLimitMiddleware
where
    S: Service<ServiceRequest, Response = ServiceResponse<B>, Error = actix_web::Error> + 'static,
    S::Future: 'static,
    B: 'static,
{
    type Response = ServiceResponse<B>;
    type Error = actix_web::Error;
    type Transform = RateLimitMiddlewareService<S>;
    type InitError = ();
    type Future = Ready<Result<Self::Transform, Self::InitError>>;

    fn new_transform(&self, service: S) -> Self::Future {
        ready(Ok(RateLimitMiddlewareService { service }))
    }
}

pub struct RateLimitMiddlewareService<S> {
    service: S,
}

impl<S, B> Service<ServiceRequest> for RateLimitMiddlewareService<S>
where
    S: Service<ServiceRequest, Response = ServiceResponse<B>, Error = actix_web::Error> + 'static,
    S::Future: 'static,
    B: 'static,
{
    type Response = ServiceResponse<B>;
    type Error = actix_web::Error;
    type Future = LocalBoxFuture<'static, Result<Self::Response, Self::Error>>;

    fn poll_ready(&self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.service.poll_ready(cx)
    }

    fn call(&self, req: ServiceRequest) -> Self::Future {
        // Извлекаем RateLimiter из app_data
        let limiter_data = req.app_data::<web::Data<RateLimiter>>();
        
        // Проверяем rate limit только для определенных путей
        let path = req.path();
        
        let check_rate_limit = path == "/login" || path.starts_with("/api/");
        
        if check_rate_limit {
            if let Some(limiter) = limiter_data {
                let client_ip = req.connection_info().realip_remote_addr().unwrap_or("unknown").to_string();
                
                let key = if let Some(auth) = req.headers().get("Authorization") {
                    if let Ok(auth_str) = auth.to_str() {
                        if let Some(token) = auth_str.strip_prefix("Bearer ") {
                            format!("user:{}", &token[..std::cmp::min(20, token.len())])
                        } else {
                            client_ip
                        }
                    } else {
                        client_ip
                    }
                } else {
                    client_ip
                };

                if !limiter.is_allowed(key) {
                    return Box::pin(async move {
                        Err(actix_web::error::ErrorTooManyRequests(
                            "Вы отправляете слишком много запросов. Подождите 60 секунд."
                        ))
                    });
                }
            }
        }

        let fut = self.service.call(req);
        
        Box::pin(async move {
            fut.await
        })
    }
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    dotenv::dotenv().ok();
    
    let pool = get_pool().await;
    let rate_limiter = RateLimiter::new(10, 60);

    // Создаем директорию для загрузок, если её нет
    std::fs::create_dir_all("./uploads/unload_places").expect("Failed to create upload directory");

    println!("🚀 Server starting on http://127.0.0.1:8080");
    println!("📊 Rate limit: 10 requests/minute per IP/user");
    println!("📁 Upload directory: ./uploads/unload_places");

    HttpServer::new(move || {
        let limiter = rate_limiter.clone();
        let pool = pool.clone();

        App::new()
            .wrap(Logger::default())
            // CORS должен быть ПЕРЕД rate limit middleware
            .wrap(
                Cors::default()
                    .allow_any_origin()
                    .allow_any_method()
                    .allow_any_header()
                    .supports_credentials()
                    .max_age(3600)
            )
            .wrap(RateLimitMiddleware)
            // Добавляем обслуживание статических файлов из папки uploads
            .service(fs::Files::new("/uploads", "./uploads").show_files_listing())
            .app_data(web::Data::new(limiter.clone()))
            .app_data(web::Data::new(pool.clone()))
            .configure(routes::config)
    })
    .bind(("127.0.0.1", 8080))?
    .run()
    .await
}