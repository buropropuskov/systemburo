mod common;

use common::auth_helper::*;
use common::setup::TestApp;

/// Helper: create a citizenship and return its id.
async fn create_citizenship(app: &TestApp, token: &str) -> i32 {
    let resp = auth_post(
        app,
        "/citizenships",
        token,
        &serde_json::json!({
            "name": "РФ",
            "icon": "🇷🇺",
            "patent_required": false
        }),
    )
    .await;
    let body: serde_json::Value = resp.json().await.unwrap();
    body["id"].as_i64().unwrap() as i32
}

/// Helper: create a unique employee and return the response body.
async fn create_employee(app: &TestApp, token: &str, citizenship_id: i32, suffix: &str) -> serde_json::Value {
    let resp = auth_post(
        app,
        "/unique-employees",
        token,
        &serde_json::json!({
            "last_name": format!("Иванов{}", suffix),
            "first_name": "Иван",
            "citizenship_id": citizenship_id,
            "position": "Водитель",
            "passport_series_number": format!("12345678{}", suffix),
            "organization_id": 1,
            "company_id": 1
        }),
    )
    .await;
    assert_eq!(resp.status().as_u16(), 200);
    resp.json().await.unwrap()
}

#[tokio::test]
async fn get_unique_employees() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_list").await;

    let response = auth_get(&app, "/unique-employees", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_employees_no_auth() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(&format!("{}/unique-employees", app.address))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 401);

    app.cleanup().await;
}

#[tokio::test]
async fn create_unique_employee() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_create").await;

    let citizenship_id = create_citizenship(&app, &token).await;

    let body = create_employee(&app, &token, citizenship_id, "01").await;
    assert!(body["id"].as_i64().is_some());

    app.cleanup().await;
}

#[tokio::test]
async fn update_unique_employee() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_upd").await;

    let citizenship_id = create_citizenship(&app, &token).await;
    let body = create_employee(&app, &token, citizenship_id, "02").await;
    let id = body["id"].as_i64().unwrap();

    let response = auth_put(
        &app,
        &format!("/unique-employees/{}", id),
        &token,
        &serde_json::json!({
            "last_name": "Петров",
            "first_name": "Пётр",
            "citizenship_id": citizenship_id,
            "position": "Менеджер",
            "passport_series_number": "9876543210",
            "organization_id": 1,
            "company_id": 1
        }),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_unique_employee() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_del").await;

    let citizenship_id = create_citizenship(&app, &token).await;
    let body = create_employee(&app, &token, citizenship_id, "03").await;
    let id = body["id"].as_i64().unwrap();

    let response = auth_delete(&app, &format!("/unique-employees/{}", id), &token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn get_employee_ownership_info() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_owner").await;

    let response = auth_get(&app, "/unique-employees/ownership-info", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("user_id").is_some());

    app.cleanup().await;
}

#[tokio::test]
async fn get_active_employees_for_table() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_active").await;

    // table_id=1 may not exist, but endpoint should return 200 with empty array
    let response = auth_get(&app, "/employees/active-for-table/1", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn unique_employee_response_structure() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_struct").await;
    let citizenship_id = create_citizenship(&app, &token).await;
    let body = create_employee(&app, &token, citizenship_id, "struct").await;
    assert!(body["id"].is_number(), "should have id");
    assert!(body.get("last_name").is_some(), "should have last_name");
    assert!(body.get("first_name").is_some(), "should have first_name");
    assert!(body.get("status").is_some(), "should have status");
    assert!(body.get("created_at").is_some(), "should have created_at");
    app.cleanup().await;
}

#[tokio::test]
async fn delete_nonexistent_employee() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_del_404").await;
    let response = auth_delete(&app, "/unique-employees/99999", &token).await;
    let status = response.status().as_u16();
    assert!(status == 404 || status == 403, "Expected 404/403, got {}", status);
    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_employees_filter_by_user() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_flt_user").await;
    let response = auth_get(&app, "/unique-employees?filter_type=user", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_employees_filter_by_organization() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_flt_org").await;
    let response = auth_get(&app, "/unique-employees?filter_type=organization", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_employees_filter_all() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "emp_flt_all").await;
    let response = auth_get(&app, "/unique-employees?filter_type=all", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}
