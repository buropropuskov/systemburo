use sqlx::PgPool;

pub async fn create_test_organization(pool: &PgPool, name: &str) -> i32 {
    sqlx::query_scalar::<_, i32>("INSERT INTO organizations (name) VALUES ($1) RETURNING id")
        .bind(name)
        .fetch_one(pool)
        .await
        .expect("Failed to create test organization")
}

pub async fn create_test_company(pool: &PgPool, name: &str) -> i32 {
    sqlx::query_scalar::<_, i32>("INSERT INTO companies (name) VALUES ($1) RETURNING id")
        .bind(name)
        .fetch_one(pool)
        .await
        .expect("Failed to create test company")
}
