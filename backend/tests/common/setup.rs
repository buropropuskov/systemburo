use sqlx::{PgPool, PgPoolOptions, Executor};
use std::net::TcpListener;
use uuid::Uuid;

pub struct TestApp {
    pub address: String,
    pub port: u16,
    pub db_pool: PgPool,
    pub db_name: String,
    pub api_client: reqwest::Client,
}

impl TestApp {
    pub async fn spawn() -> Self {
        let db_name = format!("test_{}", Uuid::new_v4().to_string().replace("-", ""));

        let db_host = std::env::var("DB_TEST_HOST").unwrap_or_else(|_| "localhost".to_string());
        let db_user = std::env::var("DB_TEST_USER").unwrap_or_else(|_| "postgres".to_string());
        let db_pass = std::env::var("DB_TEST_PASS").unwrap_or_else(|_| "123".to_string());

        let maintenance_url = format!("postgres://{}:{}@{}/postgres", db_user, db_pass, db_host);
        let maintenance_pool = PgPoolOptions::new()
            .max_connections(2)
            .connect(&maintenance_url)
            .await
            .expect("Failed to connect to PostgreSQL");

        maintenance_pool
            .execute(format!("CREATE DATABASE \"{}\"", db_name).as_str())
            .await
            .expect("Failed to create test DB");

        let db_url = format!("postgres://{}:{}@{}/{}", db_user, db_pass, db_host, db_name);
        let db_pool = PgPoolOptions::new()
            .max_connections(5)
            .connect(&db_url)
            .await
            .expect("Failed to connect to test DB");

        sqlx::migrate!("./migrations")
            .run(&db_pool)
            .await
            .expect("Failed to run migrations");

        let listener = TcpListener::bind("127.0.0.1:0").expect("Failed to bind port");
        let port = listener.local_addr().unwrap().port();
        drop(listener);

        let pool_clone = db_pool.clone();
        let _ = tokio::spawn(async move {
            use actix_web::{App, HttpServer, web};
            use actix_cors::Cors;

            HttpServer::new(move || {
                App::new()
                    .wrap(
                        Cors::default()
                            .allow_any_origin()
                            .allow_any_method()
                            .allow_any_header()
                            .supports_credentials()
                            .max_age(3600)
                    )
                    .app_data(web::Data::new(pool_clone.clone()))
                    .configure(backend::routes::config)
            })
            .bind(("127.0.0.1", port))
            .expect("Failed to start test server")
            .run()
            .await
        });

        tokio::time::sleep(std::time::Duration::from_millis(500)).await;

        let api_client = reqwest::Client::builder()
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .unwrap();

        TestApp {
            address: format!("http://127.0.0.1:{}", port),
            port,
            db_pool,
            db_name,
            api_client,
        }
    }

    pub async fn cleanup(self) {
        let db_host = std::env::var("DB_TEST_HOST").unwrap_or_else(|_| "localhost".to_string());
        let db_user = std::env::var("DB_TEST_USER").unwrap_or_else(|_| "postgres".to_string());
        let db_pass = std::env::var("DB_TEST_PASS").unwrap_or_else(|_| "123".to_string());

        self.db_pool.close().await;

        let maintenance_url = format!("postgres://{}:{}@{}/postgres", db_user, db_pass, db_host);
        let maintenance_pool = PgPoolOptions::new()
            .max_connections(2)
            .connect(&maintenance_url)
            .await
            .expect("Failed to connect for cleanup");

        let _ = maintenance_pool
            .execute(
                format!(
                    "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '{}'",
                    self.db_name
                )
                .as_str(),
            )
            .await;

        let _ = maintenance_pool
            .execute(format!("DROP DATABASE IF EXISTS \"{}\"", self.db_name).as_str())
            .await;
    }
}
