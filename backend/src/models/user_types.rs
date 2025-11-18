// models/user_types.rs
use serde::{Serialize, Deserialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct UserType {
    pub id: i32,
    pub name: String,
    pub code: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UserTypeWithCount {
    pub id: i32,
    pub name: String,
    pub code: String,
    pub users_count: i64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateUserTypeRequest {
    pub name: String,
    pub code: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateUserTypeRequest {
    pub name: String,
    pub code: String,
}