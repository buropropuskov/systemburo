mod common;

use common::auth_helper::*;
use common::setup::TestApp;

#[tokio::test]
async fn get_unload_places() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_list").await;

    let response = auth_get(&app, "/unload-places", &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array());

    app.cleanup().await;
}

#[tokio::test]
async fn get_unload_places_no_auth() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(&format!("{}/unload-places", app.address))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 401);

    app.cleanup().await;
}

#[tokio::test]
async fn create_unload_place() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_create").await;

    let response = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({
            "name": "Test Place",
            "description": "Test description"
        }),
    )
    .await;

    assert_eq!(response.status().as_u16(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body["id"].as_i64().is_some());

    app.cleanup().await;
}

#[tokio::test]
async fn get_unload_place_by_id() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_getid").await;

    let create_resp = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({
            "name": "Place By Id",
            "description": "Desc"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_get(&app, &format!("/unload-places/{}", id), &token).await;
    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert_eq!(body["name"].as_str().unwrap(), "Place By Id");

    app.cleanup().await;
}

#[tokio::test]
async fn update_unload_place() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_upd").await;

    let create_resp = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({
            "name": "Original Place",
            "description": "Original"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_put(
        &app,
        &format!("/unload-places/{}", id),
        &token,
        &serde_json::json!({
            "name": "Updated Place"
        }),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_unload_place() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_del").await;

    let create_resp = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({
            "name": "To Delete",
            "description": "Will be deleted"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let response = auth_delete(&app, &format!("/unload-places/{}", id), &token).await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn manage_time_slots() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_slots").await;

    let create_resp = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({
            "name": "Place With Slots",
            "description": "Has time slots"
        }),
    )
    .await;
    let body: serde_json::Value = create_resp.json().await.unwrap();
    let id = body["id"].as_i64().unwrap();

    let slot_resp = auth_post(
        &app,
        &format!("/unload-places/{}/time-slots", id),
        &token,
        &serde_json::json!({
            "day_of_week": 1,
            "open_time": "08:00",
            "close_time": "18:00"
        }),
    )
    .await;
    assert_eq!(slot_resp.status().as_u16(), 200);

    let slots_resp = auth_get(&app, &format!("/unload-places/{}/time-slots", id), &token).await;
    assert_eq!(slots_resp.status().as_u16(), 200);

    let slots_body: serde_json::Value = slots_resp.json().await.unwrap();
    assert!(slots_body.is_array());
    assert_eq!(slots_body.as_array().unwrap().len(), 1);

    app.cleanup().await;
}

#[tokio::test]
async fn update_time_slot() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_upd_slot").await;

    // Create place
    let cr = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({"name": "SlotUpdPlace", "description": "test"}),
    )
    .await;
    let id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    // Create time slot
    let sr = auth_post(
        &app,
        &format!("/unload-places/{}/time-slots", id),
        &token,
        &serde_json::json!({"day_of_week": 2, "open_time": "09:00", "close_time": "18:00"}),
    )
    .await;
    assert_eq!(sr.status().as_u16(), 200);
    let slot_id = sr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    // Update
    let response = auth_put(
        &app,
        &format!("/unload-places/{}/time-slots/{}", id, slot_id),
        &token,
        &serde_json::json!({"day_of_week": 3, "open_time": "10:00", "close_time": "19:00"}),
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_time_slot() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_del_slot").await;

    // Create place
    let cr = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({"name": "SlotDelPlace", "description": "test"}),
    )
    .await;
    let id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    // Create time slot
    let sr = auth_post(
        &app,
        &format!("/unload-places/{}/time-slots", id),
        &token,
        &serde_json::json!({"day_of_week": 4, "open_time": "07:00", "close_time": "16:00"}),
    )
    .await;
    let slot_id = sr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    // Delete
    let response = auth_delete(
        &app,
        &format!("/unload-places/{}/time-slots/{}", id, slot_id),
        &token,
    )
    .await;
    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn upload_unload_place_photo() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_photo_up").await;

    // Create a place first
    let cr = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({"name": "PhotoPlace", "description": "test"}),
    )
    .await;
    let place_id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    // Upload photo via multipart
    let form = reqwest::multipart::Form::new().part(
        "file",
        reqwest::multipart::Part::bytes(b"fake image data for testing".to_vec())
            .file_name("test_photo.jpg")
            .mime_str("image/jpeg")
            .unwrap(),
    );

    let response = app
        .api_client
        .post(format!(
            "{}/unload-places/{}/photos",
            app.address, place_id
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
async fn delete_unload_place_photo() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_photo_del").await;

    // Create place + upload photo
    let cr = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({"name": "PhotoDelPlace", "description": "test"}),
    )
    .await;
    let place_id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    let form = reqwest::multipart::Form::new().part(
        "file",
        reqwest::multipart::Part::bytes(b"delete me".to_vec())
            .file_name("delete_test.jpg")
            .mime_str("image/jpeg")
            .unwrap(),
    );

    let upload_resp = app
        .api_client
        .post(format!(
            "{}/unload-places/{}/photos",
            app.address, place_id
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
        &format!("/unload-places/{}/photos/{}", place_id, photo_id),
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
async fn set_main_unload_place_photo() {
    let app = TestApp::spawn().await;
    let (token, _, _) = create_admin_user(&app, "unload_photo_main").await;

    // Create place + upload photo
    let cr = auth_post(
        &app,
        "/unload-places",
        &token,
        &serde_json::json!({"name": "PhotoMainPlace", "description": "test"}),
    )
    .await;
    let place_id = cr.json::<serde_json::Value>().await.unwrap()["id"]
        .as_i64()
        .unwrap();

    let form = reqwest::multipart::Form::new().part(
        "file",
        reqwest::multipart::Part::bytes(b"main photo".to_vec())
            .file_name("main_test.jpg")
            .mime_str("image/jpeg")
            .unwrap(),
    );

    let upload_resp = app
        .api_client
        .post(format!(
            "{}/unload-places/{}/photos",
            app.address, place_id
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

    // Set as main (POST, not PUT)
    let response = auth_post(
        &app,
        &format!("/unload-places/{}/photos/{}/main", place_id, photo_id),
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
