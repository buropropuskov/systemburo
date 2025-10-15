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
pub struct CompanyWithUsersExtended {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
    pub unload_places: Vec<CompanyUnloadPlace>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyUser {
    pub id: i32,
    pub username: String,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub position: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCompanyUsersRequest {
    pub user_ids: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateCompanyUnloadPlacesRequest {
    pub unload_place_ids: Vec<i32>,
}