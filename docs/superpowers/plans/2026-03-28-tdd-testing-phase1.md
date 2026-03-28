# E2E тестирование systemburo — Фаза 1: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Создать тестовую инфраструктуру и E2E тесты авторизации для backend systemburo, чтобы зафиксировать API-контракты перед миграцией на Go.

**Architecture:** Black-box HTTP тесты через reqwest против реального actix-web сервера с тестовой PostgreSQL в Docker. Каждый тест создаёт изолированную БД, запускает миграции, выполняет HTTP-запросы, проверяет ответы, удаляет БД.

**Tech Stack:** Rust, actix-web 4, sqlx 0.8 (+ migrate), reqwest 0.12, tokio, PostgreSQL 16 (Docker)

**ВАЖНО:** Всё через Docker. Локальный Rust НЕ нужен.

---

## Текущий статус

- [x] Task 1: Cargo.toml обновлён (dev-dependencies + migrate feature)
- [x] Частично: миграция создана, но НЕ СООТВЕТСТВУЕТ реальной схеме БД
- [x] TestApp, хелперы, lib.rs, auth_test.rs — созданы
- [x] docker-compose.yml — добавлен test сервис
- [ ] **БЛОКЕР: Нужен дамп реальной БД** — миграция из кода не совпадает со схемой

## Что нужно сделать когда будет дамп

### Task 0: Применить дамп БД (БЛОКЕР)

- [ ] **Step 1: Получить дамп от пользователя**

Пользователь предоставит SQL-дамп реальной БД (pg_dump).

- [ ] **Step 2: Заменить миграцию на реальный дамп**

Взять schema из дампа (только CREATE TABLE, без данных) и заменить файл:
```
backend/migrations/20260328000000_initial_schema.sql
```

Если дамп содержит данные — вырезать INSERT'ы, оставить только DDL + seed для user_types.

- [ ] **Step 3: Применить к Docker PostgreSQL**

```bash
# Пересоздать схему
docker compose exec db psql -U postgres -d auto_registry -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Применить дамп
docker compose exec -T db psql -U postgres -d auto_registry < backend/migrations/20260328000000_initial_schema.sql

# Проверить таблицы
docker compose exec db psql -U postgres -d auto_registry -c "\dt"
```

- [ ] **Step 4: Пересобрать и запустить тесты**

```bash
# Собрать test-образ (sqlx проверит schema при компиляции)
docker compose --profile test build test

# Запустить тесты
docker compose --profile test run --rm test
```

---

## Уже созданные файлы

```
backend/
├── migrations/
│   └── 20260328000000_initial_schema.sql   # ЗАМЕНИТЬ на реальный дамп
├── tests/
│   ├── common/
│   │   ├── mod.rs                          # Re-exports
│   │   ├── setup.rs                        # TestApp: сервер + тестовая БД
│   │   ├── auth_helper.rs                  # register, login, auth_get/post/put/delete
│   │   └── db_helper.rs                    # create_test_organization, create_test_company
│   └── auth_test.rs                        # 1 тест (register_success)
├── src/
│   └── lib.rs                              # pub mod для тестов
├── Cargo.toml                              # + dev-dependencies, migrate
└── Dockerfile                              # dev stage с cargo-watch
```

docker-compose.yml — добавлен `test` сервис с `profiles: [test]`

---

## Task 1: Дописать остальные auth тесты

После того как дамп применён и `register_success` проходит, дописать в `auth_test.rs`:

### Тесты регистрации (3 теста)
```rust
#[tokio::test]
async fn register_success() { /* уже есть */ }

#[tokio::test]
async fn register_duplicate_username() {
    let app = TestApp::spawn().await;
    register_user(&app, "dupuser", "Password123!").await;
    let r2 = register_user(&app, "dupuser", "Password123!").await;
    assert_eq!(r2.status(), 400);
    app.cleanup().await;
}

#[tokio::test]
async fn register_missing_fields() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/register", app.address))
        .json(&serde_json::json!({"username": "incomplete"}))
        .send().await.unwrap();
    assert!(response.status().is_client_error() || response.status().is_server_error());
    app.cleanup().await;
}
```

### Тесты логина (3 теста)
```rust
#[tokio::test]
async fn login_success() {
    let app = TestApp::spawn().await;
    register_user(&app, "loginuser", "Password123!").await;
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&serde_json::json!({"username": "loginuser", "password": "Password123!"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("token").is_some());
    assert!(body.get("refreshToken").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn login_wrong_password() {
    let app = TestApp::spawn().await;
    register_user(&app, "wrongpwd", "CorrectPass123!").await;
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&serde_json::json!({"username": "wrongpwd", "password": "WrongPass!"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

#[tokio::test]
async fn login_nonexistent_user() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&serde_json::json!({"username": "noexist", "password": "AnyPass123!"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}
```

