# Спек: E2E тестирование systemburo — Фаза 1 (Инфраструктура + Auth)

## Цель

Покрыть Rust backend E2E API-тестами для фиксации поведения перед миграцией на Go. Тесты работают как чёрный ящик — шлют HTTP-запросы, проверяют ответы. Код Rust не рефакторится.

**Фаза 1** — фундамент: SQL-миграции, тестовая инфраструктура, тесты авторизации.

## Контекст

- Backend: Rust/Actix-web 4, sqlx 0.8, PostgreSQL
- 121 эндпоинт, 0 тестов, 0 миграций
- Нет БД и схемы — нужно извлечь из кода
- Цель тестов: зафиксировать API-контракты для 1-в-1 переноса на Go

## Декомпозиция проекта на 4 фазы

| Фаза | Scope | Тесты |
|------|-------|-------|
| **1 (эта)** | Инфраструктура + Auth | ~15 |
| 2 | CRUD-сущности (orgs, companies, citizenships, user_types, formats, feedback) | ~40 |
| 3 | Основные сущности (users, unique_cars/employees, unload_places, tables, attachments) | ~50 |
| 4 | Заявки + workflow (applications, cars, employees, history) | ~30 |

Каждая фаза — отдельный spec → plan → implementation цикл.

---

## Архитектура тестирования

### Тип тестов
- **E2E / Black-box API тесты** через HTTP
- Реальная PostgreSQL (тестовая БД в Docker)
- Не трогают внутреннюю структуру handlers
- Фиксируют: HTTP status codes, JSON-структуру ответов, бизнес-логику

### Инструменты
- `reqwest` — HTTP клиент для тестов
- `sqlx` с фичей `migrate` — автоматические миграции
- `tokio` test runtime
- `serde_json` — проверка JSON ответов
- `uuid` — уникальные имена тестовых БД

### Структура файлов

```
backend/
├── tests/                              # Integration tests
│   ├── common/
│   │   ├── mod.rs                      # Re-exports
│   │   ├── setup.rs                    # TestApp: сервер + тестовая БД
│   │   ├── auth_helper.rs              # Хелперы: register, login, auth_get/post
│   │   └── db_helper.rs               # Seed данные, cleanup
│   └── auth_test.rs                    # Тесты авторизации (Фаза 1)
├── migrations/
│   └── 20260328000000_initial_schema.sql  # Полная схема БД
└── Cargo.toml                          # + dev-dependencies
```

---

## Шаг 1: SQL-миграции

### Задача
Извлечь полную схему PostgreSQL из sqlx макросов и handler-запросов. Создать единый файл миграции.

### Источники схемы
- `sqlx::query!()` и `sqlx::query_as!()` макросы — содержат имена таблиц, колонок, типы
- Модели в `src/models/` — структуры с `sqlx::FromRow`
- INSERT/UPDATE запросы в handlers — определяют NOT NULL, FK constraints

### Таблицы (~30)

**Ядро:**
- `users` (id, username, password, organization_id, company_id, type_id, last_name, first_name, middle_name, position, email, phone)
- `user_types` (id, name, code)
- `organizations` (id, name)
- `companies` (id, name)
- `refresh_tokens` (id, user_id, token_hash, expires_at, created_at, is_revoked)

**Заявки:**
- `applications` (id, application_number, confirmation, sending_datetime, reading_datetime, confirmation_datetime, organization_id, company_id, sender_user_id, status, responsible_user_id, responsible_comment, data_approval)
- `application_approvers` (id, user_id, created_at, created_by)
- `application_viewers` (id, application_id, user_id, created_at, created_by)
- `application_history` (id, application_id, user_id, action_type, action_status, old_value, new_value, comment, created_at, metadata)

**Вложения:**
- `unique_attachments` (id, attachment_type, status, ...)
- `attachments` (id, application_id, unique_attachment_id, entry_date_from, entry_date_to, entry_time_from, entry_time_to, status)

