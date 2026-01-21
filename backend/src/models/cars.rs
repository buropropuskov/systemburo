use serde::{Serialize, Deserialize};
use chrono::{NaiveDateTime, NaiveDate, NaiveTime};

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct Car {
    pub id: i32,
    pub attachment_id: i32,
    pub car_number: String,
    pub car_brand: String,
    pub unload_place: Option<String>,
    pub entry_date_from: NaiveDate,
    pub entry_time_from: NaiveTime,
    pub entry_date_to: NaiveDate,
    pub entry_time_to: NaiveTime,
    pub territory_entry_time: Option<NaiveDateTime>,
    pub territory_status: i32,
    pub status: i32,
    pub date_added: NaiveDate,
    pub date_removed: Option<NaiveDate>,
    pub created_at: NaiveDateTime,
    pub updated_at: NaiveDateTime,
}

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct CarUnloadPlace {
    pub id: i32,
    pub car_id: i32,
    pub unload_place_id: i32,
    pub order_index: i32,
    pub planned_time: Option<NaiveTime>,
    pub notes: Option<String>,
}