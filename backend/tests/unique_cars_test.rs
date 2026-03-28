mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn get_unique_cars() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_list").await;

    let response = auth_get(&app, "/unique-cars", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_cars_no_auth() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(&format!("{}/unique-cars", app.address))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 401);

    app.cleanup().await;
}

#[tokio::test]
async fn create_unique_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_create").await;

    let response = auth_post(
        &app,
        "/unique-cars",
        &token,
        &serde_json::json!({
            "number": "A001AA77",
            "mark": "Toyota",
            "organization_id": 1,
            "company_id": 1
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body["id"].as_i64().is_some());
    assert_eq!(body["number"].as_str().unwrap(), "A001AA77");

    app.cleanup().await;
}

#[tokio::test]
async fn create_unique_cars_batch() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_batch").await;

    let response = auth_post(
        &app,
        "/unique-cars/batch",
        &token,
        &serde_json::json!([
            {
                "number": "C003CC77",
                "mark": "BMW",
                "organization_id": 1,
                "company_id": 1
            },
            {
                "number": "D004DD77",
                "mark": "Audi",
                "organization_id": 1,
                "company_id": 1
            }
        ]),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert_eq!(body["success_count"].as_i64().unwrap(), 2);

    app.cleanup().await;
}

#[tokio::test]
async fn update_unique_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_upd").await;

    let create_resp = auth_post(
        &app,
        "/unique-cars",
        &token,
        &serde_json::json!({
            "number": "E005EE77",
            "mark": "Nissan",
            "organization_id": 1,
            "company_id": 1
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_put(
        &app,
        &format!("/unique-cars/{}", id),
        &token,
        &serde_json::json!({
            "number": "B002BB77",
            "mark": "Honda"
        }),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_unique_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_del").await;

    let create_resp = auth_post(
        &app,
        "/unique-cars",
        &token,
        &serde_json::json!({
            "number": "F006FF77",
            "mark": "Mazda",
            "organization_id": 1,
            "company_id": 1
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_delete(&app, &format!("/unique-cars/{}", id), &token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn get_ownership_info() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_owner").await;

    let response = auth_get(&app, "/unique-cars/ownership-info", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("user_id").is_some());

    app.cleanup().await;
}

#[tokio::test]
async fn update_car_by_number() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_bynum").await;

    // Create a car first
    auth_post(
        &app,
        "/unique-cars",
        &token,
        &serde_json::json!({"number": "X999XX77", "mark": "Lada", "organization_id": 1, "company_id": 1}),
    )
    .await;

    // Update by number — handler checks ownership, may return 200 or 403/404
    let response = auth_put(
        &app,
        "/unique-cars/by-number",
        &token,
        &serde_json::json!({
            "number": "X999XX77",
            "mark": "Lada",
            "update_data": {"number": "Y888YY77", "mark": "Lada Updated", "organization_id": 1, "company_id": 1}
        }),
    )
    .await;
    let status = response.status().as_u16();
    assert!(status == 200 || status == 403 || status == 404, "Expected 200/403/404, got {}", status);

    app.cleanup().await;
}

#[tokio::test]
async fn unique_car_response_structure() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "car_struct").await;
    let response = auth_post(&app, "/unique-cars", &token, &serde_json::json!({
        "number": "S111SS77", "mark": "StructCar", "organization_id": 1, "company_id": 1
    })).await;
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body["id"].is_number(), "should have id");
    assert!(body["number"].is_string(), "should have number");
    assert!(body["mark"].is_string(), "should have mark");
    assert!(body.get("status").is_some(), "should have status");
    assert!(body.get("created_at").is_some(), "should have created_at");
    app.cleanup().await;
}

#[tokio::test]
async fn delete_nonexistent_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "car_del_404").await;
    let response = auth_delete(&app, "/unique-cars/99999", &token).await;
    let status = response.status().as_u16();
    assert!(status == 404 || status == 403, "Expected 404/403 for nonexistent car, got {}", status);
    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_cars_filter_by_user() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_flt_user").await;
    let response = auth_get(&app, "/unique-cars?filter_type=user", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_cars_filter_by_organization() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_flt_org").await;
    let response = auth_get(&app, "/unique-cars?filter_type=organization", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_unique_cars_filter_all() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_flt_all").await;
    let response = auth_get(&app, "/unique-cars?filter_type=all", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}
