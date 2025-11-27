use serde::{Serialize, Deserialize};
use chrono::NaiveDateTime;

#[derive(Debug, Serialize, Deserialize)]
pub struct UniqueEmployee {
    pub id: i32,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub citizenship_id: Option<i32>,
    pub position: Option<String>,
    pub passport_series_number: Option<String>,
    pub patent_number: Option<String>,
    pub other_permission: Option<String>,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub user_id: Option<i32>,
    pub status: bool,
    pub created_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NewUniqueEmployee {
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub citizenship_id: Option<i32>,
    pub position: Option<String>,
    pub passport_series_number: Option<String>,
    pub patent_number: Option<String>,
    pub other_permission: Option<String>,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub user_id: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UniqueEmployeeWithRelations {
    pub id: i32,
    pub last_name: Option<String>,
    pub first_name: Option<String>,
    pub middle_name: Option<String>,
    pub citizenship_id: Option<i32>,
    pub position: Option<String>,
    pub passport_series_number: Option<String>,
    pub patent_number: Option<String>,
    pub other_permission: Option<String>,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub user_id: Option<i32>,
    pub status: bool,
    pub created_at: Option<NaiveDateTime>,
    pub organization_name: Option<String>,
    pub company_name: Option<String>,
    pub citizenship_name: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct EmployeeOwnerInfo {
    pub has_organization: bool,
    pub has_company: bool,
    pub organization_id: Option<i32>,
    pub company_id: Option<i32>,
    pub user_id: i32,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct EmployeeFile {
    pub id: i32,
    pub employee_id: i32,
    pub file_path: String,
    pub file_type: String,
    pub file_name: Option<String>,
    pub uploaded_at: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UploadEmployeeFilesRequest {
    pub files: Vec<EmployeeFileUpload>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct EmployeeFileUpload {
    pub file_type: String,
    pub file_name: String,
    pub file_data: String, // base64 encoded
}