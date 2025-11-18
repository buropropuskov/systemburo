// models/users.rs
use serde::{Serialize, Deserialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct UserRegister {
    pub username: String,
    pub password: String,
    pub organization_id: i32,
    pub company_id: i32,
    pub type_id: i32,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub email: Option<String>,
    pub phone: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UserInfo {
    pub username: String,
    pub organization: String,
    pub organization_id: i32,
    pub company: String,
    pub company_id: i32,
    pub type_id: i32,
    pub user_type: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub email: Option<String>,
    pub phone: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UserData {
    pub username: String,
    pub organization: String,
    pub organization_id: i32,
    pub company: String,
    pub company_id: i32,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub phone: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateUserTypeRequest {
    pub type_id: i32,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateUserRequest {
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub email: Option<String>,
    pub phone: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdatePasswordRequest {
    pub password: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateOrganizationRequest {
    pub organization_id: i32,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCompanyRequest {
    pub company_id: i32,
}