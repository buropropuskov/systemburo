mod common;

use common::auth_helper::*;
use common::setup::TestApp;

/// Create a unique_attachment and return its id.
async fn create_test_attachment(app: &TestApp) -> i64 {
    let response = app
        .api_client
        .post(format!("{}/attachments", &app.address))
        .json(&serde_json::json!({
            "attachment_type": "cars",
            "name": "test_blank",
            "display_name": "Тестовый бланк",
            "title": "Бланк ТС",
            "instruction": "Инструкция"
        }))
        .send()
        .await
        .expect("Failed to create attachment");

    assert_eq!(response.status().as_u16(), 200, "Attachment creation failed");

    let body: serde_json::Value = response.json().await.expect("Failed to parse attachment response");
    body["id"].as_i64().expect("Attachment response should contain id")
}

/// Build the submit-complete-application payload referencing the given attachment id.
fn submit_application_payload(attachment_id: i64) -> serde_json::Value {
    serde_json::json!({
        "message": "Test application",
        "organization": "Тестовая организация",
        "company": "Тестовая компания",
        "responsible_person": "Admin User",
        "contact_phone": "79001234567",
        "data_approval": true,
        "attachments": [
            {
                "attachment_type": "cars",
                "attachment_name": "test_blank",
                "attachment_display_name": "Тестовый бланк",
                "unique_attachment_id": attachment_id,
                "entry_date_from": "2026-04-01",
                "entry_date_to": "2026-04-30",
                "entry_time_from": "08:00:00",
                "entry_time_to": "20:00:00",
                "data": {
                    "vehicles": [
                        {
                            "car_number": "A001AA77",
                            "car_brand": "Toyota",
                            "unload_places": []
                        }
                    ]
                }
            }
        ]
    })
}

/// Helper: create attachment + submit application, return (application_id, application_number).
async fn submit_test_application(app: &TestApp, token: &str) -> (i64, String) {
    let attachment_id = create_test_attachment(app).await;
    let payload = submit_application_payload(attachment_id);

    let response = auth_post(app, "/applications/submit-complete-application", token, &payload).await;
    assert_eq!(
        response.status().as_u16(),
        200,
        "Submit application should return 200"
    );

    let body: serde_json::Value = response.json().await.expect("Failed to parse submit response");
    let application_id = body["application_id"].as_i64().expect("Response should contain application_id");
    let application_number = body["application_number"]
        .as_str()
        .expect("Response should contain application_number")
        .to_string();

    (application_id, application_number)
}

#[tokio::test]
async fn submit_complete_application() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "submit_app").await;

    let attachment_id = create_test_attachment(&app).await;
    let payload = submit_application_payload(attachment_id);

    let response = auth_post(&app, "/applications/submit-complete-application", &token, &payload).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.get("application_id").is_some(), "Response should contain application_id");
    assert!(body.get("application_number").is_some(), "Response should contain application_number");

    app.cleanup().await;
}

#[tokio::test]
async fn submit_application_no_auth() {
    let app = TestApp::spawn().await;

    let attachment_id = create_test_attachment(&app).await;
    let payload = submit_application_payload(attachment_id);

    let response = app
        .api_client
        .post(format!("{}/applications/submit-complete-application", &app.address))
        .json(&payload)
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(response.status().as_u16(), 401);

    app.cleanup().await;
}

#[tokio::test]
async fn get_applications_list() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "list_apps").await;

    // Submit an application first so the list is not empty
    let _ = submit_test_application(&app, &token).await;

    let response = auth_get(&app, "/applications", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_user_applications() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "user_apps").await;

    let (application_id, _) = submit_test_application(&app, &token).await;

    let response = auth_get(&app, "/applications/user", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    let apps = body.as_array().unwrap();
    assert!(
        !apps.is_empty(),
        "User should have at least one application"
    );

    // Verify the created application is in the list
    let found = apps.iter().any(|a| a["id"].as_i64() == Some(application_id));
    assert!(found, "Created application should appear in user applications list");

    app.cleanup().await;
}

#[tokio::test]
async fn get_application_by_id() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "app_by_id").await;

    let (application_id, application_number) = submit_test_application(&app, &token).await;

    let path = format!("/applications/{}", application_id);
    let response = auth_get(&app, &path, &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert_eq!(
        body["application_number"].as_str().unwrap(),
        application_number,
        "Application number should match"
    );

    app.cleanup().await;
}

