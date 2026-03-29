use serde::{Serialize, Deserialize};
use chrono::NaiveDateTime;

#[derive(Debug, Serialize, Deserialize)]
pub struct NewsItem {
    pub id: i32,
    pub title: String,
    pub description: String,
    pub full_text: String,
    pub created_by: i32,
    pub created_by_name: Option<String>,
    pub created_at: NaiveDateTime,
    pub updated_at: Option<NaiveDateTime>,
    pub updated_by: Option<i32>,
    pub updated_by_name: Option<String>,
    pub is_active: bool,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateNewsRequest {
    pub title: String,
    pub description: String,
    pub full_text: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateNewsRequest {
    pub title: Option<String>,
    pub description: Option<String>,
    pub full_text: Option<String>,
    #[serde(rename = "is_active")]
    pub is_active: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AnnouncementItem {
    pub id: i32,
    pub title: String,
    pub description: String,
    pub full_text: String,
    pub is_important: bool,
    pub is_active: bool,
    pub created_by: i32,
    pub created_by_name: Option<String>,
    pub created_at: NaiveDateTime,
    pub updated_at: Option<NaiveDateTime>,
    pub updated_by: Option<i32>,
    pub updated_by_name: Option<String>,
    pub activated_at: Option<NaiveDateTime>,
    pub activated_by: Option<i32>,
    pub activated_by_name: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateAnnouncementRequest {
    pub title: String,
    pub description: String,
    pub full_text: Option<String>,
    pub is_important: bool,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateAnnouncementRequest {
    pub title: Option<String>,
    pub description: Option<String>,
    pub full_text: Option<String>,
    pub is_important: Option<bool>,
    #[serde(rename = "is_active")]  // явно указываем snake_case
    pub is_active: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SetActiveAnnouncementRequest {
    pub announcement_id: i32,
}