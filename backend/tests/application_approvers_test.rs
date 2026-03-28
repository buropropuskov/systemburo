mod common;

use common::auth_helper::*;
use common::setup::TestApp;

/// Get the user id for a given token by calling /users/current.
async fn get_user_id(app: &TestApp, token: &str) -> i64 {
    let response = auth_get(app, "/users/me", token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse user response");
    body["id"].as_i64().expect("User response should contain id")
}

#[tokio::test]
async fn get_approvers() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "approvers_list").await;

    let response = auth_get(&app, "/application-approvers", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_approvers_unauthorized() {
    let app = TestApp::spawn().await;
    let (user_token, _, _) = create_authenticated_user(&app, "approvers_unauth").await;

    let response = auth_get(&app, "/application-approvers", &user_token).await;
    let status = response.status().as_u16();
    assert!(
        status == 403 || status == 401,
        "Expected 403 or 401 for regular user, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn get_available_users() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "avail_users").await;

    let response = auth_get(&app, "/application-approvers/available-users", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn create_approver() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "create_approver").await;

    // Create a regular user to use as the approver target
    let (user_token, _, _) = create_authenticated_user(&app, "approver_target").await;
    let user_id = get_user_id(&app, &user_token).await;

    let payload = serde_json::json!({ "user_id": user_id });
    let response = auth_post(&app, "/application-approvers", &admin_token, &payload).await;
    assert!(response.status().is_success());

    app.cleanup().await;
}

#[tokio::test]
async fn delete_approver() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "del_approver").await;

    // Create a regular user and make them an approver
    let (user_token, _, _) = create_authenticated_user(&app, "del_approver_target").await;
    let user_id = get_user_id(&app, &user_token).await;

    let payload = serde_json::json!({ "user_id": user_id });
    let create_response = auth_post(&app, "/application-approvers", &admin_token, &payload).await;
    assert!(create_response.status().is_success());

    // Get the approver id from the list
    let list_response = auth_get(&app, "/application-approvers", &admin_token).await;
    let list: serde_json::Value = list_response.json().await.unwrap();
    let approvers = list.as_array().expect("Should be array");
    let approver = approvers.iter().find(|a| a["user_id"].as_i64() == Some(user_id)).expect("Approver not found");
    let approver_id = approver["id"].as_i64().expect("Approver should have id");

    // Delete the approver
    let path = format!("/application-approvers/{}", approver_id);
    let response = auth_delete(&app, &path, &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}
