// models/table_constructor.rs
use serde::{Serialize, Deserialize};
use chrono::{NaiveDateTime, Utc};

#[derive(Debug, Serialize, Deserialize)]
pub struct SystemTable {
    pub id: i32,
    pub name: String,
    pub display_name: String,
    pub table_type: String,
    pub show_fact_table: Option<bool>,
    pub fact_table_hint: Option<String>,
    pub instruction: Option<String>,
    pub is_active: Option<bool>,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
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
pub struct SystemTableWithFields {
    pub table: SystemTable,
    pub fields: Vec<TableField>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateSystemTableRequest {
    pub name: String,
    pub display_name: String,
    pub table_type: String,
    pub show_fact_table: Option<bool>,
    pub fact_table_hint: Option<String>,
    pub instruction: Option<String>,
    pub is_active: Option<bool>, // Добавьте это поле
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateSystemTableRequest {
    pub display_name: String,
    pub table_type: String,
    pub show_fact_table: Option<bool>,
    pub fact_table_hint: Option<String>,
    pub instruction: Option<String>,
}