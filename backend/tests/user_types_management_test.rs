mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn get_user_types_management() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "utm_get").await;

    let response = auth_get(&app, "/user-types-management", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array(), "Response should be an array");

    let arr = body.as_array().unwrap();
    assert!(!arr.is_empty(), "Should have seeded user types");

    // Verify users_count field is present
    let first = &arr[0];
    assert!(
        first.get("users_count").is_some(),
        "Each user type should have users_count"
    );

    app.cleanup().await;
}

#[tokio::test]
async fn get_user_types_management_unauthorized() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "utm_unauth").await;

    let response = auth_get(&app, "/user-types-management", &token).await;
    let status = response.status().as_u16();
    assert!(
        status == 403 || status == 401,
        "Expected 403 or 401, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn create_user_type() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "utm_create").await;

    let response = auth_post(
        &app,
        "/user-types-management",
        &admin_token,
        &serde_json::json!({
            "name": "TestType",
            "code": "test_type"
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("id").is_some(), "Response should contain id");

    app.cleanup().await;
}

#[tokio::test]
async fn update_user_type() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "utm_update").await;

    // Create first
    let create_resp = auth_post(
        &app,
        "/user-types-management",
        &admin_token,
        &serde_json::json!({
            "name": "TestType",
            "code": "test_type_upd"
        }),
    )
    .await;
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Update
    let response = auth_put(
        &app,
        &format!("/user-types-management/{}", id),
        &admin_token,
        &serde_json::json!({
            "name": "UpdatedType",
            "code": "test_type_upd"
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_user_type() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "utm_delete").await;

    // Create first
    let create_resp = auth_post(
        &app,
        "/user-types-management",
        &admin_token,
        &serde_json::json!({
            "name": "TestType",
            "code": "test_type_del"
        }),
    )
    .await;
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Delete
    let response = auth_delete(
        &app,
        &format!("/user-types-management/{}", id),
        &admin_token,
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}
