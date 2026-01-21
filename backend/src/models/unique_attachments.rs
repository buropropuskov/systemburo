// models/attachments.rs
use serde::{Serialize, Deserialize};
use chrono::{NaiveDateTime, Utc};

#[derive(Debug, Serialize, Deserialize)]
pub struct UniqueAttachment {
    pub id: i32,
    pub attachment_type: String,
    pub name: String,
    pub display_name: String,
    pub title: String,
    pub instruction: Option<String>,
    pub is_active: Option<bool>,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateAttachmentRequest {
    pub name: String,
    pub display_name: String,
    pub title: String,
    pub attachment_type: String,
    pub instruction: Option<String>,
    pub is_active: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateAttachmentRequest {
    pub name: String,
    pub display_name: String,
    pub title: String,
    pub attachment_type: String,
    pub instruction: Option<String>,
}