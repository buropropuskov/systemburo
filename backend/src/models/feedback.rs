// models/feedback.rs
use serde::{Serialize, Deserialize};
use chrono::{DateTime, Utc};

#[derive(Debug, Serialize, Deserialize)]
pub struct Feedback {
    pub id: i32,
    pub user_id: i32,
    pub message: String,
    pub status: String,
    pub is_read: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct FeedbackWithUser {
    pub id: i32,
    pub user_id: i32,
    pub user_name: String,
    pub message: String,
    pub status: String,
    pub is_read: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateFeedbackRequest {
    pub message: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateFeedbackStatusRequest {
    pub status: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct MarkAsReadRequest {
    pub is_read: bool,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct FeedbackStats {
    pub total: i64,
    pub resolved: i64,
    pub unresolved: i64,
    pub unread: i64,
}

#[derive(Debug, Serialize)]
pub struct MyFeedback {
    pub id: i32,
    pub user_id: i32,
    pub message: String,
    pub status: String,
    pub is_read: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}