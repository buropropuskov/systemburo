use sqlx::{PgPool, Executor};
use sqlx::postgres::PgPoolOptions;
use std::net::TcpListener;
use std::sync::Once;
use uuid::Uuid;

static INIT_TEMPLATE: Once = Once::new();
static TEMPLATE_DB_NAME: &str = "test_template_systemburo";

pub struct TestApp {
    pub address: String,
    pub port: u16,
    pub db_pool: PgPool,
    pub db_name: String,
    pub api_client: reqwest::Client,
}

fn get_maintenance_url() -> String {
    let db_host = std::env::var("DB_TEST_HOST").unwrap_or_else(|_| "localhost".to_string());
    let db_user = std::env::var("DB_TEST_USER").unwrap_or_else(|_| "postgres".to_string());
    let db_pass = std::env::var("DB_TEST_PASS").unwrap_or_else(|_| "123".to_string());
    format!("postgres://{}:{}@{}/postgres", db_user, db_pass, db_host)
}

fn get_db_url(db_name: &str) -> String {
    let db_host = std::env::var("DB_TEST_HOST").unwrap_or_else(|_| "localhost".to_string());
    let db_user = std::env::var("DB_TEST_USER").unwrap_or_else(|_| "postgres".to_string());
    let db_pass = std::env::var("DB_TEST_PASS").unwrap_or_else(|_| "123".to_string());
    format!("postgres://{}:{}@{}/{}", db_user, db_pass, db_host, db_name)
}

/// Create template DB once with all migrations. Subsequent tests clone from it.
/// Uses an advisory lock to prevent race conditions between parallel tests.
async fn ensure_template_db() {
    let maintenance_pool = PgPoolOptions::new()
        .max_connections(2)
        .connect(&get_maintenance_url())
        .await
        .expect("Failed to connect to PostgreSQL");

    // Advisory lock to prevent parallel template creation
    let _lock = sqlx::query("SELECT pg_advisory_lock(12345678)")
        .execute(&maintenance_pool)
        .await;

    // Check if template already exists
    let exists = sqlx::query_scalar::<_, bool>(
        "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
    )
    .bind(TEMPLATE_DB_NAME)
    .fetch_one(&maintenance_pool)
    .await
    .unwrap_or(false);

    if !exists {
        maintenance_pool
            .execute(format!("CREATE DATABASE \"{}\"", TEMPLATE_DB_NAME).as_str())
            .await
            .expect("Failed to create template DB");

        let template_pool = PgPoolOptions::new()
            .max_connections(2)
            .connect(&get_db_url(TEMPLATE_DB_NAME))
            .await
            .expect("Failed to connect to template DB");

        sqlx::migrate!("./migrations")
            .run(&template_pool)
            .await
            .expect("Failed to run migrations on template DB");

        template_pool.close().await;
    }

    let _ = sqlx::query("SELECT pg_advisory_unlock(12345678)")
        .execute(&maintenance_pool)
        .await;
}

impl TestApp {
    pub async fn spawn() -> Self {
        // Ensure template DB exists (idempotent)
        ensure_template_db().await;

        let db_name = format!("test_{}", Uuid::new_v4().to_string().replace("-", ""));

        let maintenance_pool = PgPoolOptions::new()
            .max_connections(2)
            .connect(&get_maintenance_url())
            .await
            .expect("Failed to connect to PostgreSQL");

        // Clone from template — instant instead of running 6000 lines of migrations
        maintenance_pool
            .execute(
                format!(
                    "CREATE DATABASE \"{}\" TEMPLATE \"{}\"",
                    db_name, TEMPLATE_DB_NAME
                )
                .as_str(),
            )
            .await
            .expect("Failed to create test DB from template");

        let db_pool = PgPoolOptions::new()
            .max_connections(5)
            .connect(&get_db_url(&db_name))
            .await
            .expect("Failed to connect to test DB");

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

        // Poll until server is ready instead of blind sleep
        let api_client = reqwest::Client::builder()
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .unwrap();

        let addr = format!("http://127.0.0.1:{}", port);
        for _ in 0..50 {
            if api_client
                .get(&format!("{}/user-types", addr))
                .send()
                .await
                .is_ok()
            {
                break;
            }
            tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        }

        TestApp {
            address: addr,
            port,
            db_pool,
            db_name,
            api_client,
        }
    }

    pub async fn cleanup(self) {
        self.db_pool.close().await;

        let maintenance_pool = PgPoolOptions::new()
            .max_connections(2)
            .connect(&get_maintenance_url())
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
