use jsonwebtoken::{encode, decode, Header, Validation, EncodingKey, DecodingKey};
use chrono::{Utc, Duration};
use argon2::{Argon2, PasswordHash, PasswordHasher, PasswordVerifier};
use argon2::password_hash::SaltString;
use rand::thread_rng;
use crate::models::auth::Claims;

pub fn create_token(username: &str, type_id: i32) -> String {
    let expiration = Utc::now() + Duration::hours(24);
    let claims = Claims {
        sub: username.to_string(),
        exp: expiration.timestamp() as usize,
        type_id,  // Changed from role to type_id
    };
    encode(
        &Header::default(),
        &claims,
        &EncodingKey::from_secret(b"secret"))
        .expect("Failed to create token")
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

pub fn decode_token(token: &str) -> Result<Claims, jsonwebtoken::errors::Error> {
    decode::<Claims>(
        &token,
        &DecodingKey::from_secret(b"secret"),
        &Validation::default(),
    )
    .map(|data| data.claims)
}
