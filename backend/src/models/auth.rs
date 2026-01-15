use serde::{Deserialize, Serialize};
use chrono::{DateTime, Utc};

#[derive(Debug, Deserialize)]
pub struct UserLogin {
    pub username: String,
    pub password: String,
}

#[derive(Debug, Deserialize)]
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

#[derive(Serialize)]
pub struct LoginResponse {
    pub token: String,
    pub refreshToken: String,
    pub organization: String,
    pub organization_id: i32,
    pub company: String,
    pub company_id: i32,
    pub type_id: i32,
    pub user_type: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Claims {
    pub sub: String,
    pub exp: usize,
    pub type_id: i32,
}

#[derive(Deserialize)]
pub struct RefreshRequest {
    pub refresh_token: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct RefreshTokenRecord {
    pub id: i32,
    pub user_id: i32,
    pub token_hash: String,
    pub expires_at: DateTime<Utc>,
    pub created_at: DateTime<Utc>,
    pub is_revoked: bool,
}

#[derive(Deserialize)]
pub struct LogoutRequest {
    pub refresh_token: String,
}