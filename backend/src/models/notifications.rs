use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct Notification {
    pub id: i32,
    pub user_id: i32,
    #[serde(rename = "type")]
    pub type_: String,
    pub title: String,
    pub message: String,
    pub data: Option<serde_json::Value>,
    pub is_read: bool,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Deserialize)]
pub struct MarkReadRequest {
    pub is_read: bool,
}

#[derive(Debug, Deserialize)]
pub struct CreateNotificationRequest {
    pub user_id: i32,
    #[serde(rename = "type")]
    pub type_: String,
    pub title: String,
    pub message: String,
    pub data: Option<serde_json::Value>,
}