### Тесты refresh token (3 теста)
```rust
#[tokio::test]
async fn refresh_token_success() {
    let app = TestApp::spawn().await;
    register_user(&app, "refreshuser", "Password123!").await;
    let (_, refresh_token) = login(&app, "refreshuser", "Password123!").await;
    let response = app.api_client
        .post(&format!("{}/refresh-token", app.address))
        .json(&serde_json::json!({"refresh_token": refresh_token}))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert!(body.get("token").is_some());
    app.cleanup().await;
}

#[tokio::test]
async fn refresh_token_invalid() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/refresh-token", app.address))
        .json(&serde_json::json!({"refresh_token": "invalid.jwt.token"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

#[tokio::test]
async fn logout_then_refresh_fails() {
    let app = TestApp::spawn().await;
    register_user(&app, "logoutuser", "Password123!").await;
    let (access_token, refresh_token) = login(&app, "logoutuser", "Password123!").await;
    // Logout
    app.api_client.post(&format!("{}/logout", app.address))
        .header("Authorization", format!("Bearer {}", access_token))
        .json(&serde_json::json!({"refresh_token": refresh_token.clone()}))
        .send().await.unwrap();
    // Refresh should fail
    let response = app.api_client
        .post(&format!("{}/refresh-token", app.address))
        .json(&serde_json::json!({"refresh_token": refresh_token}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}
```

### Тесты logout (2 теста)
```rust
#[tokio::test]
async fn logout_success() {
    let app = TestApp::spawn().await;
    register_user(&app, "logoutok", "Password123!").await;
    let (access_token, refresh_token) = login(&app, "logoutok", "Password123!").await;
    let response = app.api_client
        .post(&format!("{}/logout", app.address))
        .header("Authorization", format!("Bearer {}", access_token))
        .json(&serde_json::json!({"refresh_token": refresh_token}))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    app.cleanup().await;
}

#[tokio::test]
async fn logout_no_auth_header() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .post(&format!("{}/logout", app.address))
        .json(&serde_json::json!({"refresh_token": "some.token"}))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}
```

### Тесты user-data и user-me (4 теста)
```rust
#[tokio::test]
async fn get_user_data_authenticated() {
    let app = TestApp::spawn().await;
    register_user(&app, "datauser", "Password123!").await;
    let (token, _) = login(&app, "datauser", "Password123!").await;
    let response = auth_get(&app, "/user-data", &token).await;
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert_eq!(body["username"], "datauser");
    app.cleanup().await;
}

#[tokio::test]
async fn get_user_data_no_token() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(&format!("{}/user-data", app.address))
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

#[tokio::test]
async fn get_user_data_invalid_token() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(&format!("{}/user-data", app.address))
        .header("Authorization", "Bearer invalid.jwt.token")
        .send().await.unwrap();
    assert_eq!(response.status(), 401);
    app.cleanup().await;
}

#[tokio::test]
async fn get_user_me() {
    let app = TestApp::spawn().await;
    register_user(&app, "meuser", "Password123!").await;
    let (token, _) = login(&app, "meuser", "Password123!").await;
    let response = auth_get(&app, "/users/me", &token).await;
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    assert_eq!(body["username"], "meuser");
    assert!(body.get("type_id").is_some());
    app.cleanup().await;
}
```

### Тест user-types (1 тест)
```rust
#[tokio::test]
async fn get_user_types() {
    let app = TestApp::spawn().await;
    let response = app.api_client
        .get(&format!("{}/user-types", app.address))
        .send().await.unwrap();
    assert_eq!(response.status(), 200);
    let body: serde_json::Value = response.json().await.unwrap();
    let types = body.as_array().expect("Should be array");
    assert!(types.len() >= 1);
    assert!(types[0].get("id").is_some());
    assert!(types[0].get("name").is_some());
    assert!(types[0].get("code").is_some());
    app.cleanup().await;
}
```

---

## Task 2: Запуск и верификация

```bash
# Пересобрать
docker compose --profile test build test

# Запустить все 15 тестов
docker compose --profile test run --rm test

# Ожидаемый результат:
# test result: ok. 15 passed; 0 failed;
```

---

## Task 3: Коммит

```bash
git add backend/migrations/ backend/tests/ backend/src/lib.rs backend/Cargo.toml backend/Cargo.lock docker-compose.yml
git commit -m "test: E2E тесты авторизации (15 тестов) + миграции + тестовая инфраструктура

- Добавлена SQL-миграция из реального дампа БД
- TestApp: изолированная тестовая БД для каждого теста
- 15 auth тестов: register, login, refresh, logout, user-data, user-types
- Docker test сервис: docker compose --profile test run test"
```

---

## Декомпозиция проекта на 4 фазы (напоминание)

| Фаза | Scope | Тесты | Статус |
|------|-------|-------|--------|
| **1 (эта)** | Инфраструктура + Auth | ~15 | В работе |
| 2 | CRUD (orgs, companies, citizenships, user_types, formats, feedback) | ~40 | Не начата |
| 3 | Основные сущности (users, unique_cars/employees, unload_places, tables) | ~50 | Не начата |
| 4 | Заявки + workflow (applications, cars, employees, history) | ~30 | Не начата |
