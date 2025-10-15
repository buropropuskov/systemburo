mod models;
mod routes;
mod handlers;
mod auth;
mod database;

use actix_web::{App, HttpServer};
use actix_cors::Cors;
use database::get_pool;

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    dotenv::dotenv().ok();
    let pool = get_pool().await;

    HttpServer::new(move || {
        App::new()
            // CORS политика (разрешаем все — для разработки)
            .wrap(
                Cors::default()
                    .allow_any_origin()
                    .allow_any_method()
                    .allow_any_header()
            )
            // доступ к базе через actix-web Data
            .app_data(actix_web::web::Data::new(pool.clone()))
            // подключаем все маршруты из routes.rs
            .configure(routes::config)
    })
    .bind(("127.0.0.1", 8080))?  // слушаем на localhost:8080
    .run()
    .await
}
