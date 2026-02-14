use serde::{Serialize, Deserialize};
use chrono::NaiveDateTime;

#[derive(Debug, Serialize, Deserialize, sqlx::FromRow)]
pub struct ApplicationApprover {
    pub id: i32,
    pub user_id: i32,
    pub created_at: Option<NaiveDateTime>,
    pub created_by: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationApproverWithUser {
    pub id: i32,
    pub user_id: i32,
    pub username: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub organization: Option<String>,
    pub company: Option<String>,
    pub created_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateApproverRequest {
    pub user_id: i32,
}