use crate::common::setup::TestApp;
use serde_json::{json, Value};

pub async fn register_user(app: &TestApp, username: &str, password: &str) -> reqwest::Response {
    app.api_client
        .post(&format!("{}/register", app.address))
        .json(&json!({
            "username": username,
            "password": password,
            "organization_id": 1,
            "company_id": 1,
            "type_id": 1,
            "last_name": null,
            "first_name": null,
            "middle_name": null,
            "position": null,
            "email": null,
            "phone": null
        }))
        .send()
        .await
        .expect("Failed to send register request")
}

pub async fn login(app: &TestApp, username: &str, password: &str) -> (String, String) {
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&json!({"username": username, "password": password}))
        .send()
        .await
        .expect("Failed to send login request");

    let body: Value = response.json().await.expect("Failed to read JSON");
    let token = body["token"].as_str().expect("No token").to_string();
    let refresh = body["refreshToken"].as_str().expect("No refreshToken").to_string();
    (token, refresh)
}

pub async fn create_authenticated_user(app: &TestApp, suffix: &str) -> (String, String, String) {
    let username = format!("testuser_{}", suffix);
    let password = "TestPassword123!";
    register_user(app, &username, password).await;
    let (token, refresh) = login(app, &username, password).await;
    (token, refresh, username)
}

pub async fn auth_get(app: &TestApp, path: &str, token: &str) -> reqwest::Response {
    app.api_client
        .get(&format!("{}{}", app.address, path))
        .header("Authorization", format!("Bearer {}", token))
        .send()
        .await
        .expect("Failed to send auth GET")
}

pub async fn auth_post(app: &TestApp, path: &str, token: &str, body: &Value) -> reqwest::Response {
    app.api_client
        .post(&format!("{}{}", app.address, path))
        .header("Authorization", format!("Bearer {}", token))
        .json(body)
        .send()
        .await
        .expect("Failed to send auth POST")
}

pub async fn auth_put(app: &TestApp, path: &str, token: &str, body: &Value) -> reqwest::Response {
    app.api_client
        .put(&format!("{}{}", app.address, path))
        .header("Authorization", format!("Bearer {}", token))
        .json(body)
        .send()
        .await
        .expect("Failed to send auth PUT")
}

pub async fn auth_delete(app: &TestApp, path: &str, token: &str) -> reqwest::Response {
    app.api_client
        .delete(&format!("{}{}", app.address, path))
        .header("Authorization", format!("Bearer {}", token))
        .send()
        .await
        .expect("Failed to send auth DELETE")
}
