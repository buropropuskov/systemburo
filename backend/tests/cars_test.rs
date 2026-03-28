mod common;

use common::auth_helper::*;
use common::setup::TestApp;

/// Create an attachment, submit an application with a car, take it to work, return (app_id, car_id).
async fn create_active_app_with_car(app: &TestApp, admin_token: &str) -> (i64, i64) {
    // 1. Create unique_attachment
    let att_resp = app
        .api_client
        .post(format!("{}/attachments", &app.address))
        .json(&serde_json::json!({
            "attachment_type": "cars",
            "name": format!("blank_{}", uuid()),
            "display_name": "Test",
            "title": "T",
            "instruction": "I"
        }))
        .send()
        .await
        .unwrap();
    let att_id = att_resp.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    // 2. Submit application
    let submit_resp = auth_post(
        app,
        "/applications/submit-complete-application",
        admin_token,
        &serde_json::json!({
            "message": "Car test app",
            "organization": "Тестовая организация",
            "company": "Тестовая компания",
            "responsible_person": "Admin",
            "contact_phone": "79001234567",
            "data_approval": true,
            "attachments": [{
                "attachment_type": "cars",
                "attachment_name": "test",
                "attachment_display_name": "Test",
                "unique_attachment_id": att_id,
                "entry_date_from": "2026-04-01",
                "entry_date_to": "2026-12-31",
                "entry_time_from": "00:00:00",
                "entry_time_to": "23:59:59",
                "data": {
                    "vehicles": [{
                        "car_number": format!("T{}TT77", rand_num()),
                        "car_brand": "TestCar",
                        "unload_places": []
                    }]
                }
            }]
        }),
    )
    .await;
    let submit_body: serde_json::Value = submit_resp.json().await.unwrap();
    let app_id = submit_body["application_id"].as_i64().unwrap();

    // 3. Get admin user_id and create approver
    let me = auth_get(app, "/users/me", admin_token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();
    auth_post(
        app,
        "/application-approvers",
        admin_token,
        &serde_json::json!({"user_id": user_id}),
    )
    .await;

    // 4. Take to work (activates cars)
    auth_post(
        app,
        &format!("/applications/{}/take-to-work", app_id),
        admin_token,
        &serde_json::json!({
            "user_id": user_id,
            "action": "accept",
            "comment": "Accepted"
        }),
    )
    .await;

    // 5. Get car_id
    let atts_resp = auth_get(
        app,
        &format!("/applications/{}/attachments", app_id),
        admin_token,
    )
    .await;
    let atts: serde_json::Value = atts_resp.json().await.unwrap();
    let attachment_id = atts.as_array().unwrap()[0]["id"].as_i64().unwrap();

    let cars_resp = auth_get(
        app,
        &format!("/attachments/{}/cars", attachment_id),
        admin_token,
    )
    .await;
    let cars: serde_json::Value = cars_resp.json().await.unwrap();
    let car_id = cars.as_array().unwrap()[0]["id"].as_i64().unwrap();

    (app_id, car_id)
}

fn uuid() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let n = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .subsec_nanos();
    format!("{}", n)
}

fn rand_num() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let n = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .subsec_micros()
        % 999;
    format!("{:03}", n)
}

