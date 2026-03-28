mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn get_system_tables() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_list").await;

    let response = auth_get(&app, "/system-tables", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn create_system_table() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_create").await;

    let response = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({
            "name": "test_table",
            "display_name": "Тестовая таблица",
            "table_type": "cars"
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body["id"].as_i64().is_some());

    app.cleanup().await;
}

#[tokio::test]
async fn get_system_table_by_id() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_getid").await;

    let create_resp = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({
            "name": "table_by_id",
            "display_name": "По ID",
            "table_type": "cars"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_get(&app, &format!("/system-tables/{}", id), &token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn update_system_table() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_upd").await;

    let create_resp = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({
            "name": "original_table",
            "display_name": "Оригинальная",
            "table_type": "cars"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_put(
        &app,
        &format!("/system-tables/{}", id),
        &token,
        &serde_json::json!({
            "name": "updated_table",
            "display_name": "Обновлённая"
        }),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_system_table() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_del").await;

    let create_resp = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({
            "name": "to_delete_table",
            "display_name": "Удалить",
            "table_type": "cars"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_delete(&app, &format!("/system-tables/{}", id), &token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn manage_table_time_slots() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_slots").await;

    let create_resp = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({
            "name": "table_with_slots",
            "display_name": "Со слотами",
            "table_type": "cars"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let slot_resp = auth_post(
        &app,
        &format!("/system-tables/{}/time-slots", id),
        &token,
        &serde_json::json!({
            "day_of_week": 0,
            "open_time": "09:00",
            "close_time": "17:00"
        }),
    )
    .await;
    assert_eq!(slot_resp.status().as_u16(), 200);

    let slots_resp = auth_get(&app, &format!("/system-tables/{}/time-slots", id), &token).await;
    assert_eq!(slots_resp.status().as_u16(), 200);

    let slots_body: serde_json::Value = slots_resp.json().await.unwrap();
    assert!(slots_body.is_array());
    assert_eq!(slots_body.as_array().unwrap().len(), 1);

    app.cleanup().await;
}

#[tokio::test]
async fn get_system_table_by_name() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_name").await;

    // Create a table first
    let create_resp = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({
            "name": "lookup_table",
            "display_name": "Lookup",
            "table_type": "cars"
        }),
    )
    .await;
    assert_eq!(create_resp.status().as_u16(), 200);

    // Get by name
    let response = auth_get(&app, "/system-tables/name/lookup_table", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn update_table_time_slot() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_upd_slot").await;

    // Create table
    let cr = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({"name": "slot_upd_tbl", "display_name": "SlotUpd", "table_type": "cars"}),
    )
    .await;
    let id = cr.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();

    // Create time slot
    let sr = auth_post(
        &app,
        &format!("/system-tables/{}/time-slots", id),
        &token,
        &serde_json::json!({"day_of_week": 1, "open_time": "08:00", "close_time": "17:00"}),
    )
    .await;
    assert_eq!(sr.status().as_u16(), 200);
    let slot_id = sr.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();

    // Update time slot
    let response = auth_put(
        &app,
        &format!("/system-tables/{}/time-slots/{}", id, slot_id),
        &token,
        &serde_json::json!({"day_of_week": 2, "open_time": "09:00", "close_time": "18:00"}),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_table_time_slot() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_del_slot").await;

    // Create table
    let cr = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({"name": "slot_del_tbl", "display_name": "SlotDel", "table_type": "cars"}),
    )
    .await;
    let id = cr.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();

    // Create time slot
    let sr = auth_post(
        &app,
        &format!("/system-tables/{}/time-slots", id),
        &token,
        &serde_json::json!({"day_of_week": 3, "open_time": "10:00", "close_time": "19:00"}),
    )
    .await;
    let slot_id = sr.json::<serde_json::Value>().await.unwrap()["id"].as_i64().unwrap();

    // Delete time slot
    let response = auth_delete(
        &app,
        &format!("/system-tables/{}/time-slots/{}", id, slot_id),
        &token,
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn upload_system_table_photo() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_photo_up").await;

    // Create table first
    let cr = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({"name": "photo_tbl", "display_name": "Photo Table", "table_type": "cars"}),
    )
    .await;
    let table_id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    // Upload photo
    let form = reqwest::multipart::Form::new().part(
        "file",
        reqwest::multipart::Part::bytes(b"fake table photo".to_vec())
            .file_name("table_photo.jpg")
            .mime_str("image/jpeg")
            .unwrap(),
    );

    let response = app
        .api_client
        .post(format!(
            "{}/system-tables/{}/photos",
            app.address, table_id
        ))
        .header("Authorization", format!("Bearer {}", token))
        .multipart(form)
        .send()
        .await
        .unwrap();

    assert!(
        response.status().is_success(),
        "Upload should succeed, got {}",
        response.status()
    );
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(
        body.get("photo_ids").is_some(),
        "Response should contain photo_ids"
    );

    app.cleanup().await;
}

#[tokio::test]
async fn delete_system_table_photo() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_photo_del").await;

    // Create table + upload
    let cr = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({"name": "photo_del_tbl", "display_name": "Del Photo", "table_type": "cars"}),
    )
    .await;
    let table_id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    let form = reqwest::multipart::Form::new().part(
        "file",
        reqwest::multipart::Part::bytes(b"delete me table photo".to_vec())
            .file_name("del_table.jpg")
            .mime_str("image/jpeg")
            .unwrap(),
    );

    let upload_resp = app
        .api_client
        .post(format!(
            "{}/system-tables/{}/photos",
            app.address, table_id
        ))
        .header("Authorization", format!("Bearer {}", token))
        .multipart(form)
        .send()
        .await
        .unwrap();

    let upload_body: serde_json::Value = upload_resp.json().await.unwrap();
    let photo_id = upload_body["photo_ids"].as_array().unwrap()[0]
        .as_i64()
        .unwrap();

    // Delete
    let response = auth_delete(
        &app,
        &format!("/system-tables/{}/photos/{}", table_id, photo_id),
        &token,
    )
    .await;
    assert!(
        response.status().is_success(),
        "Delete should succeed, got {}",
        response.status()
    );

    app.cleanup().await;
}

#[tokio::test]
async fn set_main_system_table_photo() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "systbl_photo_main").await;

    // Create table + upload
    let cr = auth_post(
        &app,
        "/system-tables",
        &token,
        &serde_json::json!({"name": "photo_main_tbl", "display_name": "Main Photo", "table_type": "cars"}),
    )
    .await;
    let table_id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    let form = reqwest::multipart::Form::new().part(
        "file",
        reqwest::multipart::Part::bytes(b"main table photo".to_vec())
            .file_name("main_table.jpg")
            .mime_str("image/jpeg")
            .unwrap(),
    );

    let upload_resp = app
        .api_client
        .post(format!(
            "{}/system-tables/{}/photos",
            app.address, table_id
        ))
        .header("Authorization", format!("Bearer {}", token))
        .multipart(form)
        .send()
        .await
        .unwrap();

    let upload_body: serde_json::Value = upload_resp.json().await.unwrap();
    let photo_id = upload_body["photo_ids"].as_array().unwrap()[0]
        .as_i64()
        .unwrap();

    // Set as main
    let response = auth_post(
        &app,
        &format!("/system-tables/{}/photos/{}/main", table_id, photo_id),
        &token,
        &serde_json::json!({}),
    )
    .await;
    assert!(
        response.status().is_success(),
        "Set main should succeed, got {}",
        response.status()
    );

    app.cleanup().await;
}