**Транспорт и сотрудники:**
- `cars` (id, attachment_id, car_number, car_brand, territory_entry_time, territory_status, status, ...)
- `car_unload_places` (id, car_id, unload_place_id, order_index, planned_time, notes)
- `car_history` (id, car_id, user_id, action_type, field_name, old_value, new_value, comment, created_at, metadata)
- `employees` (id, attachment_id, last_name, first_name, middle_name, citizenship_id, position, ...)
- `employee_target_tables` (id, employee_id, table_id, order_index)
- `items` (id, attachment_id, name, count, date_created, date_deleted)

**Справочники:**
- `citizenships` (id, name, icon, is_active, is_default, patent_required)
- `license_plate_formats` (id, name, country_code, icon, is_active, is_default)
- `license_plate_format_cells` (id, format_id, cell_order, cell_type, min_length, max_length, ...)
- `unique_cars` (id, number, mark, organization_id, company_id, format_id, user_id, status, created_at)
- `unique_employees` (id, last_name, first_name, ..., organization_id, company_id, user_id, status, created_at)

**Места и таблицы:**
- `unload_places` (id, name, description, map_link, status, status_comment, is_active)
- `unload_place_time_slots` (id, unload_place_id, day_of_week, open_time, close_time, is_next_day, is_active)
- `unload_place_photos` (id, unload_place_id, photo_url, file_name, file_size, mime_type, is_main, uploaded_at, uploaded_by)
- `system_tables` (id, name, display_name, table_type, show_fact_table, fact_table_hint, instruction, map_link, status, status_comment, location_description, is_active, created_at, updated_at)
- `system_table_time_slots` (аналог unload_place_time_slots)
- `system_table_photos` (аналог unload_place_photos)

**Связи (junction tables):**
- `organization_unload_places` (organization_id, unload_place_id)
- `company_unload_places` (company_id, unload_place_id)
- `organization_tables` (organization_id, table_id)
- `company_tables` (company_id, table_id)
- `organization_users` или FK в users
- `company_users` или FK в users

**Обратная связь:**
- `feedback` (id, user_id, message, status, is_read, created_at, updated_at)

### Формат миграции
Файл: `migrations/20260328000000_initial_schema.sql`
- Все CREATE TABLE с типами, PK, FK, NOT NULL
- CREATE INDEX для часто используемых полей
- Seed данных для user_types (минимум: manager, buropropuskov, dispatcher)

---

## Шаг 2: Тестовая инфраструктура

### TestApp (`tests/common/setup.rs`)

```rust
pub struct TestApp {
    pub address: String,          // http://127.0.0.1:{random_port}
    pub port: u16,
    pub db_pool: PgPool,
    pub db_name: String,
    pub api_client: reqwest::Client,
}
```

**spawn():**
1. Генерирует уникальное имя БД: `test_systemburo_{uuid}`
2. Подключается к PostgreSQL (из TEST_DATABASE_URL или DATABASE_URL)
3. Создаёт тестовую БД: `CREATE DATABASE test_systemburo_{uuid}`
4. Запускает миграции: `sqlx::migrate!("./migrations").run(&pool)`
5. Seed: вставляет базовые user_types
6. Создаёт actix_web App с тем же конфигом что main.rs (routes, middleware)
7. Биндит на случайный порт (port 0)
8. Возвращает TestApp

**cleanup():**
1. Закрывает пул к тестовой БД
2. Подключается к postgres (default DB)
3. `DROP DATABASE test_systemburo_{uuid}`

### Auth Helper (`tests/common/auth_helper.rs`)

```rust
/// Регистрирует пользователя, возвращает Response
pub async fn register_user(app: &TestApp, username: &str, password: &str) -> reqwest::Response

/// Логинится, возвращает (access_token, refresh_token)
pub async fn login(app: &TestApp, username: &str, password: &str) -> (String, String)

/// GET с Bearer токеном
pub async fn auth_get(app: &TestApp, path: &str, token: &str) -> reqwest::Response

/// POST с Bearer токеном и JSON body
pub async fn auth_post(app: &TestApp, path: &str, token: &str, body: &serde_json::Value) -> reqwest::Response

/// PUT с Bearer токеном и JSON body
pub async fn auth_put(app: &TestApp, path: &str, token: &str, body: &serde_json::Value) -> reqwest::Response

/// DELETE с Bearer токеном
pub async fn auth_delete(app: &TestApp, path: &str, token: &str) -> reqwest::Response

/// Создаёт и логинит тестового пользователя, возвращает токен
pub async fn create_authenticated_user(app: &TestApp, suffix: &str) -> (String, String) // (token, username)
```

