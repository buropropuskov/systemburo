mod common;

use common::setup::TestApp;
use common::auth_helper::*;

// === Регистрация (3 теста) ===

#[tokio::test]
async fn register_success() {
    let app = TestApp::spawn().await;
    let response = register_user(&app, "newuser", "Password123!").await;
    assert_eq!(response.status(), 200);
    app.cleanup().await;
}

#[tokio::test]
async fn register_duplicate_username() {
    let app = TestApp::spawn().await;
    register_user(&app, "dupuser", "Password123!").await;
    let r2 = register_user(&app, "dupuser", "Password123!").await;
    assert_eq!(r2.status(), 400);
    app.cleanup().await;
}

#[tokio::test]
async fn register_missing_fields() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/register", app.address))
        .json(&serde_json::json!({"username": "incomplete"}))
        .send().await.unwrap();
    assert!(response.status().is_client_error() || response.status().is_server_error());
    app.cleanup().await;
}

// === Логин (3 теста) ===

#[tokio::test]
async fn login_success() {
    let app = TestApp::spawn().await;
    register_user(&app, "loginuser", "Password123!").await;
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&serde_json::json!({"username": "loginuser", "password": "Password123!"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("token").is_some());
    assert!(body.get("refreshToken").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn login_wrong_password() {
    let app = TestApp::spawn().await;
    register_user(&app, "wrongpwd", "CorrectPass123!").await;
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&serde_json::json!({"username": "wrongpwd", "password": "WrongPass!"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

#[tokio::test]
async fn login_nonexistent_user() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&serde_json::json!({"username": "noexist", "password": "AnyPass123!"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

// === Refresh Token (3 теста) ===

#[tokio::test]
async fn refresh_token_success() {
    let app = TestApp::spawn().await;
    register_user(&app, "refreshuser", "Password123!").await;
    let (_, refresh_token) = login(&app, "refreshuser", "Password123!").await;
    let response = app.api_client
        .post(&format!("{}/refresh-token", app.address))
        .json(&serde_json::json!({"refresh_token": refresh_token}))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("token").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn refresh_token_invalid() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/refresh-token", app.address))
        .json(&serde_json::json!({"refresh_token": "invalid.jwt.token"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

#[tokio::test]
async fn logout_then_refresh_fails() {
    let app = TestApp::spawn().await;
    register_user(&app, "logoutuser", "Password123!").await;
    let (access_token, refresh_token) = login(&app, "logoutuser", "Password123!").await;
    // Logout
    app.api_client.post(&format!("{}/logout", app.address))
        .header("Authorization", format!("Bearer {}", access_token))
        .json(&serde_json::json!({"refresh_token": refresh_token.clone()}))
        .send().await.unwrap();
    // Refresh should fail
    let response = app.api_client
        .post(&format!("{}/refresh-token", app.address))
        .json(&serde_json::json!({"refresh_token": refresh_token}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

// === Logout (2 теста) ===

#[tokio::test]
async fn logout_success() {
    let app = TestApp::spawn().await;
    register_user(&app, "logoutok", "Password123!").await;
    let (access_token, refresh_token) = login(&app, "logoutok", "Password123!").await;
    let response = app.api_client
        .post(&format!("{}/logout", app.address))
        .header("Authorization", format!("Bearer {}", access_token))
        .json(&serde_json::json!({"refresh_token": refresh_token}))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    app.cleanup().await;
}

#[tokio::test]
async fn logout_no_auth_header() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/logout", app.address))
        .json(&serde_json::json!({"refresh_token": "some.token"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

// === User Data (3 теста) ===

#[tokio::test]
async fn get_user_data_authenticated() {
    let app = TestApp::spawn().await;
    register_user(&app, "datauser", "Password123!").await;
    let (token, _) = login(&app, "datauser", "Password123!").await;
    let response = auth_get(&app, "/user-data", &token).await;
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert_eq!(body["username"], "datauser");
    app.cleanup().await;
}

#[tokio::test]
async fn get_user_data_no_token() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(&format!("{}/user-data", app.address))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

#[tokio::test]
async fn get_user_data_invalid_token() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(&format!("{}/user-data", app.address))
        .header("Authorization", "Bearer invalid.jwt.token")
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

// === Users/Me + User Types (2 теста) ===

#[tokio::test]
async fn get_user_me() {
    let app = TestApp::spawn().await;
    register_user(&app, "meuser", "Password123!").await;
    let (token, _) = login(&app, "meuser", "Password123!").await;
    let response = auth_get(&app, "/users/me", &token).await;
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert_eq!(body["username"], "meuser");
    assert!(body.get("type_id").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn get_user_types() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(&format!("{}/user-types", app.address))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    let types = body.as_array().expect("Should be array");
    assert!(types.len() >= 1);
    assert!(types[0].get("id").is_some());
    assert!(types[0].get("name").is_some());
    assert!(types[0].get("code").is_some());
    app.cleanup().await;
}

// === Response Structure Tests ===

#[tokio::test]
async fn login_response_structure() {
    let app = TestApp::spawn().await;
    register_user(&app, "structuser", "Password123!").await;
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&serde_json::json!({"username": "structuser", "password": "Password123!"}))
        .send().await.unwrap();
    let body: serde_json::Value = response.json().await.unwrap();
    // Verify all expected fields exist
    assert!(body["token"].is_string(), "token should be string");
    assert!(body["refreshToken"].is_string(), "refreshToken should be string");
    assert!(body["organization"].is_string(), "organization should be string");
    assert!(body["organization_id"].is_number(), "organization_id should be number");
    assert!(body["company"].is_string(), "company should be string");
    assert!(body["company_id"].is_number(), "company_id should be number");
    assert!(body["type_id"].is_number(), "type_id should be number");
    assert!(body["user_type"].is_string(), "user_type should be string");
    app.cleanup().await;
}

#[tokio::test]
async fn user_data_response_structure() {
    let app = TestApp::spawn().await;
    register_user(&app, "structdata", "Password123!").await;
    let (token, _) = login(&app, "structdata", "Password123!").await;
    let response = auth_get(&app, "/user-data", &token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("username").is_some(), "should have username");
    assert!(body.get("organization").is_some(), "should have organization");
    assert!(body.get("organization_id").is_some(), "should have organization_id");
    assert!(body.get("company").is_some(), "should have company");
    assert!(body.get("company_id").is_some(), "should have company_id");
    app.cleanup().await;
}

#[tokio::test]
async fn user_me_response_structure() {
    let app = TestApp::spawn().await;
    register_user(&app, "structme", "Password123!").await;
    let (token, _) = login(&app, "structme", "Password123!").await;
    let response = auth_get(&app, "/users/me", &token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body["id"].is_number(), "should have id");
    assert!(body["username"].is_string(), "should have username");
    assert!(body.get("type_id").is_some(), "should have type_id");
    assert!(body.get("user_type").is_some(), "should have user_type");
    assert!(body.get("last_name").is_some(), "should have last_name");
    assert!(body.get("first_name").is_some(), "should have first_name");
    assert!(body.get("position").is_some(), "should have position");
    assert!(body.get("email").is_some(), "should have email");
    assert!(body.get("phone").is_some(), "should have phone");
    app.cleanup().await;
}

#[tokio::test]
async fn user_types_response_structure() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(&format!("{}/user-types", app.address))
        .send().await.unwrap();
    let body: serde_json::Value = response.json().await.unwrap();
    let types = body.as_array().unwrap();
    assert!(types.len() >= 6, "should have at least 6 user types");
    for t in types {
        assert!(t["id"].is_number(), "each type should have id");
        assert!(t["name"].is_string(), "each type should have name");
        assert!(t["code"].is_string(), "each type should have code");
    }
    app.cleanup().await;
}
