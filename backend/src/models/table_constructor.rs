use serde::{Serialize, Deserialize};
use chrono::{NaiveTime, NaiveDateTime};

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct SystemTable {
    pub id: i32,
    pub name: String,
    pub display_name: String,
    pub table_type: String,
    pub show_fact_table: Option<bool>,
    pub fact_table_hint: Option<String>,
    pub instruction: Option<String>,
    pub map_link: Option<String>,
    pub status: Option<String>,
    pub status_comment: Option<String>,
    pub location_description: Option<String>,
    pub is_active: Option<bool>,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct SystemTableTimeSlot {
    pub id: i32,
    pub table_id: i32,
    pub day_of_week: i32,
    pub open_time: NaiveTime,
    pub close_time: NaiveTime,
    pub is_next_day: bool,
    pub is_active: bool,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct SystemTablePhoto {
    pub id: i32,
    pub table_id: i32,
    pub photo_url: String,
    pub file_name: String,
    pub file_size: Option<i32>,
    pub mime_type: Option<String>,
    pub is_main: bool,
    pub uploaded_at: Option<NaiveDateTime>,
    pub uploaded_by: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct TableField {
    pub id: i32,
    pub table_id: i32,
    pub field_name: String,
    pub field_type: String,
    pub display_order: i32,
    pub is_visible: Option<bool>,
    pub created_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SystemTableWithDetails {
    pub table: SystemTable,
    pub fields: Vec<TableField>,
    pub time_slots: Vec<SystemTableTimeSlot>,
    pub photos: Vec<SystemTablePhoto>,
    pub current_status: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SystemTableWithFields {
    pub table: SystemTable,
    pub fields: Vec<TableField>,
}

#[derive(Debug, Deserialize)]
pub struct CreateSystemTableRequest {
    pub name: String,
    pub display_name: String,
    pub table_type: String,
    pub show_fact_table: Option<bool>,
    pub fact_table_hint: Option<String>,
    pub instruction: Option<String>,
    pub map_link: Option<String>,
    pub status: Option<String>,
    pub status_comment: Option<String>,
    pub location_description: Option<String>,
    pub is_active: Option<bool>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateSystemTableRequest {
    pub display_name: Option<String>,
    pub table_type: Option<String>,
    pub show_fact_table: Option<bool>,
    pub fact_table_hint: Option<String>,
    pub instruction: Option<String>,
    pub map_link: Option<String>,
    pub status: Option<String>,
    pub status_comment: Option<String>,
    pub location_description: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct CreateTimeSlotRequest {
    pub day_of_week: i32,
    pub open_time: String,
    pub close_time: String,
    pub is_next_day: Option<bool>,
    pub is_active: Option<bool>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateTimeSlotRequest {
    pub day_of_week: Option<i32>,
    pub open_time: Option<String>,
    pub close_time: Option<String>,
    pub is_next_day: Option<bool>,
    pub is_active: Option<bool>,
}