mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn get_all_companies() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "getcomps").await;

    let response = auth_get(&app, "/companies", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    let companies = body.as_array().expect("Response should be an array");
    assert!(
        companies.len() >= 1,
        "Should have at least 1 seed company"
    );

    app.cleanup().await;
}

#[tokio::test]
async fn create_company_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "createcomp").await;

    let payload = serde_json::json!({ "name": "Test Company" });
    let response = auth_post(&app, "/companies", &admin_token, &payload).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body["id"].is_number(), "Response should contain id");
    assert_eq!(body["name"].as_str().unwrap(), "Test Company");

    app.cleanup().await;
}

#[tokio::test]
async fn create_company_unauthorized() {
    let app = TestApp::spawn().await;
    let (user_token, _, _) = create_authenticated_user(&app, "compunauth").await;

    let payload = serde_json::json!({ "name": "Unauthorized Company" });
    let response = auth_post(&app, "/companies", &user_token, &payload).await;
    let status = response.status().as_u16();
    assert!(
        status == 403 || status == 401,
        "Expected 403 or 401, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn update_company_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "updcomp").await;

    // Create a company first
    let payload = serde_json::json!({ "name": "Company To Update" });
    let create_response = auth_post(&app, "/companies", &admin_token, &payload).await;
    assert_eq!(create_response.status().as_u16(), 200);
    let created: serde_json::Value = create_response.json().await.expect("Failed to parse");
    let company_id = created["id"].as_i64().expect("Should have id");

    // Update the company
    let update_payload = serde_json::json!({ "name": "Updated" });
    let path = format!("/companies/{}", company_id);
    let response = auth_put(&app, &path, &admin_token, &update_payload).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_company_as_admin() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "delcomp").await;

    // Create a company first
    let payload = serde_json::json!({ "name": "Company To Delete" });
    let create_response = auth_post(&app, "/companies", &admin_token, &payload).await;
    assert_eq!(create_response.status().as_u16(), 200);
    let created: serde_json::Value = create_response.json().await.expect("Failed to parse");
    let company_id = created["id"].as_i64().expect("Should have id");

    // Delete the company
    let path = format!("/companies/{}", company_id);
    let response = auth_delete(&app, &path, &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn get_company_users() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "compusers").await;

    let response = auth_get(&app, "/companies/1/users", &admin_token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_companies_with_users() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "compwithusers").await;

    let response = auth_get(&app, "/companies/with-users", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_companies_extended() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!(
            "{}/companies/with-users-extended",
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
async fn get_company_unload_places() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!(
            "{}/companies/1/unload-places",
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
async fn update_company_unload_places() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "comp_upd_places").await;

    let response = auth_put(
        &app,
        "/companies/1/unload-places",
        &admin_token,
        &serde_json::json!({"unload_place_ids": []}),
    )
    .await;
    assert!(response.status().is_success());

    app.cleanup().await;
}

#[tokio::test]
async fn update_company_users() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "comp_upd_users").await;
    let (_, _, username) = create_authenticated_user(&app, "comp_user_target").await;

    let response = auth_put(
        &app,
        "/companies/1/users",
        &admin_token,
        &serde_json::json!({"users": [{"username": username, "is_primary": false, "required_approval": false}]}),
    )
    .await;
    assert!(response.status().is_success());

    app.cleanup().await;
}

#[tokio::test]
async fn get_company_tables() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!("{}/companies/1/tables", &app.address))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn update_company_tables() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "comp_upd_tables").await;

    let response = auth_put(
        &app,
        "/companies/1/tables",
        &admin_token,
        &serde_json::json!({"table_ids": []}),
    )
    .await;
    assert!(response.status().is_success());

    app.cleanup().await;
}

// === Response Structure & Negative Cases ===

#[tokio::test]
async fn company_response_structure() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(format!("{}/companies", &app.address))
        .send().await.unwrap();
    let body: serde_json::Value = response.json().await.unwrap();
    let comps = body.as_array().unwrap();
    assert!(!comps.is_empty());
    assert!(comps[0]["id"].is_number(), "should have id");
    assert!(comps[0]["name"].is_string(), "should have name");
    app.cleanup().await;
}

#[tokio::test]
async fn delete_nonexistent_company() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "comp_del_404").await;
    let response = auth_delete(&app, "/companies/99999", &admin_token).await;
    // Handler returns 200 even for nonexistent — this is the contract to preserve in Go
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn update_nonexistent_company() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "comp_upd_404").await;
    let response = auth_put(&app, "/companies/99999", &admin_token, &serde_json::json!({"name": "Ghost"})).await;
    assert_eq!(response.status().as_u16(), 404);
    app.cleanup().await;
}
