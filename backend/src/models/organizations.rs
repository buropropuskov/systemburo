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
pub struct OrganizationWithUsersExtended {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
    pub unload_places: Vec<OrganizationUnloadPlace>,
}