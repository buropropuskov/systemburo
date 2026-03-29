use futures::future::LocalBoxFuture;
use std::future::ready;
use std::rc::Rc;
use actix_web::{
    dev::{Service, ServiceRequest, ServiceResponse, Transform},
    Error, HttpMessage, web,
};
use actix_web::web::Data;
use sqlx::PgPool;
use serde_json::Value;
use crate::auth::decode_token;
use crate::websocket::broadcast_new_log;

use std::time::Instant;
use std::task::Poll;
use std::task::Context;

pub struct RequestLogger;

impl<S, B> Transform<S, ServiceRequest> for RequestLogger
where
    S: Service<ServiceRequest, Response = ServiceResponse<B>, Error = Error> + 'static,
    S::Future: 'static,
    B: 'static,
{
    type Response = ServiceResponse<B>;
    type Error = Error;
    type Transform = RequestLoggerMiddleware<S>;
    type InitError = ();
    type Future = std::future::Ready<Result<Self::Transform, Self::InitError>>;

    fn new_transform(&self, service: S) -> Self::Future {
        ready(Ok(RequestLoggerMiddleware {
            service: Rc::new(service),
        }))
    }
}

pub struct RequestLoggerMiddleware<S> {
    service: Rc<S>,
}

impl<S, B> Service<ServiceRequest> for RequestLoggerMiddleware<S>
where
    S: Service<ServiceRequest, Response = ServiceResponse<B>, Error = Error> + 'static,
    S::Future: 'static,
    B: 'static,
{
    type Response = ServiceResponse<B>;
    type Error = Error;
    type Future = LocalBoxFuture<'static, Result<Self::Response, Self::Error>>;

    fn poll_ready(&self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.service.poll_ready(cx)
    }

    fn call(&self, req: ServiceRequest) -> Self::Future {
        let start = Instant::now();
        let method = req.method().to_string();
        let url = req.uri().to_string();

        let username = if let Some(auth_header) = req.headers().get("Authorization") {
            if let Ok(auth_str) = auth_header.to_str() {
                if let Some(token) = auth_str.strip_prefix("Bearer ") {
                    match decode_token(token) {
                        Ok(claims) => Some(claims.sub),
                        Err(_) => None,
                    }
                } else {
                    None
                }
            } else {
                None
            }
        } else {
            None
        };

        let service = self.service.clone();
        let pool = req.app_data::<Data<PgPool>>().cloned();

        Box::pin(async move {
            let fut = service.call(req);
            let res = fut.await?;

            let duration = start.elapsed().as_millis() as i32;
            let status = res.status().as_u16();

            if let Some(pool) = pool {
                let mut user_id: Option<i32> = None;
                if let Some(ref uname) = username {
                    let row = sqlx::query!("SELECT id FROM users WHERE username = $1", uname)
                        .fetch_optional(pool.get_ref())
                        .await;
                    if let Ok(Some(row)) = row {
                        user_id = Some(row.id);
                    }
                }

                let headers_map = res.headers();
                let headers_vec: Vec<(String, String)> = headers_map
                    .iter()
                    .map(|(k, v)| (k.as_str().to_string(), v.to_str().unwrap_or("").to_string()))
                    .collect();
                let headers_json = serde_json::to_value(headers_vec).unwrap_or(Value::Null);

                let _ = sqlx::query!(
                    r#"
                    INSERT INTO request_logs (user_id, username, method, url, headers, response_status, duration_ms)
                    VALUES ($1, $2, $3, $4, $5, $6, $7)
                    "#,
                    user_id,
                    username,
                    method,
                    url,
                    headers_json,
                    status as i32,
                    duration
                )
                .execute(pool.get_ref())
                .await;

                // После сохранения лога – рассылаем по WebSocket
                let _ = broadcast_new_log(user_id, username, method, url, status as i32, duration).await;
            }

            Ok(res)
        })
    }
}