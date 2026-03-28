mod common;

use common::setup::TestApp;

fn sample_format_payload() -> serde_json::Value {
    serde_json::json!({
        "name": "Test Format",
        "country_code": "TST",
        "icon": "🏁",
        "is_default": false,
        "cells": [
            {
                "cell_order": 1,
                "cell_type": "letters",
                "min_length": 1,
                "max_length": 1,
                "allowed_letters": "АВЕКМНОРСТУХ",
                "alphabet_type": "cyrillic",
                "language": "ru"
            }
        ]
    })
}

#[tokio::test]
async fn get_license_plate_formats() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!("{}/license-plate-formats", &app.address))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn create_license_plate_format() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .post(format!("{}/license-plate-formats", &app.address))
        .json(&sample_format_payload())
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("id").is_some(), "Response should contain id");

    app.cleanup().await;
}

#[tokio::test]
async fn update_license_plate_format() {
    let app = TestApp::spawn().await;

    // Create first
    let create_resp = app
        .api_client
        .post(format!("{}/license-plate-formats", &app.address))
        .json(&sample_format_payload())
        .send()
        .await
        .unwrap();
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Update
    let updated_payload = serde_json::json!({
        "name": "Updated Format",
        "country_code": "TST",
        "icon": "🏁",
        "is_default": false,
        "cells": [
            {
                "cell_order": 1,
                "cell_type": "letters",
                "min_length": 1,
                "max_length": 2,
                "allowed_letters": "АВЕКМНОРСТУХ",
                "alphabet_type": "cyrillic",
                "language": "ru"
            }
        ]
    });

    let response = app
        .api_client
        .put(format!("{}/license-plate-formats/{}", &app.address, id))
        .json(&updated_payload)
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_license_plate_format() {
    let app = TestApp::spawn().await;

    // Create first
    let create_resp = app
        .api_client
        .post(format!("{}/license-plate-formats", &app.address))
        .json(&sample_format_payload())
        .send()
        .await
        .unwrap();
    assert_eq!(create_resp.status().as_u16(), 200);

    let created: serde_json::Value = create_resp.json().await.unwrap();
    let id = created["id"].as_i64().unwrap();

    // Delete
    let response = app
        .api_client
        .delete(format!("{}/license-plate-formats/{}", &app.address, id))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}
