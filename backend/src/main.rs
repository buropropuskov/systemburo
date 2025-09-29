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
            .wrap(Cors::default().allow_any_origin().allow_any_method().allow_any_header())
            .app_data(actix_web::web::Data::new(pool.clone()))
            .configure(routes::config)
    })
    .bind("127.0.0.1:8080")?
    .run()
    .await
}