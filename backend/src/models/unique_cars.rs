use serde::{Serialize, Deserialize};
use chrono::NaiveDateTime;

#[derive(Debug, Serialize, Deserialize)]
pub struct UniqueCar {
    pub id: i32,
    pub number: String,
    pub mark: String,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub format_id: Option<i32>,
    pub user_id: Option<i32>,
    pub status: bool,
    pub created_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NewUniqueCar {
    pub number: String,
    pub mark: String,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub format_id: Option<i32>,
    pub user_id: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UniqueCarWithRelations {
    pub id: i32,
    pub number: String,
    pub mark: String,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub format_id: Option<i32>,
    pub user_id: Option<i32>,
    pub status: bool,
    pub created_at: Option<NaiveDateTime>,
    pub organization_name: Option<String>,
    pub company_name: Option<String>,
    pub format_name: Option<String>,
    pub user_name: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CarOwnerInfo {
    pub has_organization: bool,
    pub has_company: bool,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub user_id: i32,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct BatchCreateResponse {
    pub created_cars: Vec<UniqueCar>,
    pub errors: Vec<String>,
    pub success_count: usize,
    pub error_count: usize,
}