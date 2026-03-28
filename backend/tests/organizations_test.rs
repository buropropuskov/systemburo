mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn get_all_organizations() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "getorgs").await;

    let response = auth_get(&app, "/organizations", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    let orgs = body.as_array().expect("Response should be an array");
    assert!(orgs.len() >= 1, "Should have at least 1 seed organization");

    app.cleanup().await;
}

#[tokio::test]
async fn create_organization_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "createorg").await;

    let payload = serde_json::json!({ "name": "Test Org" });
    let response = auth_post(&app, "/organizations", &admin_token, &payload).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body["id"].is_number(), "Response should contain id");
    assert_eq!(body["name"].as_str().unwrap(), "Test Org");

    app.cleanup().await;
}

#[tokio::test]
async fn create_organization_unauthorized() {
    let app = TestApp::spawn().await;
    let (user_token, _, _) = create_authenticated_user(&app, "orgunauth").await;

    let payload = serde_json::json!({ "name": "Unauthorized Org" });
    let response = auth_post(&app, "/organizations", &user_token, &payload).await;
    let status = response.status().as_u16();
    assert!(
        status == 403 || status == 401,
        "Expected 403 or 401, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn update_organization_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "updorg").await;

    // Create an organization first
    let payload = serde_json::json!({ "name": "Org To Update" });
    let create_response = auth_post(&app, "/organizations", &admin_token, &payload).await;
    assert_eq!(create_response.status().as_u16(), 200);
    let created: serde_json::Value = create_response.json().await.expect("Failed to parse");
    let org_id = created["id"].as_i64().expect("Should have id");

    // Update the organization
    let update_payload = serde_json::json!({ "name": "Updated" });
    let path = format!("/organizations/{}", org_id);
    let response = auth_put(&app, &path, &admin_token, &update_payload).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_organization_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "delorg").await;

    // Create an organization first
    let payload = serde_json::json!({ "name": "Org To Delete" });
    let create_response = auth_post(&app, "/organizations", &admin_token, &payload).await;
    assert_eq!(create_response.status().as_u16(), 200);
    let created: serde_json::Value = create_response.json().await.expect("Failed to parse");
    let org_id = created["id"].as_i64().expect("Should have id");

    // Delete the organization
    let path = format!("/organizations/{}", org_id);
    let response = auth_delete(&app, &path, &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn get_organization_users() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "orgusers").await;

    let response = auth_get(&app, "/organizations/1/users", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_organizations_with_users() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "orgwithusers").await;

    let response = auth_get(&app, "/organizations/with-users", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_current_organization() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "get_org").await;

    let response = auth_get(&app, "/get-organization", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("organization").is_some());

    app.cleanup().await;
}

#[tokio::test]
async fn get_organizations_extended() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!(
            "{}/organizations/with-users-extended",
            &app.address
        ))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn get_organization_unload_places() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!(
            "{}/organizations/1/unload-places",
            &app.address
        ))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn update_organization_unload_places() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "org_upd_places").await;

    let response = auth_put(
        &app,
        "/organizations/1/unload-places",
        &admin_token,
        &serde_json::json!({"unload_place_ids": []}),
    )
    .await;
    assert!(response.status().is_success());

    app.cleanup().await;
}

#[tokio::test]
async fn update_organization_users() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "org_upd_users").await;
    let (_, _, username) = create_authenticated_user(&app, "org_user_target").await;

    let response = auth_put(
        &app,
        "/organizations/1/users",
        &admin_token,
        &serde_json::json!({"users": [{"username": username, "is_primary": false, "required_approval": false}]}),
    )
    .await;
    assert!(response.status().is_success());

    app.cleanup().await;
}

#[tokio::test]
async fn get_organization_tables() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!("{}/organizations/1/tables", &app.address))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn update_organization_tables() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "org_upd_tables").await;

    let response = auth_put(
        &app,
        "/organizations/1/tables",
        &admin_token,
        &serde_json::json!({"table_ids": []}),
    )
    .await;
    assert!(response.status().is_success());

    app.cleanup().await;
}

// === Response Structure & Negative Cases ===

#[tokio::test]
async fn organization_response_structure() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(format!("{}/organizations", &app.address))
        .send().await.unwrap();
    let body: serde_json::Value = response.json().await.unwrap();
    let orgs = body.as_array().unwrap();
    assert!(!orgs.is_empty());
    assert!(orgs[0]["id"].is_number(), "should have id");
    assert!(orgs[0]["name"].is_string(), "should have name");
    app.cleanup().await;
}

#[tokio::test]
async fn delete_nonexistent_organization() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "org_del_404").await;
    let response = auth_delete(&app, "/organizations/99999", &admin_token).await;
    // Handler may return 200 even for nonexistent — this is the contract to preserve in Go
    let status = response.status().as_u16();
    assert!(status == 200 || status == 404, "Expected 200 or 404, got {}", status);
    app.cleanup().await;
}

#[tokio::test]
async fn update_nonexistent_organization() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "org_upd_404").await;
    let response = auth_put(&app, "/organizations/99999", &admin_token, &serde_json::json!({"name": "Ghost"})).await;
    assert_eq!(response.status().as_u16(), 404);
    app.cleanup().await;
}
