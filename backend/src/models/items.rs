use serde::{Serialize, Deserialize};
use chrono::{NaiveDateTime, NaiveDate};

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct Item {
    pub id: i32,
    pub attachment_id: i32,
    pub name: String,
    pub count: i32,
    pub date_created: NaiveDate,
    pub date_deleted: Option<NaiveDate>,
    pub created_at: NaiveDateTime,
    pub updated_at: NaiveDateTime,
}