#[tokio::test]
async fn get_active_cars_for_tables() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_active").await;
    let _ = create_active_app_with_car(&app, &token).await;
    let response = auth_get(&app, "/cars/active-for-tables", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_fact_cars_for_tables() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_fact").await;
    let response = auth_get(&app, "/cars/fact-for-tables", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_car_unload_places() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_unload").await;
    let response = auth_get(&app, "/cars/unload-places", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_fact_car_unload_places() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_fact_unload").await;
    let response = auth_get(&app, "/cars/fact-unload-places", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn check_active_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_check").await;
    let response = auth_get(
        &app,
        "/cars/check-active?car_number=NONEXIST&car_brand=None",
        &token,
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("active").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn get_car_history() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_hist").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    let response = auth_get(&app, &format!("/cars/{}/history", car_id), &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn add_car_history_entry() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_add_hist").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    let response = auth_post(
        &app,
        &format!("/cars/{}/history", car_id),
        &token,
        &serde_json::json!({
            "action_type": "manual_note",
            "comment": "Test history entry"
        }),
    )
    .await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn get_all_cars_history() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_all_hist").await;
    let response = auth_get(&app, "/cars/history/all", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn get_cars_current_status() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_status").await;
    let response = auth_get(&app, "/cars/history/current-status", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn update_car_territory_status() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_territory").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    let response = auth_put(
        &app,
        &format!("/cars/{}/territory-status", car_id),
        &token,
        &serde_json::json!({
            "territory_status": 1
        }),
    )
    .await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn deactivate_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_deact").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    let response = auth_put(
        &app,
        &format!("/cars/{}/deactivate", car_id),
        &token,
        &serde_json::json!({
            "status": 0
        }),
    )
    .await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn activate_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_act").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    // Deactivate first, then activate
    auth_put(
        &app,
        &format!("/cars/{}/deactivate", car_id),
        &token,
        &serde_json::json!({"status": 0}),
    )
    .await;
    let response = auth_put(
        &app,
        &format!("/cars/{}/activate", car_id),
        &token,
        &serde_json::json!({}),
    )
    .await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn restore_car() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_restore").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    // Deactivate first, then restore
    auth_put(
        &app,
        &format!("/cars/{}/deactivate", car_id),
        &token,
        &serde_json::json!({"status": 0}),
    )
    .await;
    let response = auth_put(
        &app,
        &format!("/cars/{}/restore", car_id),
        &token,
        &serde_json::json!({}),
    )
    .await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn get_unified_car_history() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_unified").await;
    let response = auth_get(
        &app,
        "/cars/history/unified?car_number=NONEXIST&car_brand=None",
        &token,
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn active_cars_response_structure() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_struct").await;
    let _ = create_active_app_with_car(&app, &token).await;
    let response = auth_get(&app, "/cars/active-for-tables", &token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    let cars = body.as_array().unwrap();
    if !cars.is_empty() {
        let car = &cars[0];
        assert!(car["id"].is_number(), "should have id");
        assert!(car["car_number"].is_string(), "should have car_number");
        assert!(car["car_brand"].is_string(), "should have car_brand");
        assert!(car.get("status").is_some(), "should have status");
        assert!(car.get("territory_status").is_some(), "should have territory_status");
    }
    app.cleanup().await;
}

#[tokio::test]
async fn car_history_response_structure() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_hist_struct").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    let response = auth_get(&app, &format!("/cars/{}/history", car_id), &token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    let history = body.as_array().unwrap();
    if !history.is_empty() {
        let entry = &history[0];
        assert!(entry.get("id").is_some(), "should have id");
        assert!(entry.get("action_type").is_some(), "should have action_type");
        assert!(entry.get("created_at").is_some(), "should have created_at");
    }
    app.cleanup().await;
}

#[tokio::test]
async fn territory_status_entry_and_exit() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_entry_exit").await;
    let (_, car_id) = create_active_app_with_car(&app, &token).await;
    // Entry
    let entry = auth_put(&app, &format!("/cars/{}/territory-status", car_id), &token, &serde_json::json!({"territory_status": 1})).await;
    assert!(entry.status().is_success());
    let entry_body: serde_json::Value = entry.json().await.unwrap();
    assert_eq!(entry_body["territory_status"].as_i64().unwrap(), 1);
    // Exit
    let exit = auth_put(&app, &format!("/cars/{}/territory-status", car_id), &token, &serde_json::json!({"territory_status": 2})).await;
    assert!(exit.status().is_success());
    let exit_body: serde_json::Value = exit.json().await.unwrap();
    assert_eq!(exit_body["territory_status"].as_i64().unwrap(), 2);
    app.cleanup().await;
}

#[tokio::test]
async fn car_not_found() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "cars_notfound").await;
    let response = auth_get(&app, "/cars/99999/history", &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.as_array().unwrap().is_empty(), "should be empty array for nonexistent car");
    app.cleanup().await;
}
