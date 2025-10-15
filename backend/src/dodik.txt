use serde::{Serialize, Deserialize};
use chrono::{NaiveDate, NaiveTime, NaiveDateTime};

#[derive(Debug, Serialize, Deserialize)]
pub struct Claims {
    pub sub: String,
    pub exp: usize,
    pub type_id: i32,
}

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
pub struct UserLogin {
    pub username: String,
    pub password: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct LoginResponse {
    pub token: String,
    pub organization: String,
    pub organization_id: i32,
    pub company: String,
    pub company_id: i32,
    pub type_id: i32,
    pub user_type: String, 
}

// Структура для заявки
#[derive(Debug, Serialize, Deserialize)]
pub struct Application {
    pub id: Option<i32>,
    pub organization: Option<String>,
    pub responsible_person: Option<String>,
    pub contact_phone: Option<String>,
    pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
    pub submission_datetime: Option<NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationWithCars {
    pub id: Option<i32>,
    pub organization: Option<String>,
    pub responsible_person: Option<String>,
    pub contact_phone: Option<String>,
    pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
    pub submission_datetime: Option<NaiveDateTime>,
    pub cars: Vec<Car>,
}

// Упрощенная структура для создания машины (без лишних полей)
#[derive(Debug, Serialize, Deserialize)]
pub struct NewCar {
    pub car_number: Option<String>,
    pub car_brand: Option<String>,
    pub status: Option<i32>,
     pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
}

// Полная структура машины (для чтения из БД)
#[derive(Debug, Serialize, Deserialize)]
pub struct Car {
    pub id: Option<i32>,
    pub application_id: Option<i32>,
    pub car_number: Option<String>,
    pub car_brand: Option<String>,
    pub unload_place: Option<String>,
    pub entry_date_from: Option<NaiveDate>,
    pub entry_date_to: Option<NaiveDate>,
    pub entry_time_from: Option<NaiveTime>,
    pub entry_time_to: Option<NaiveTime>,
    pub status: Option<i32>,
    pub date_added: Option<NaiveDate>,
    pub date_removed: Option<NaiveDate>,
    pub unload_places: Option<Vec<CarUnloadPlace>>,
}

// Места разгрузки
#[derive(Debug, Serialize, Deserialize)]
pub struct UnloadPlace {
    pub id: i32,
    pub name: String,
    pub description: Option<String>,
    pub is_active: Option<bool>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CarUnloadPlace {
    pub id: Option<i32>,
    pub car_id: Option<i32>,
    pub unload_place_id: i32,
    pub unload_place_name: Option<String>,
    pub order_index: i32,
    pub planned_time: Option<NaiveTime>,
    pub notes: Option<String>,
}

// Структура для отправки заявки с машинами и местами разгрузки
#[derive(Debug, Serialize, Deserialize)]
pub struct CarWithUnloadPlaces {
    pub car: NewCar,  // Используем NewCar вместо Car
    pub unload_places: Vec<CarUnloadPlace>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ApplicationSubmitRequest {
    pub message: Option<String>,
    pub application: Application,
    pub cars: Vec<CarWithUnloadPlaces>,
}

// Пользовательские данные
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
pub struct UserType {
    pub id: i32,
    pub name: String,
    pub code: String,
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

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationInfo {
    pub id: i32,
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Company {
    pub id: i32,
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NewOrganization {
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NewCompany {
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct OrganizationWithUsers {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompanyWithUsers {
    pub id: i32,
    pub name: String,
    pub user_count: i64,
}