use serde::{Serialize, Deserialize};
use chrono::{NaiveDate, NaiveTime};

use crate::models::unload_places::CarUnloadPlace;

#[derive(Debug, Serialize, Deserialize)]
pub struct NewCar {
    pub car_number: Option<String>,
    pub car_brand: Option<String>,
    pub status: Option<i32>,
    pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Car {
    pub id: Option<i32>,
    pub application_id: Option<i32>,
    pub car_number: Option<String>,
    pub car_brand: Option<String>,
    pub unload_place: Option<String>,
    pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
    pub status: Option<i32>,
    pub date_added: Option<NaiveDate>,
    pub date_removed: Option<NaiveDate>,
    pub unload_places: Option<Vec<CarUnloadPlace>>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CarWithUnloadPlaces {
    pub car: NewCar,
    pub unload_places: Vec<CarUnloadPlace>,
}
