use serde::{Serialize, Deserialize};
use chrono::{DateTime, Utc};
use sqlx::FromRow;

#[derive(Debug, Serialize, Deserialize, FromRow)]
pub struct EmployeeHistory {
    pub id: i32,
    pub employee_id: i32,
    pub user_id: Option<i32>,
    pub table_id: Option<i32>,
    pub action_type: String,
    pub field_name: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub metadata: Option<serde_json::Value>,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Deserialize)]
pub struct AddEmployeeHistoryRequest {
    pub user_id: Option<i32>,
    pub table_id: Option<i32>,
    pub action_type: String,
    pub field_name: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateTerritoryStatusRequest {
    pub territory_status: i32,
    pub user_id: Option<i32>,
    pub table_id: Option<i32>,
}

#[derive(Debug, Deserialize)]
pub struct DeactivateEmployeeRequest {
    pub status: i32,
    pub user_id: Option<i32>,
}

#[derive(Debug, Deserialize)]
pub struct ActivateEmployeeRequest {
    pub user_id: Option<i32>,
}

#[derive(Debug, Deserialize)]
pub struct RestoreEmployeeRequest {
    pub user_id: Option<i32>,
}

#[derive(Debug, Serialize)]
pub struct EmployeeHistoryItem {
    pub id: i32,
    pub employee_id: i32,
    pub user_id: Option<i32>,
    pub table_id: Option<i32>,
    pub table_name: Option<String>,
    pub user_name: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub action_type: String,
    pub field_name: Option<String>,
    pub old_value: Option<String>,
    pub new_value: Option<String>,
    pub comment: Option<String>,
    pub created_at: String,
    pub metadata: Option<serde_json::Value>,
    pub employee_last_name: Option<String>,
    pub employee_first_name: Option<String>,
    pub employee_middle_name: Option<String>,
    pub organization: Option<String>,
    pub company: Option<String>,
}