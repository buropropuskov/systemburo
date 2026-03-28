mod common;

use common::auth_helper::*;
use common::setup::TestApp;

fn sample_attachment_payload() -> serde_json::Value {
    serde_json::json!({
        "attachment_type": "cars",
        "name": "test_blank",
        "display_name": "Тестовый бланк",
        "title": "Бланк",
        "instruction": "Инструкция"
    })
}

#[tokio::test]
async fn get_attachments() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!("{}/attachments", &app.address))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn get_all_attachments() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!("{}/attachments/all", &app.address))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn create_attachment() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .post(format!("{}/attachments", &app.address))
        .json(&sample_attachment_payload())
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("id").is_some(), "Response should contain id");

    app.cleanup().await;
}

#[tokio::test]
async fn get_attachment_by_id() {
    let app = TestApp::spawn().await;

    // Create first
    let create_resp = app
        .api_client
        .post(format!("{}/attachments", &app.address))
        .json(&sample_attachment_payload())
        .send()
        .await
        .unwrap();
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Get by id
    let response = app
        .api_client
        .get(format!("{}/attachments/{}", &app.address, id))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn update_attachment() {
    let app = TestApp::spawn().await;

    // Create first
    let create_resp = app
        .api_client
        .post(format!("{}/attachments", &app.address))
        .json(&sample_attachment_payload())
        .send()
        .await
        .unwrap();
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Update
    let response = app
        .api_client
        .put(format!("{}/attachments/{}", &app.address, id))
        .json(&serde_json::json!({
            "attachment_type": "cars",
            "name": "test_blank_updated",
            "display_name": "Обновлённый бланк",
            "title": "Бланк обновлён",
            "instruction": "Новая инструкция"
        }))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_attachment() {
    let app = TestApp::spawn().await;

    // Create first
    let create_resp = app
        .api_client
        .post(format!("{}/attachments", &app.address))
        .json(&sample_attachment_payload())
        .send()
        .await
        .unwrap();
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Soft delete
    let response = app
        .api_client
        .delete(format!("{}/attachments/{}", &app.address, id))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn restore_attachment() {
    let app = TestApp::spawn().await;

    // Create
    let create_resp = app
        .api_client
        .post(format!("{}/attachments", &app.address))
        .json(&sample_attachment_payload())
        .send()
        .await
        .unwrap();
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Delete (soft)
    let delete_resp = app
        .api_client
        .delete(format!("{}/attachments/{}", &app.address, id))
        .send()
        .await
        .unwrap();
    assert_eq!(delete_resp.status().as_u16(), 200);

    // Restore (uses PUT, not POST)
    let response = app
        .api_client
        .put(format!("{}/attachments/{}/restore", &app.address, id))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn get_attachment_cars() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "att_cars").await;

    // Use a non-existent application attachment id
    let response = auth_get(&app, "/attachments/99999/cars", &token).await;
    let status = response.status().as_u16();
    assert!(
        status == 200 || status == 404,
        "Expected 200 or 404, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn get_attachment_employees() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "att_emp").await;

    let response = auth_get(&app, "/attachments/99999/employees", &token).await;
    let status = response.status().as_u16();
    assert!(
        status == 200 || status == 404,
        "Expected 200 or 404, got {}",
        status
    );

    app.cleanup().await;
}

#[tokio::test]
async fn get_attachment_items() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "att_items").await;

    let response = auth_get(&app, "/attachments/99999/items", &token).await;
    let status = response.status().as_u16();
    assert!(
        status == 200 || status == 404,
        "Expected 200 or 404, got {}",
        status
    );

    app.cleanup().await;
}
