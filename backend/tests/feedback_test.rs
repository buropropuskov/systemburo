mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn create_feedback() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "fb_create").await;

    let response = auth_post(
        &app,
        "/feedback",
        &token,
        &serde_json::json!({
            "message": "Test feedback message for testing"
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("id").is_some(), "Response should contain id");

    app.cleanup().await;
}

#[tokio::test]
async fn create_feedback_too_short() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "fb_short").await;

    let response = auth_post(
        &app,
        "/feedback",
        &token,
        &serde_json::json!({
            "message": "short"
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 400);

    app.cleanup().await;
}

#[tokio::test]
async fn create_feedback_no_auth() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .post(format!("{}/feedback", &app.address))
        .json(&serde_json::json!({
            "message": "Test feedback message for testing"
        }))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 401);

    app.cleanup().await;
}

#[tokio::test]
async fn get_my_feedback() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "fb_my").await;

    // Create feedback first
    let create_resp = auth_post(
        &app,
        "/feedback",
        &token,
        &serde_json::json!({
            "message": "Test feedback message for testing"
        }),
    )
    .await;
    assert_eq!(create_resp.status().as_u16(), 200);

    // Get my feedback
    let response = auth_get(&app, "/feedback/my", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array(), "Response should be an array");

    let arr = body.as_array().unwrap();
    assert!(!arr.is_empty(), "Should have at least one feedback entry");

    app.cleanup().await;
}

#[tokio::test]
async fn get_all_feedback_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "fb_all").await;

    let response = auth_get(&app, "/feedback/all", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_all_feedback_unauthorized() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "fb_unauth").await;

    let response = auth_get(&app, "/feedback/all", &token).await;
    let status = response.status().as_u16();
    assert!(
        status == 403 || status == 401,
        "Expected 403 or 401, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn get_feedback_stats_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "fb_stats").await;

    let response = auth_get(&app, "/feedback/stats", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("total").is_some(), "Response should have 'total'");
    assert!(
        body.get("resolved").is_some(),
        "Response should have 'resolved'"
    );
    assert!(
        body.get("unresolved").is_some(),
        "Response should have 'unresolved'"
    );
    assert!(
        body.get("unread").is_some(),
        "Response should have 'unread'"
    );

    app.cleanup().await;
}

#[tokio::test]
async fn update_feedback_status() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "fb_status").await;

    // Create feedback as admin
    let create_resp = auth_post(
        &app,
        "/feedback",
        &admin_token,
        &serde_json::json!({
            "message": "Test feedback message for status update"
        }),
    )
    .await;
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Update status
    let response = auth_put(
        &app,
        &format!("/feedback/{}/status", id),
        &admin_token,
        &serde_json::json!({
            "status": "Решено"
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn mark_feedback_as_read() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "fb_read").await;

    // Create feedback
    let create_resp = auth_post(
        &app,
        "/feedback",
        &admin_token,
        &serde_json::json!({
            "message": "Test feedback message for read marking"
        }),
    )
    .await;
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Mark as read
    let response = auth_put(
        &app,
        &format!("/feedback/{}/read", id),
        &admin_token,
        &serde_json::json!({
            "is_read": true
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

// === Response Structure Tests ===

#[tokio::test]
async fn feedback_response_structure() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "fb_struct").await;
    // Create feedback first
    auth_post(&app, "/feedback", &admin_token, &serde_json::json!({"message": "Structure test feedback message"})).await;
    let response = auth_get(&app, "/feedback/all", &admin_token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    let items = body.as_array().unwrap();
    assert!(!items.is_empty());
    let fb = &items[0];
    assert!(fb["id"].is_number(), "should have id");
    assert!(fb["user_id"].is_number(), "should have user_id");
    assert!(fb["message"].is_string(), "should have message");
    assert!(fb["status"].is_string(), "should have status");
    assert!(fb.get("is_read").is_some(), "should have is_read");
    assert!(fb.get("created_at").is_some(), "should have created_at");
    assert!(fb.get("updated_at").is_some(), "should have updated_at");
    app.cleanup().await;
}

#[tokio::test]
async fn feedback_stats_response_structure() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "fb_stats_struct").await;
    let response = auth_get(&app, "/feedback/stats", &admin_token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("total").is_some(), "should have total");
    assert!(body.get("resolved").is_some(), "should have resolved");
    assert!(body.get("unresolved").is_some(), "should have unresolved");
    assert!(body.get("unread").is_some(), "should have unread");
    app.cleanup().await;
}