#[tokio::test]
async fn get_application_details() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "app_details").await;

    let (application_id, _) = submit_test_application(&app, &token).await;

    let path = format!("/applications/{}/details", application_id);
    let response = auth_get(&app, &path, &token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn get_application_history() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "app_history").await;

    let (application_id, _) = submit_test_application(&app, &token).await;

    let path = format!("/applications/{}/history", application_id);
    let response = auth_get(&app, &path, &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "History response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_application_attachments() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "app_attachments").await;

    let (application_id, _) = submit_test_application(&app, &token).await;

    let path = format!("/applications/{}/attachments", application_id);
    let response = auth_get(&app, &path, &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Attachments response should be an array");
    assert!(
        !body.as_array().unwrap().is_empty(),
        "Application should have at least one attachment"
    );

    app.cleanup().await;
}

#[tokio::test]
async fn create_application_simple() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "app_simple").await;
    let response = auth_post(&app, "/applications", &token, &serde_json::json!({
        "organization_id": 1,
        "company_id": 1,
        "data_approval": true,
        "message": "Simple test application"
    })).await;
    assert!(response.status().is_success());
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("application_id").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn update_application() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_update").await;
    let (app_id, _) = submit_test_application(&app, &token).await;
    let response = auth_put(&app, &format!("/applications/{}", app_id), &token, &serde_json::json!({
        "status": "В обработке"
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn get_responsible_users() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_resp").await;
    let (app_id, _) = submit_test_application(&app, &token).await;
    let response = auth_get(&app, &format!("/applications/{}/responsible-users", app_id), &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn check_approval_status() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_approval").await;
    let (app_id, _) = submit_test_application(&app, &token).await;
    let response = auth_get(&app, &format!("/applications/{}/check-approval-status", app_id), &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("confirmation").is_some());
    assert!(body.get("status").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn get_application_viewers() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_viewers").await;
    let (app_id, _) = submit_test_application(&app, &token).await;
    let response = auth_get(&app, &format!("/applications/{}/viewers", app_id), &token).await;
    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());
    app.cleanup().await;
}

#[tokio::test]
async fn add_history_entry() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_hist").await;
    let (app_id, _) = submit_test_application(&app, &token).await;
    // Get user_id
    let me = auth_get(&app, "/users/me", &token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();
    let response = auth_post(&app, "/applications/history", &token, &serde_json::json!({
        "application_id": app_id,
        "user_id": user_id,
        "action_type": "manual_entry",
        "comment": "Test history entry"
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn forward_application() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "app_fwd").await;
    let (user_token, _, _) = create_authenticated_user(&app, "app_fwd_target").await;
    let (app_id, _) = submit_test_application(&app, &admin_token).await;
    // Get target user_id
    let me = auth_get(&app, "/users/me", &user_token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();
    let response = auth_post(&app, &format!("/applications/{}/forward", app_id), &admin_token, &serde_json::json!({
        "users": [{"user_id": user_id, "required_approval": true, "can_view": false}]
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn approve_application() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "app_approve").await;
    let (user_token, _, _) = create_authenticated_user(&app, "app_approve_user").await;
    let (app_id, _) = submit_test_application(&app, &admin_token).await;
    // Get user_id for the responsible user
    let me = auth_get(&app, "/users/me", &user_token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();
    // Forward to make user responsible
    auth_post(&app, &format!("/applications/{}/forward", app_id), &admin_token, &serde_json::json!({
        "users": [{"user_id": user_id, "required_approval": true, "can_view": false}]
    })).await;
    // Approve
    let response = auth_post(&app, &format!("/applications/{}/approve", app_id), &user_token, &serde_json::json!({
        "user_id": user_id,
        "status": "approved",
        "comment": "Looks good"
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn take_application_to_work() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "app_take").await;
    let (app_id, _) = submit_test_application(&app, &admin_token).await;
    // Get admin user_id
    let me = auth_get(&app, "/users/me", &admin_token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();
    // Make admin an approver
    auth_post(&app, "/application-approvers", &admin_token, &serde_json::json!({"user_id": user_id})).await;
    // Take to work
    let response = auth_post(&app, &format!("/applications/{}/take-to-work", app_id), &admin_token, &serde_json::json!({
        "user_id": user_id,
        "action": "accept",
        "comment": "Taking to work"
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn revoke_from_work() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "app_revoke").await;
    let (app_id, _) = submit_test_application(&app, &admin_token).await;
    let me = auth_get(&app, "/users/me", &admin_token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();
    // Make approver + take to work first
    auth_post(&app, "/application-approvers", &admin_token, &serde_json::json!({"user_id": user_id})).await;
    auth_post(&app, &format!("/applications/{}/take-to-work", app_id), &admin_token, &serde_json::json!({"user_id": user_id, "action": "accept"})).await;
    // Revoke from work
    let response = auth_post(&app, &format!("/applications/{}/revoke-from-work", app_id), &admin_token, &serde_json::json!({
        "user_id": user_id,
        "comment": "Need revisions"
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn restore_to_work() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "app_restore").await;
    let (app_id, _) = submit_test_application(&app, &admin_token).await;
    let me = auth_get(&app, "/users/me", &admin_token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();
    // Make approver + take to work + revoke
    auth_post(&app, "/application-approvers", &admin_token, &serde_json::json!({"user_id": user_id})).await;
    auth_post(&app, &format!("/applications/{}/take-to-work", app_id), &admin_token, &serde_json::json!({"user_id": user_id, "action": "accept"})).await;
    auth_post(&app, &format!("/applications/{}/revoke-from-work", app_id), &admin_token, &serde_json::json!({"user_id": user_id})).await;
    // Restore to work
    let response = auth_post(&app, &format!("/applications/{}/restore-to-work", app_id), &admin_token, &serde_json::json!({
        "user_id": user_id,
        "comment": "Re-approved"
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn revoke_approval() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "app_rev_appr").await;
    let (user_token, _, _) = create_authenticated_user(&app, "app_rev_user").await;
    let (app_id, _) = submit_test_application(&app, &admin_token).await;
    // Get user_id
    let me = auth_get(&app, "/users/me", &user_token).await;
    let user_id = me.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();
    // Forward to make responsible, then approve, then revoke
    auth_post(&app, &format!("/applications/{}/forward", app_id), &admin_token, &serde_json::json!({
        "users": [{"user_id": user_id, "required_approval": true, "can_view": false}]
    })).await;
    auth_post(&app, &format!("/applications/{}/approve", app_id), &user_token, &serde_json::json!({
        "user_id": user_id, "status": "approved"
    })).await;
    // Revoke approval
    let response = auth_post(&app, &format!("/applications/{}/revoke-approval", app_id), &user_token, &serde_json::json!({
        "comment": "Changed my mind"
    })).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}

#[tokio::test]
async fn submit_application_response_structure() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_struct").await;
    let (app_id, app_number) = submit_test_application(&app, &token).await;
    assert!(app_id > 0, "application_id should be positive");
    assert!(!app_number.is_empty(), "application_number should not be empty");
    app.cleanup().await;
}

#[tokio::test]
async fn get_application_response_structure() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_get_struct").await;
    let (app_id, _) = submit_test_application(&app, &token).await;
    let response = auth_get(&app, &format!("/applications/{}", app_id), &token).await;
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("id").is_some(), "should have id");
    assert!(body.get("application_number").is_some(), "should have application_number");
    assert!(body.get("status").is_some(), "should have status");
    assert!(body.get("confirmation").is_some(), "should have confirmation");
    assert!(body.get("sending_datetime").is_some(), "should have sending_datetime");
    app.cleanup().await;
}

#[tokio::test]
async fn get_nonexistent_application() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "app_404").await;
    let response = auth_get(&app, "/applications/999999", &token).await;
    let status = response.status().as_u16();
    assert!(status == 404 || status == 500, "Expected 404 or error for nonexistent app, got {}", status);
    app.cleanup().await;
}

#[tokio::test]
async fn create_application_without_data_approval() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_authenticated_user(&app, "app_no_appr").await;
    let response = auth_post(&app, "/applications", &token, &serde_json::json!({
        "organization_id": 1,
        "company_id": 1,
        "data_approval": false,
        "message": "Should fail"
    })).await;
    assert_eq!(response.status().as_u16(), 400);
    app.cleanup().await;
}

#[tokio::test]
async fn update_items_status() {
    let app = TestApp::spawn().await;
    let (admin_token, _, _) = create_admin_user(&app, "app_items").await;
    let (app_id, _) = submit_test_application(&app, &admin_token).await;
    let response = auth_post(&app, &format!("/applications/{}/update-items-status", app_id), &admin_token, &serde_json::json!({})).await;
    assert!(response.status().is_success());
    app.cleanup().await;
}
