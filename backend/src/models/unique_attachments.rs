// models/attachments.rs
use serde::{Serialize, Deserialize};
use chrono::{NaiveDateTime};

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

#[derive(Debug, Serialize, Deserialize)]
pub struct AttachmentWithUniqueInfo {
    pub id: i32,
    pub attachment_type: String,
    pub attachment_name: String,
    pub attachment_display_name: String,
    pub entry_date_from: Option<NaiveDateTime>,
    pub entry_date_to: Option<NaiveDateTime>,
    pub entry_time_from: Option<NaiveDateTime>,
    pub entry_time_to: Option<NaiveDateTime>,
    pub created_at: Option<NaiveDateTime>,
    pub unique_attachment_id: Option<i32>,
    pub unique_attachment_title: Option<String>,
    pub unique_attachment_display_name: Option<String>,
}