use serde::{Serialize, Deserialize};
use chrono::NaiveDateTime;

#[derive(Debug, Serialize, Deserialize)]
pub struct Citizenship {
    pub id: i32,
    pub name: String,
    pub icon: Option<String>,
    pub is_active: Option<bool>,
    pub is_default: Option<bool>,
    pub patent_required: Option<bool>,
    pub created_at: Option<NaiveDateTime>,
    pub updated_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateCitizenshipRequest {
    pub name: String,
    pub icon: Option<String>,
    pub is_default: Option<bool>,
    pub patent_required: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCitizenshipRequest {
    pub name: String,
    pub icon: Option<String>,
    pub is_active: Option<bool>,
    pub is_default: Option<bool>,
    pub patent_required: Option<bool>,
}