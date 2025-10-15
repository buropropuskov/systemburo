use serde::{Serialize, Deserialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct LicensePlateFormat {
    pub id: i32,
    pub name: String,
    pub country_code: Option<String>,
    pub icon: Option<String>, // Теперь храним путь к иконке
    pub is_active: Option<bool>,
    pub is_default: Option<bool>, // Добавьте это поле
    pub created_at: Option<chrono::NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct LicensePlateFormatCell {
    pub id: i32,
    pub format_id: i32,
    pub cell_order: i32,
    pub cell_type: String,
    pub min_length: i32,
    pub max_length: i32,
    pub allowed_letters: Option<String>,
    pub alphabet_type: Option<String>,
    pub language: Option<String>,
    pub padding_char: Option<String>,
    pub padding_side: Option<String>,
    pub created_at: Option<chrono::NaiveDateTime>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct LicensePlateFormatWithCells {
    pub format: LicensePlateFormat,
    pub cells: Vec<LicensePlateFormatCell>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateLicensePlateFormatRequest {
    pub name: String,
    pub country_code: Option<String>,
    pub icon: Option<String>,
    pub is_default: Option<bool>, // Добавьте это поле
    pub cells: Vec<CreateFormatCellRequest>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CreateFormatCellRequest {
    pub cell_order: i32,
    pub cell_type: String,
    pub min_length: i32,
    pub max_length: i32,
    pub allowed_letters: Option<String>,
    pub alphabet_type: Option<String>,
    pub language: Option<String>,
    pub padding_char: Option<String>,
    pub padding_side: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateLicensePlateFormatRequest {
    pub name: String,
    pub country_code: Option<String>,
    pub icon: Option<String>,
      pub is_default: Option<bool>, // Добавьте это поле
    pub cells: Vec<UpdateFormatCellRequest>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateFormatCellRequest {
    pub id: Option<i32>,
    pub cell_order: i32,
    pub cell_type: String,
    pub min_length: i32,
    pub max_length: i32,
    pub allowed_letters: Option<String>,
    pub alphabet_type: Option<String>,
    pub language: Option<String>,
    pub padding_char: Option<String>,
    pub padding_side: Option<String>,
}