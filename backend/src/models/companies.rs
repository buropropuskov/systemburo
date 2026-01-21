// models/companies.rs
use serde::{Serialize, Deserialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct Company {
    pub id: i32,
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NewCompany {
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyWithUsers {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyUnloadPlace {
    pub id: i32,
    pub name: String,
    pub description: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyTable {
    pub id: i32,
    pub name: String,
    pub display_name: String,
    pub table_type: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyWithUsersExtended {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
    pub unload_places: Vec<CompanyUnloadPlace>,
    pub tables: Vec<CompanyTable>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyUser {
    pub id: i32,
    pub username: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
    pub is_primary: Option<bool>, // Добавлено новое поле
}

// ИЗМЕНЕНИЕ: Обновляем существующую структуру
#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCompanyUsersRequest {
    pub users: Vec<CompanyUserRequest>, // Изменено с user_ids на users
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyUserRequest {
    pub username: String,
    pub is_primary: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCompanyUnloadPlacesRequest {
    pub unload_place_ids: Vec<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCompanyTablesRequest {
    pub table_ids: Vec<i32>,
}