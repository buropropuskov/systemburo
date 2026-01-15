use serde::{Serialize, Deserialize};
use chrono::{NaiveDate, NaiveTime, NaiveDateTime};

use crate::models::cars::CarWithUnloadPlaces;

#[derive(Debug, Serialize, Deserialize)]
pub struct Application {
    pub id: Option<i32>,
    pub organization: Option<String>,
    pub responsible_person: Option<String>,
    pub contact_phone: Option<String>,
    pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
    pub submission_datetime: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationWithCars {
    pub id: Option<i32>,
    pub organization: Option<String>,
    pub responsible_person: Option<String>,
    pub contact_phone: Option<String>,
    pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
    pub submission_datetime: Option<NaiveDateTime>,
    pub cars: Vec<crate::models::cars::Car>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationSubmitRequest {
    pub message: Option<String>,
    pub application: Application,
    pub cars: Vec<CarWithUnloadPlaces>,
}

// Добавьте эти структуры в models/applications.rs

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationItem {
    pub id: Option<i32>,
    pub application_id: Option<i32>,
    pub item_name: String,
    pub quantity: i32,
    pub description: Option<String>,
    pub order_index: i32,
    pub created_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ItemApplicationSubmitRequest {
    pub message: Option<String>,
    pub application: Application,
    pub items: Vec<ApplicationItem>,
}