// src/websocket.rs – полностью переработанная версия с периодической рассылкой статистики и таймлайна

use actix::{Actor, StreamHandler, AsyncContext, Addr, ActorContext, Running, Message, Handler};
use actix_web::{web, HttpRequest, HttpResponse, Error};
use actix_web_actors::ws;
use dashmap::DashMap;
use once_cell::sync::Lazy;
use serde_json::json;
use std::time::{Instant, Duration};
use sqlx::PgPool;
use crate::auth::decode_token;
use crate::handlers::request_logs;

#[derive(Clone, Message)]
#[rtype(result = "()")]
struct BroadcastMessage {
    payload: String,
}

pub static WS_CLIENTS: Lazy<DashMap<usize, Addr<LogsWebSocket>>> = Lazy::new(DashMap::new);

pub struct LogsWebSocket {
    last_ping: Instant,
    user_id: i32,
}

impl Actor for LogsWebSocket {
    type Context = ws::WebsocketContext<Self>;

    fn started(&mut self, ctx: &mut Self::Context) {
        self.last_ping = Instant::now();
        WS_CLIENTS.insert(self.user_id as usize, ctx.address());
        ctx.run_interval(Duration::from_secs(10), |act, ctx| {
            if Instant::now().duration_since(act.last_ping) > Duration::from_secs(30) {
                ctx.stop();
                return;
            }
            ctx.ping(b"");
        });
    }

    fn stopping(&mut self, _ctx: &mut Self::Context) -> Running {
        WS_CLIENTS.remove(&(self.user_id as usize));
        Running::Stop
    }
}

impl Handler<BroadcastMessage> for LogsWebSocket {
    type Result = ();

    fn handle(&mut self, msg: BroadcastMessage, ctx: &mut Self::Context) {
        ctx.text(msg.payload);
    }
}

impl StreamHandler<Result<ws::Message, ws::ProtocolError>> for LogsWebSocket {
    fn handle(&mut self, msg: Result<ws::Message, ws::ProtocolError>, ctx: &mut Self::Context) {
        match msg {
            Ok(ws::Message::Ping(msg)) => {
                self.last_ping = Instant::now();
                ctx.pong(&msg);
            }
            Ok(ws::Message::Pong(_)) => {
                self.last_ping = Instant::now();
            }
            _ => {}
        }
    }
}

pub async fn ws_logs(req: HttpRequest, stream: web::Payload) -> Result<HttpResponse, Error> {
    let token = req.query_string()
        .split('&')
        .find(|p| p.starts_with("token="))
        .and_then(|p| p.split('=').nth(1))
        .unwrap_or("");

    if token.is_empty() {
        return Ok(HttpResponse::Unauthorized().finish());
    }

    let pool = req.app_data::<web::Data<PgPool>>().unwrap();

    match decode_token(token) {
        Ok(claims) => {
            let user_type = sqlx::query!(
                "SELECT code FROM user_types WHERE id = $1",
                claims.type_id
            )
            .fetch_one(pool.get_ref())
            .await
            .map_err(|e| {
                log::error!("Failed to fetch user type: {}", e);
                actix_web::error::ErrorUnauthorized("Unauthorized")
            })?;

            if user_type.code != "manager" && user_type.code != "buropropuskov" {
                return Ok(HttpResponse::Forbidden().finish());
            }

            let user_row = sqlx::query!(
                "SELECT id FROM users WHERE username = $1",
                claims.sub
            )
            .fetch_one(pool.get_ref())
            .await
            .map_err(|e| {
                log::error!("Failed to fetch user id: {}", e);
                actix_web::error::ErrorUnauthorized("Unauthorized")
            })?;

            let response = ws::start(
                LogsWebSocket {
                    last_ping: Instant::now(),
                    user_id: user_row.id,
                },
                &req,
                stream,
            )?;
            Ok(response)
        }
        Err(_) => Ok(HttpResponse::Unauthorized().finish()),
    }
}

// Функция для рассылки нового лога (вызывается из middleware)
pub async fn broadcast_new_log(
    user_id: Option<i32>,
    username: Option<String>,
    method: String,
    url: String,
    status: i32,
    duration: i32,
) {
    let message = json!({
        "type": "new_log",
        "log": {
            "user_id": user_id,
            "username": username,
            "method": method,
            "url": url,
            "response_status": status,
            "duration_ms": duration,
            "created_at": chrono::Utc::now().to_rfc3339()
        }
    }).to_string();

    for entry in WS_CLIENTS.iter() {
        entry.value().do_send(BroadcastMessage { payload: message.clone() });
    }
}

// Запуск периодической рассылки статистики и таймлайна
pub fn start_broadcast_tasks(pool: web::Data<PgPool>) {
    let pool_clone = pool.clone();
    actix_rt::spawn(async move {
        let mut interval_stats = tokio::time::interval(Duration::from_secs(1));
        let mut interval_timeline = tokio::time::interval(Duration::from_secs(30));

        loop {
            tokio::select! {
                _ = interval_stats.tick() => {
                    let pool = pool_clone.clone();
                    if let Ok((stats, realtime)) = request_logs::get_stats_for_broadcast(&pool).await {
                        let message = json!({
                            "type": "stats_update",
                            "stats": {
                                "total": stats.total,
                                "today": stats.today,
                                "avg_duration": stats.avg_duration,
                                "error_rate": stats.error_rate,
                            },
                            "realtime": {
                                "last_second_count": realtime.last_second_count,
                                "last_minute_count": realtime.last_minute_count,
                            }
                        }).to_string();

                        for entry in WS_CLIENTS.iter() {
                            entry.value().do_send(BroadcastMessage { payload: message.clone() });
                        }
                    }
                }
                _ = interval_timeline.tick() => {
                    let pool = pool_clone.clone();
                    if let Ok(timeline) = request_logs::get_timeline_for_broadcast(&pool).await {
                        let message = json!({
                            "type": "timeline_update",
                            "timeline": timeline
                        }).to_string();

                        for entry in WS_CLIENTS.iter() {
                            entry.value().do_send(BroadcastMessage { payload: message.clone() });
                        }
                    }
                }
            }
        }
    });
}