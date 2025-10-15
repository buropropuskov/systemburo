use serde::{Serialize, Deserialize};
use chrono::NaiveTime;

#[derive(Debug, Serialize, Deserialize)]
pub struct UnloadPlace {
    pub id: i32,
    pub name: String,
    pub description: Option<String>,
    pub is_active: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CarUnloadPlace {
    pub id: Option<i32>,
    pub car_id: Option<i32>,
    pub unload_place_id: i32,
    pub unload_place_name: Option<String>,
    pub order_index: i32,
    pub planned_time: Option<NaiveTime>,
    pub notes: Option<String>,
}
