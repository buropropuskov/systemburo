mod common;

use common::setup::TestApp;

#[tokio::test]
async fn get_citizenships() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .get(format!("{}/citizenships", &app.address))
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body.is_array(), "Response should be an array");

    app.cleanup().await;
}

#[tokio::test]
async fn create_citizenship() {
    let app = TestApp::spawn().await;

    let payload = serde_json::json!({
        "name": "TestCountry",
        "icon": "\u{1f3f3}\u{fe0f}",
        "is_default": false,
        "patent_required": false
    });

    let response = app
        .api_client
        .post(format!("{}/citizenships", &app.address))
        .json(&payload)
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(response.status().as_u16(), 200);

    let body: serde_json::Value = response.json().await.expect("Failed to parse response");
    assert!(body["id"].is_number(), "Response should contain id");

    app.cleanup().await;
}

#[tokio::test]
async fn update_citizenship() {
    let app = TestApp::spawn().await;

    // Create a citizenship first
    let payload = serde_json::json!({
        "name": "CountryToUpdate",
        "icon": "\u{1f3f3}\u{fe0f}",
        "is_default": false,
        "patent_required": false
    });

    let create_response = app
        .api_client
        .post(format!("{}/citizenships", &app.address))
        .json(&payload)
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(create_response.status().as_u16(), 200);
    let created: serde_json::Value = create_response.json().await.expect("Failed to parse");
    let citizenship_id = created["id"].as_i64().expect("Should have id");

    // Update the citizenship
    let update_payload = serde_json::json!({
        "name": "Updated",
        "icon": "\u{1f3f4}",
        "is_active": true,
        "is_default": false,
        "patent_required": true
    });

    let response = app
        .api_client
        .put(format!("{}/citizenships/{}", &app.address, citizenship_id))
        .json(&update_payload)
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn delete_citizenship() {
    let app = TestApp::spawn().await;

    // Create a citizenship first
    let payload = serde_json::json!({
        "name": "CountryToDelete",
        "icon": "\u{1f3f3}\u{fe0f}",
        "is_default": false,
        "patent_required": false
    });

    let create_response = app
        .api_client
        .post(format!("{}/citizenships", &app.address))
        .json(&payload)
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(create_response.status().as_u16(), 200);
    let created: serde_json::Value = create_response.json().await.expect("Failed to parse");
    let citizenship_id = created["id"].as_i64().expect("Should have id");

    // Delete the citizenship
    let response = app
        .api_client
        .delete(format!("{}/citizenships/{}", &app.address, citizenship_id))
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn clear_default_citizenships() {
    let app = TestApp::spawn().await;

    let response = app
        .api_client
        .post(format!("{}/citizenships/clear-default", &app.address))
        .send()
        .await
        .expect("Failed to send request");

    assert_eq!(response.status().as_u16(), 200);

    app.cleanup().await;
}

#[tokio::test]
async fn citizenship_response_structure() {
    let app = TestApp::spawn().await;
    let cr = app.api_client
        .post(format!("{}/citizenships", &app.address))
        .json(&serde_json::json!({"name": "StructCountry", "icon": "\u{1f3f3}\u{fe0f}", "patent_required": false}))
        .send().await.unwrap();
    let body: serde_json::Value = cr.json().await.unwrap();
    assert!(body["id"].is_number(), "should have id");
    // Get list and check structure
    let response = app.api_client
        .get(format!("{}/citizenships", &app.address))
        .send().await.unwrap();
    let list: serde_json::Value = response.json().await.unwrap();
    let items = list.as_array().unwrap();
    assert!(!items.is_empty());
    let cit = &items[0];
    assert!(cit["id"].is_number(), "should have id");
    assert!(cit["name"].is_string(), "should have name");
    assert!(cit.get("icon").is_some(), "should have icon");
    assert!(cit.get("is_active").is_some(), "should have is_active");
    assert!(cit.get("is_default").is_some(), "should have is_default");
    assert!(cit.get("patent_required").is_some(), "should have patent_required");
    app.cleanup().await;
}

#[tokio::test]
async fn delete_nonexistent_citizenship() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .delete(format!("{}/citizenships/99999", &app.address))
        .send().await.unwrap();
    let status = response.status().as_u16();
    assert!(status == 404 || status == 200, "Expected 404 or 200 for nonexistent, got {}", status);
    app.cleanup().await;
}
