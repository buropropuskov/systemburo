use serde::{Serialize, Deserialize};
use chrono::{NaiveTime, NaiveDateTime};

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct UnloadPlace {
    pub id: i32,
    pub name: String,
    pub description: Option<String>,
    pub map_link: Option<String>,
    pub status: Option<String>,
    pub status_comment: Option<String>,
    pub is_active: Option<bool>,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct UnloadPlaceTimeSlot {
    pub id: i32,
    pub unload_place_id: i32,
    pub day_of_week: i32,
    pub open_time: NaiveTime,
    pub close_time: NaiveTime,
    pub is_next_day: bool,
    pub is_active: bool,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct UnloadPlacePhoto {
    pub id: i32,
    pub unload_place_id: i32,
    pub photo_url: String,
    pub file_name: String,
    pub file_size: Option<i32>,
    pub mime_type: Option<String>,
    pub is_main: bool,
    pub uploaded_at: Option<NaiveDateTime>,
    pub uploaded_by: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UnloadPlaceWithDetails {
    pub id: i32,
    pub name: String,
    pub description: Option<String>,
    pub map_link: Option<String>,
    pub status: String,
    pub status_comment: Option<String>,
    pub is_active: Option<bool>,
    pub current_status: String, // "open" или "closed" на основе расписания
    pub time_slots: Vec<UnloadPlaceTimeSlot>,
    pub photos: Vec<UnloadPlacePhoto>,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
}

#[derive(Debug, Deserialize)]
pub struct CreateUnloadPlaceRequest {
    pub name: String,
    pub description: Option<String>,
    pub map_link: Option<String>,
    pub status: Option<String>,
    pub status_comment: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateUnloadPlaceRequest {
    pub name: Option<String>,
    pub description: Option<String>,
    pub map_link: Option<String>,
    pub status: Option<String>,
    pub status_comment: Option<String>,
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