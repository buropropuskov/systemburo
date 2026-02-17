use serde::{Serialize, Deserialize};
use chrono::{DateTime, Utc};
use sqlx::FromRow;

#[derive(Debug, Serialize, Deserialize, FromRow)]
pub struct ApplicationHistory {
    pub id: i32,
    pub application_id: i32,
    pub user_id: i32,
    pub action_type: String,
    pub action_status: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub created_at: DateTime<Utc>,
    pub metadata: Option<serde_json::Value>,
}

// Эту структуру можно удалить, если не используется
// pub struct ApplicationHistoryWithUser { ... }

#[derive(Debug, Serialize, Deserialize)]
pub struct AddHistoryRequest {
    pub application_id: i32,
    pub user_id: i32,
    pub action_type: String,
    pub action_status: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationHistoryItem {
    pub id: i32,
    pub application_id: i32,
    pub user_id: i32,
    pub user_name: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub action_type: String,
    pub action_status: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub created_at: DateTime<Utc>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct RevokeApprovalRequest {
    pub comment: Option<String>,
}