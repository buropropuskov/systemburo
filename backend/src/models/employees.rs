use serde::{Serialize, Deserialize};
use chrono::{NaiveDateTime, NaiveDate};

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct Employee {
    pub id: i32,
    pub attachment_id: i32,
    pub last_name: String,
    pub first_name: String,
    pub middle_name: Option<String>,
    pub citizenship_id: Option<i32>,
    pub position: Option<String>,
    pub passport_series_number: Option<String>,
    pub patent_number: Option<String>,
    pub other_permission: Option<String>,
    pub territory_entry_time: Option<NaiveDateTime>,
    pub territory_status: i32,
    pub status: i32,
    pub date_created: NaiveDate,
    pub date_deleted: Option<NaiveDate>,
    pub created_at: NaiveDateTime,
    pub updated_at: NaiveDateTime,
}

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct EmployeeTargetTable {
    pub id: i32,
    pub employee_id: i32,
    pub table_id: i32,
    pub order_index: i32,
}