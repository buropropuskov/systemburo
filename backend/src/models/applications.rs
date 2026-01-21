// models/applications.rs
use serde::{Serialize, Deserialize};
use chrono::{DateTime, Utc, NaiveDate};

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct Application {
    pub id: i32,
    pub application_number: String,
    pub confirmation: String,
    pub sending_datetime: DateTime<Utc>,
    pub reading_datetime: Option<DateTime<Utc>>,
    pub confirmation_datetime: Option<DateTime<Utc>>,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub sender_user_id: i32,
    pub message: Option<String>,
    pub status: String,
    pub responsible_user_id: Option<i32>,
    pub responsible_comment: Option<String>,
    pub data_approval: bool,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationWithDetails {
    pub id: i32,
    pub application_number: String,
    pub confirmation: String,
    pub sending_datetime: DateTime<Utc>,
    pub reading_datetime: Option<DateTime<Utc>>,
    pub confirmation_datetime: Option<DateTime<Utc>>,
    pub organization_id: i32,
    pub organization_name: String,
    pub company_id: Option<i32>,
    pub company_name: String,
    pub sender_user_id: i32,
    pub sender_full_name: Option<String>,
    pub sender_name: String,
    pub message: Option<String>,
    pub status: String,
    pub responsible_user_id: Option<i32>,
    pub responsible_full_name: Option<String>,
    pub responsible_name: String,
    pub responsible_comment: Option<String>,
    pub data_approval: bool,
}

#[derive(Debug, Deserialize)]
pub struct ApplicationFilter {
    pub search_query: Option<String>,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub confirmation: Option<String>,
    pub status: Option<String>,
    pub date_from: Option<NaiveDate>,
    pub date_to: Option<NaiveDate>,
}

#[derive(Debug, Deserialize)]
pub struct ApplicationCreateRequest {
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub message: Option<String>,
    pub data_approval: bool,
}

#[derive(Debug, Deserialize)]
pub struct ApplicationUpdateRequest {
    pub confirmation: Option<String>,
    pub status: Option<String>,
    pub responsible_comment: Option<String>,
}

// Новая структура для информации об ответственных
#[derive(Debug, Serialize, Deserialize)]
pub struct ResponsibleUserInfo {
    pub id: i32,
    pub username: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub is_primary: bool,
}