// models/organizations.rs
use serde::{Serialize, Deserialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationInfo {
    pub id: i32,
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NewOrganization {
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationWithUsers {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationUnloadPlace {
    pub id: i32,
    pub name: String,
    pub description: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationTable {
    pub id: i32,
    pub name: String,
    pub display_name: String,
    pub table_type: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationWithUsersExtended {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
    pub unload_places: Vec<OrganizationUnloadPlace>,
    pub tables: Vec<OrganizationTable>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationUser {
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
pub struct UpdateOrganizationUsersRequest {
    pub users: Vec<OrganizationUserRequest>, // Изменено с user_ids на users
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationUserRequest {
    pub username: String,
    pub is_primary: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateOrganizationUnloadPlacesRequest {
    pub unload_place_ids: Vec<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateOrganizationTablesRequest {
    pub table_ids: Vec<i32>,
}