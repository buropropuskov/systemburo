mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn get_all_users_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "allusers").await;

    let response = auth_get(&app, "/users/all", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_all_users_unauthorized() {
    let app = TestApp::spawn().await;
    let (user_token, _, _) = create_authenticated_user(&app, "unauth").await;

    let response = auth_get(&app, "/users/all", &user_token).await;
    let status = response.status().as_u16();
    assert!(
        status == 403 || status == 401,
        "Expected 403 or 401, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn get_current_user() {
    let app = TestApp::spawn().await;
    let (token, _, username) = create_authenticated_user(&app, "current").await;

    let response = auth_get(&app, "/users/me", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert_eq!(body["username"].as_str().unwrap(), username);

    app.cleanup().await;
}

#[tokio::test]
async fn update_user_type_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "updtype").await;
    let (_, _, target_username) = create_authenticated_user(&app, "target_type").await;

    let payload = serde_json::json!({ "type_id": 2 });
    let path = format!("/users/{}/type", target_username);
    let response = auth_put(&app, &path, &admin_token, &payload).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn update_user_password_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "updpass").await;
    let (_, _, target_username) = create_authenticated_user(&app, "target_pass").await;

    let new_password = "NewPass123!";
    let payload = serde_json::json!({ "password": new_password });
    let path = format!("/users/{}/password", target_username);
    let response = auth_put(&app, &path, &admin_token, &payload).await;
    assert_eq!(response.status().as_u16(), 200);

    // Verify login with new password succeeds (login returns (token, refresh) or panics)
    let (new_token, _) = login(&app, &target_username, new_password).await;
    assert!(!new_token.is_empty());

    app.cleanup().await;
}

#[tokio::test]
async fn update_user_info_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "updinfo").await;
    let (_, _, target_username) = create_authenticated_user(&app, "target_info").await;

    let payload = serde_json::json!({
        "last_name": "Updated",
        "first_name": "Name"
    });
    let path = format!("/users/{}/info", target_username);
    let response = auth_put(&app, &path, &admin_token, &payload).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_user_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "delusr").await;
    let (_, _, target_username) = create_authenticated_user(&app, "target_del").await;

    let path = format!("/users/{}", target_username);
    let response = auth_delete(&app, &path, &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_user_unauthorized() {
    let app = TestApp::spawn().await;
    let (user_token, _, _) = create_authenticated_user(&app, "delunauth").await;
    let (_, _, target_username) = create_authenticated_user(&app, "target_delunauth").await;

    let path = format!("/users/{}", target_username);
    let response = auth_delete(&app, &path, &user_token).await;
    let status = response.status().as_u16();
    assert!(
        status == 403 || status == 401,
        "Expected 403 or 401, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn update_user_organization() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "upd_org").await;
    let (_, _, target) = create_authenticated_user(&app, "target_org").await;

    let response = auth_put(
        &app,
        &format!("/users/{}/organization", target),
        &admin_token,
        &serde_json::json!({"organization_id": 1}),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn update_user_company() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "upd_comp").await;
    let (_, _, target) = create_authenticated_user(&app, "target_comp").await;

    let response = auth_put(
        &app,
        &format!("/users/{}/company", target),
        &admin_token,
        &serde_json::json!({"company_id": 1}),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn get_all_users_response_structure() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "users_struct").await;
    let response = auth_get(&app, "/users/all", &admin_token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    let users = body.as_array().unwrap();
    assert!(!users.is_empty());
    let u = &users[0];
    assert!(u["id"].is_number(), "should have id");
    assert!(u["username"].is_string(), "should have username");
    assert!(u.get("type_id").is_some(), "should have type_id");
    assert!(u.get("user_type").is_some(), "should have user_type");
    app.cleanup().await;
}

#[tokio::test]
async fn delete_nonexistent_user() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "del_ghost").await;
    let response = auth_delete(&app, "/users/nonexistent_user_xyz", &admin_token).await;
    let status = response.status().as_u16();
    // Idempotent delete — returns 200 even if user doesn't exist. Preserve this contract in Go.
    assert!(status == 200 || status == 404, "Expected 200 or 404, got {}", status);
    app.cleanup().await;
}
