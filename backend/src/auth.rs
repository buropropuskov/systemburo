use jsonwebtoken::{encode, decode, Header, Validation, EncodingKey, DecodingKey};
use chrono::{Utc, Duration};
use argon2::{Argon2, PasswordHash, PasswordHasher, PasswordVerifier};
use argon2::password_hash::SaltString;
use rand::thread_rng;
use crate::models::auth::Claims;

// Создание access token (3 минуты)
pub fn create_token(username: &str, type_id: i32) -> String {
    let expiration = Utc::now() + Duration::minutes(120); // 3 минуты
    let claims = Claims {
        sub: username.to_string(),
        exp: expiration.timestamp() as usize,
        type_id,
    };
    encode(
        &Header::default(),
        &claims,
        &EncodingKey::from_secret(b"secret"))
        .expect("Failed to create token")
}

// Создание refresh token (24 часа)
pub fn create_refresh_token(username: &str) -> String {
    let expiration = Utc::now() + Duration::hours(24); // 24 часа
    let claims = Claims {
        sub: username.to_string(),
        exp: expiration.timestamp() as usize,
        type_id: 0,
    };
    encode(
        &Header::default(),
        &claims,
        &EncodingKey::from_secret(b"refresh_secret"))
        .expect("Failed to create refresh token")
}

use sha2::{Sha256, Digest};

// Хеширование refresh token с SHA256 (консистентное)
pub fn hash_refresh_token(token: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(token.as_bytes());
    let result = hasher.finalize();
    format!("sha256:{}", hex::encode(result))
}

// Декодирование access token
pub fn decode_token(token: &str) -> Result<Claims, jsonwebtoken::errors::Error> {
    decode::<Claims>(
        &token,
        &DecodingKey::from_secret(b"secret"),
        &Validation::default(),
    )
    .map(|data| data.claims)
}

// Декодирование refresh token
pub fn decode_refresh_token(token: &str) -> Result<Claims, jsonwebtoken::errors::Error> {
    decode::<Claims>(
        &token,
        &DecodingKey::from_secret(b"refresh_secret"),
        &Validation::default(),
    )
    .map(|data| data.claims)
}

pub fn hash_password(password: &str) -> String {
    let salt = SaltString::generate(&mut thread_rng());
    let argon2 = Argon2::default();
    argon2.hash_password(password.as_bytes(), &salt)
        .unwrap()
        .to_string()
}

pub fn verify_password(hash: &str, password: &str) -> bool {
    let parsed_hash = PasswordHash::new(hash).unwrap();
    Argon2::default().verify_password(password.as_bytes(), &parsed_hash).is_ok()
}