### DB Helper (`tests/common/db_helper.rs`)

```rust
/// Seed базовых user_types (если не в миграции)
pub async fn seed_user_types(pool: &PgPool)

/// Создать тестовую организацию
pub async fn create_test_organization(pool: &PgPool, name: &str) -> i32

/// Создать тестовую компанию
pub async fn create_test_company(pool: &PgPool, name: &str) -> i32
```

---

## Шаг 3: Тесты авторизации (`tests/auth_test.rs`)

### Тест-план: 15 тестов

| # | Имя теста | Метод | Путь | Проверяет |
|---|-----------|-------|------|-----------|
| 1 | `register_success` | POST | /register | status 200, пользователь создан |
| 2 | `register_duplicate_username` | POST | /register | status 400/409, ошибка дубликата |
| 3 | `register_missing_fields` | POST | /register | status 400, валидация |
| 4 | `login_success` | POST | /login | status 200, JSON: access_token + refresh_token |
| 5 | `login_wrong_password` | POST | /login | status 401 |
| 6 | `login_nonexistent_user` | POST | /login | status 401 |
| 7 | `refresh_token_success` | POST | /refresh-token | status 200, новый access_token |
| 8 | `refresh_token_invalid` | POST | /refresh-token | status 401 |
| 9 | `logout_success` | POST | /logout | status 200 |
| 10 | `logout_then_refresh_fails` | POST | /logout → /refresh-token | refresh возвращает 401 |
| 11 | `get_user_data_authenticated` | GET | /user-data | status 200, JSON с данными пользователя |
| 12 | `get_user_data_no_token` | GET | /user-data | status 401 |
| 13 | `get_user_data_invalid_token` | GET | /user-data | status 401 |
| 14 | `get_user_me` | GET | /users/me | status 200, JSON с username, type_id |
| 15 | `get_user_types` | GET | /user-types | status 200, массив типов |

### Паттерн теста

```rust
#[tokio::test]
async fn login_success() {
    // Arrange
    let app = TestApp::spawn().await;
    register_user(&app, "testuser", "Password123!").await;

    // Act
    let response = app.api_client
        .post(&format!("{}/login", app.address))
        .json(&json!({"username": "testuser", "password": "Password123!"}))
        .send()
        .await
        .expect("Failed to send request");

    // Assert
    assert_eq!(response.status(), 200);
    let body: Value = response.json().await.unwrap();
    assert!(body.get("access_token").is_some());
    assert!(body.get("refresh_token").is_some());

    // Cleanup
    app.cleanup().await;
}
```

---

## Шаг 4: Изменения в Cargo.toml

```toml
[dev-dependencies]
reqwest = { version = "0.12", features = ["json"] }
tokio = { version = "1", features = ["full", "test-util"] }
serde_json = "1.0"
uuid = { version = "1", features = ["v4"] }

[dependencies.sqlx]
# Добавить фичу "migrate":
features = ["postgres", "runtime-tokio-native-tls", "macros", "chrono", "uuid", "migrate"]
```

---

## Запуск тестов

### Предварительные требования
- Docker (для PostgreSQL)
- `docker compose up db` — поднять тестовую БД

### Команды
```bash
# Установить sqlx-cli (для миграций)
cargo install sqlx-cli --no-default-features --features postgres

# Запустить миграции
cargo sqlx migrate run

# Запустить тесты
DATABASE_URL=postgres://postgres:123@localhost/auto_registry cargo test

# Запустить только auth тесты
cargo test --test auth_test
```

---

## Критерии успеха

1. Все 15 auth тестов проходят
2. TestApp поднимает изолированную тестовую БД
3. Тесты не влияют друг на друга (параллельные БД)
4. Миграция создаёт полную схему (~30 таблиц)
5. `cargo test` работает с Docker PostgreSQL
