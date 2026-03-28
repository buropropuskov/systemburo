mod common;

use common::setup::TestApp;
use common::auth_helper::*;

#[tokio::test]
async fn register_success() {
    let app = TestApp::spawn().await;
    let response = register_user(&app, "newuser", "Password123!").await;
    assert_eq!(response.status(), 200);
    app.cleanup().await;
}
