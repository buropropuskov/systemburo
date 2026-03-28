# Бэкенд системы "Бюро пропусков"

Серверная часть системы управления пропусками, построенная на Rust / Actix-web с PostgreSQL.

---

## Содержание

1. [Архитектура бэкенда](#1-архитектура-бэкенда)
2. [API эндпоинты](#2-api-эндпоинты)
3. [Схема базы данных](#3-схема-базы-данных)
4. [Бизнес-логика заявок](#4-бизнес-логика-заявок)
5. [Аутентификация и авторизация](#5-аутентификация-и-авторизация)
6. [Настройка и запуск](#6-настройка-и-запуск)
7. [Известные проблемы и рекомендации](#7-известные-проблемы-и-рекомендации)

---

## 1. Архитектура бэкенда

### Структура модулей

```
backend/src/
├── main.rs          — точка входа, HTTP-сервер, middleware, фоновые задачи
├── routes.rs        — конфигурация маршрутов (~100 эндпоинтов)
├── auth.rs          — JWT токены, хеширование паролей (Argon2)
├── database.rs      — пул соединений PostgreSQL (sqlx)
├── handlers/        — обработчики HTTP-запросов (21 файл)
│   ├── auth.rs                  — регистрация, логин, токены
│   ├── applications.rs          — заявки (основной модуль, 3800+ строк)
│   ├── users.rs                 — управление пользователями
│   ├── organizations.rs         — организации и связи
│   ├── companies.rs             — компании и связи
│   ├── cars.rs                  — машины в заявках
│   ├── cars_history.rs          — история перемещений машин
│   ├── employees.rs             — сотрудники в заявках
│   ├── unload_places.rs         — места разгрузки
│   ├── citizenship.rs           — справочник гражданств
│   ├── user_types.rs            — типы пользователей
│   ├── number_format.rs         — форматы номерных знаков
│   ├── unique_cars.rs           — уникальные машины (справочник)
│   ├── unique_employees.rs      — уникальные сотрудники (справочник)
│   ├── table_constructor.rs     — конструктор системных таблиц
│   ├── items.rs                 — предметы в заявках
│   ├── feedback.rs              — обратная связь
│   ├── application_approvers.rs — согласователи заявок
│   ├── application_history.rs   — история изменений заявок
│   ├── application_viewers.rs   — наблюдатели заявок
│   └── mod.rs
└── models/          — структуры данных (21 файл, зеркально handlers)
    └── mod.rs
```

### Жизненный цикл запроса

```
HTTP Request
    │
    ▼
  CORS (разрешены все origins с credentials)
    │
    ▼
  Rate Limiter (10 req / 60 sec на IP или токен)
    │
    ▼
  Logger (actix-web middleware)
    │
    ▼
  Handler (обработчик из handlers/)
    │
    ▼
  sqlx (асинхронные SQL-запросы)
    │
    ▼
  PostgreSQL
```

Каждый запрос проходит через цепочку middleware в указанном порядке. Rate limiter реализован на основе `DashMap` — потокобезопасной хеш-карты, хранящей счётчики запросов по IP-адресу или JWT-токену. Фоновая задача периодически очищает устаревшие записи.

### Технологический стек

| Компонент | Технология | Версия |
|---|---|---|
| Язык | Rust (edition 2021) | stable |
| Web-фреймворк | Actix-web | 4 |
| Асинхронный runtime | Tokio | latest |
| База данных | PostgreSQL | 14+ |
| SQL-драйвер | sqlx (compile-time проверка SQL) | 0.8.2 |
| JWT | jsonwebtoken | 9.3.0 |
| Хеширование паролей | Argon2 | через argon2 crate |
| Rate limiting | DashMap (in-memory) | latest |
| Сериализация | serde / serde_json | latest |

---

## 2. API эндпоинты

Всего около **100 эндпоинтов**, сгруппированных по ресурсам.

### Авторизация

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/register` | Регистрация нового пользователя |
| `POST` | `/login` | Авторизация (выдача access + refresh токенов) |
| `POST` | `/refresh-token` | Обновление access token по refresh token |
| `POST` | `/logout` | Выход из системы (отзыв refresh token) |
| `GET` | `/user-data` | Данные текущего пользователя (из токена) |
| `GET` | `/users/me` | Полная информация о текущем пользователе |

### Пользователи

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/users/all` | Список всех пользователей |
| `PUT` | `/users/{username}/type` | Изменить тип пользователя |
| `PUT` | `/users/{username}/password` | Изменить пароль пользователя |
| `PUT` | `/users/{username}/organization` | Привязать пользователя к организации |
| `PUT` | `/users/{username}/company` | Привязать пользователя к компании |
| `PUT` | `/users/{username}/info` | Обновить ФИО, должность, email, телефон |
| `DELETE` | `/users/{username}` | Удалить пользователя |

### Типы пользователей

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/user-types` | Получить все типы пользователей |
| `GET` | `/user-types-management` | Типы с количеством привязанных пользователей |
| `POST` | `/user-types-management` | Создать новый тип пользователя |
| `PUT` | `/user-types-management/{id}` | Обновить тип пользователя |
| `DELETE` | `/user-types-management/{id}` | Удалить тип пользователя |

### Заявки (Applications)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/applications` | Получить заявки с фильтрами и пагинацией |
| `POST` | `/applications` | Создать пустую заявку |
| `POST` | `/applications/submit-complete-application` | Отправить полную заявку (с вложениями, машинами, сотрудниками) |
| `GET` | `/applications/user` | Заявки текущего пользователя |
| `GET` | `/applications/{id}` | Детали конкретной заявки |
| `PUT` | `/applications/{id}` | Обновить заявку |
| `GET` | `/applications/{id}/responsible-users` | Ответственные пользователи заявки |
| `GET` | `/applications/{id}/details` | Полные детали заявки (с вложениями, машинами, сотрудниками) |
| `GET` | `/applications/{id}/attachments` | Вложения (бланки) заявки |
| `POST` | `/applications/{id}/update-items-status` | Обновить статус вложений заявки |
| `POST` | `/applications/{id}/forward` | Переслать заявку ответственным пользователям |
| `POST` | `/applications/{id}/approve` | Согласовать или отклонить заявку |
| `GET` | `/applications/{id}/check-approval-status` | Проверить текущий статус согласования |
| `POST` | `/applications/{id}/take-to-work` | Взять заявку в работу |
| `POST` | `/applications/{id}/revoke-from-work` | Отозвать заявку из работы |
| `POST` | `/applications/{id}/restore-to-work` | Вернуть заявку в работу |
| `GET` | `/applications/{id}/history` | История изменений заявки |
| `POST` | `/applications/{id}/revoke-approval` | Отозвать ранее данное согласование |
| `POST` | `/applications/history` | Добавить произвольную запись в историю заявки |
| `GET` | `/applications/{id}/viewers` | Список наблюдателей заявки |

### Машины

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/cars/active-for-tables` | Активные машины для отображения в таблицах |
| `GET` | `/cars/fact-for-tables` | Фактически находящиеся на территории машины |
| `GET` | `/cars/unload-places` | Места разгрузки для машин |
| `GET` | `/cars/fact-unload-places` | Фактические места разгрузки |
| `GET` | `/cars/check-active` | Проверка активности машины |
| `GET` | `/cars/{id}/history` | История конкретной машины |
| `POST` | `/cars/{id}/history` | Добавить запись в историю машины |
| `GET` | `/cars/history/all` | Полная история всех машин |
| `GET` | `/cars/history/current-status` | Текущий статус всех машин на территории |
| `PUT` | `/cars/{id}/territory-status` | Обновить статус нахождения на территории |
| `PUT` | `/cars/{id}/deactivate` | Деактивировать машину |
| `PUT` | `/cars/{id}/activate` | Активировать машину |
| `GET` | `/cars/history/unified` | Единая (объединённая) история машин |
| `PUT` | `/cars/{id}/restore` | Восстановить деактивированную машину |

### Сотрудники

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/employees/active-for-table/{table_id}` | Активные сотрудники для указанной таблицы |

### Уникальные машины (справочник)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/unique-cars` | Все уникальные машины |
| `POST` | `/unique-cars` | Создать запись уникальной машины |
| `POST` | `/unique-cars/batch` | Пакетное создание нескольких машин |
| `PUT` | `/unique-cars/{id}` | Обновить данные машины |
| `DELETE` | `/unique-cars/{id}` | Удалить машину из справочника |
| `GET` | `/unique-cars/ownership-info` | Информация о владении машинами |
| `PUT` | `/unique-cars/by-number` | Обновить машину по государственному номеру |

### Уникальные сотрудники (справочник)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/unique-employees` | Все уникальные сотрудники |
| `POST` | `/unique-employees` | Создать запись сотрудника |
| `PUT` | `/unique-employees/{id}` | Обновить данные сотрудника |
| `DELETE` | `/unique-employees/{id}` | Удалить сотрудника из справочника |
| `GET` | `/unique-employees/ownership-info` | Информация о владении (привязка к компаниям) |

### Места разгрузки

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/unload-places` | Все места разгрузки |
| `POST` | `/unload-places` | Создать место разгрузки |
| `GET` | `/unload-places/{id}` | Получить место по ID |
| `PUT` | `/unload-places/{id}` | Обновить место разгрузки |
| `DELETE` | `/unload-places/{id}` | Удалить место разгрузки |
| `GET` | `/unload-places/{id}/time-slots` | Временные слоты места |
| `POST` | `/unload-places/{id}/time-slots` | Добавить временной слот |
| `PUT` | `/unload-places/{place_id}/time-slots/{slot_id}` | Обновить временной слот |
| `DELETE` | `/unload-places/{place_id}/time-slots/{slot_id}` | Удалить временной слот |
| `POST` | `/unload-places/{id}/photos` | Загрузить фото места |
| `DELETE` | `/unload-places/{place_id}/photos/{photo_id}` | Удалить фото |
| `POST` | `/unload-places/{place_id}/photos/{photo_id}/main` | Установить фото как главное |

### Организации

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/organizations` | Все организации |
| `POST` | `/organizations` | Создать организацию |
| `PUT` | `/organizations/{id}` | Обновить организацию |
| `DELETE` | `/organizations/{id}` | Удалить организацию |
| `GET` | `/organizations/with-users` | Организации с количеством пользователей |
| `GET` | `/organizations/with-users-extended` | Организации с расширенной информацией о пользователях |
| `GET` | `/organizations/{id}/unload-places` | Места разгрузки организации |
| `PUT` | `/organizations/{id}/unload-places` | Обновить привязку мест разгрузки |
| `GET` | `/organizations/{id}/users` | Пользователи организации |
| `PUT` | `/organizations/{id}/users` | Обновить привязку пользователей |
| `GET` | `/organizations/{id}/tables` | Системные таблицы организации |
| `PUT` | `/organizations/{id}/tables` | Обновить привязку таблиц |
| `GET` | `/get-organization` | Организация текущего пользователя |

### Компании

Аналогичная структура эндпоинтов, как у организаций:

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/companies` | Все компании |
| `POST` | `/companies` | Создать компанию |
| `PUT` | `/companies/{id}` | Обновить компанию |
| `DELETE` | `/companies/{id}` | Удалить компанию |
| `GET` | `/companies/with-users` | Компании с количеством пользователей |
| `GET` | `/companies/with-users-extended` | Компании с расширенной информацией |
| `GET` | `/companies/{id}/unload-places` | Места разгрузки компании |
| `PUT` | `/companies/{id}/unload-places` | Обновить привязку мест разгрузки |
| `GET` | `/companies/{id}/users` | Пользователи компании |
| `PUT` | `/companies/{id}/users` | Обновить привязку пользователей |
| `GET` | `/companies/{id}/tables` | Системные таблицы компании |
| `PUT` | `/companies/{id}/tables` | Обновить привязку таблиц |

### Форматы номерных знаков

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/license-plate-formats` | Все форматы номеров |
| `POST` | `/license-plate-formats` | Создать формат |
| `PUT` | `/license-plate-formats/{id}` | Обновить формат |
| `DELETE` | `/license-plate-formats/{id}` | Удалить формат |

### Согласователи заявок

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/application-approvers` | Все согласователи |
| `GET` | `/application-approvers/available-users` | Доступные пользователи для назначения |
| `POST` | `/application-approvers` | Добавить согласователя |
| `DELETE` | `/application-approvers/{id}` | Удалить согласователя |

### Гражданства

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/citizenships` | Все гражданства |
| `POST` | `/citizenships` | Создать гражданство |
| `PUT` | `/citizenships/{id}` | Обновить гражданство |
| `DELETE` | `/citizenships/{id}` | Удалить гражданство |
| `POST` | `/citizenships/clear-default` | Сбросить гражданство по умолчанию |

### Конструктор таблиц (системные таблицы)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/system-tables` | Все системные таблицы |
| `POST` | `/system-tables` | Создать таблицу |
| `GET` | `/system-tables/{id}` | Получить таблицу по ID |
| `PUT` | `/system-tables/{id}` | Обновить таблицу |
| `DELETE` | `/system-tables/{id}` | Удалить таблицу |
| `GET` | `/system-tables/name/{name}` | Получить таблицу по имени |
| `GET` | `/system-tables/{id}/time-slots` | Временные слоты таблицы |
| `POST` | `/system-tables/{id}/time-slots` | Добавить временной слот |
| `PUT` | `/system-tables/{table_id}/time-slots/{slot_id}` | Обновить слот |
| `DELETE` | `/system-tables/{table_id}/time-slots/{slot_id}` | Удалить слот |
| `POST` | `/system-tables/{id}/photos` | Загрузить фото |
| `DELETE` | `/system-tables/{table_id}/photos/{photo_id}` | Удалить фото |
| `POST` | `/system-tables/{table_id}/photos/{photo_id}/main` | Установить главное фото |

### Вложения (бланки заявок)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/attachments` | Активные вложения |
| `GET` | `/attachments/all` | Все вложения (включая деактивированные) |
| `POST` | `/attachments` | Создать вложение |
| `PUT` | `/attachments/{id}` | Обновить вложение |
| `DELETE` | `/attachments/{id}` | Удалить (деактивировать) вложение |
| `PUT` | `/attachments/{id}/restore` | Восстановить деактивированное вложение |
| `GET` | `/attachments/{id}` | Получить вложение по ID |
| `GET` | `/attachments/{id}/cars` | Машины, привязанные к вложению |
| `GET` | `/attachments/{id}/employees` | Сотрудники, привязанные к вложению |
| `GET` | `/attachments/{id}/items` | Предметы, привязанные к вложению |

### Обратная связь

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/feedback` | Создать обращение |
| `GET` | `/feedback/all` | Все обращения (для администраторов) |
| `GET` | `/feedback/stats` | Статистика обращений |
| `GET` | `/feedback/my` | Обращения текущего пользователя |
| `PUT` | `/feedback/{id}/status` | Обновить статус обращения |
| `PUT` | `/feedback/{id}/read` | Отметить обращение как прочитанное |

---

## 3. Схема базы данных

### ER-диаграмма (основные сущности)

```mermaid
erDiagram
    users {
        int id PK
        varchar username UK
        varchar password_hash
        varchar first_name
        varchar last_name
        varchar middle_name
        varchar position
        varchar email
        varchar phone
        int user_type_id FK
        int organization_id FK
        int company_id FK
        timestamp created_at
    }

    user_types {
        int id PK
        varchar name UK
        varchar description
    }

    organizations {
        int id PK
        varchar name
        varchar description
        timestamp created_at
    }

    companies {
        int id PK
        varchar name
        varchar description
        timestamp created_at
    }

    applications {
        int id PK
        int organization_id FK
        int company_id FK
        int sender_id FK
        int responsible_user_id FK
        varchar status
        varchar comment
        timestamp valid_from
        timestamp valid_to
        timestamp created_at
        timestamp updated_at
    }

    attachments {
        int id PK
        int application_id FK
        varchar status
        boolean is_active
        timestamp valid_from
        timestamp valid_to
        timestamp created_at
    }

    cars {
        int id PK
        int attachment_id FK
        varchar plate_number
        varchar brand
        varchar model
        boolean is_active
        boolean on_territory
        timestamp created_at
    }

    car_unload_places {
        int id PK
        int car_id FK
        int unload_place_id FK
    }

    employees {
        int id PK
        int attachment_id FK
        varchar first_name
        varchar last_name
        varchar middle_name
        int citizenship_id FK
        boolean is_active
        timestamp created_at
    }

    employee_target_tables {
        int id PK
        int employee_id FK
        int system_table_id FK
    }

    items {
        int id PK
        int attachment_id FK
        varchar name
        varchar description
        varchar status
    }

    unload_places {
        int id PK
        varchar name
        varchar description
        varchar address
    }

    unload_place_time_slots {
        int id PK
        int unload_place_id FK
        time start_time
        time end_time
    }

    unload_place_photos {
        int id PK
        int unload_place_id FK
        varchar file_path
        boolean is_main
    }

    system_tables {
        int id PK
        varchar name UK
        varchar description
    }

    system_table_time_slots {
        int id PK
        int system_table_id FK
        time start_time
        time end_time
    }

    system_table_photos {
        int id PK
        int system_table_id FK
        varchar file_path
        boolean is_main
    }

    unique_cars {
        int id PK
        varchar plate_number UK
        varchar brand
        varchar model
        varchar owner_info
    }

    unique_employees {
        int id PK
        varchar first_name
        varchar last_name
        varchar middle_name
        varchar owner_info
    }

    citizenships {
        int id PK
        varchar name UK
        boolean is_default
    }

    license_plate_formats {
        int id PK
        varchar name
        varchar pattern
    }

    license_plate_format_cells {
        int id PK
        int format_id FK
        int position
        varchar cell_type
    }

    application_approvers {
        int id PK
        int user_id FK
        timestamp created_at
    }

    application_viewers {
        int id PK
        int application_id FK
        int user_id FK
    }

    application_history {
        int id PK
        int application_id FK
        int user_id FK
        varchar action
        varchar comment
        timestamp created_at
    }

    car_history {
        int id PK
        int car_id FK
        varchar action
        varchar comment
        timestamp created_at
    }

    refresh_tokens {
        int id PK
        int user_id FK
        varchar token_hash
        timestamp expires_at
        boolean is_revoked
    }

    feedback {
        int id PK
        int user_id FK
        varchar subject
        text message
        varchar status
        boolean is_read
        timestamp created_at
    }

    users ||--o{ applications : "отправляет"
    users }o--|| user_types : "имеет тип"
    users }o--o| organizations : "принадлежит"
    users }o--o| companies : "принадлежит"
    users ||--o{ refresh_tokens : "имеет токены"
    users ||--o{ feedback : "создаёт"
    users ||--o{ application_history : "записывает"

    applications }o--|| organizations : "от организации"
    applications }o--o| companies : "от компании"
    applications ||--o{ attachments : "содержит"
    applications ||--o{ application_history : "имеет историю"
    applications ||--o{ application_viewers : "имеет наблюдателей"

    attachments ||--o{ cars : "содержит машины"
    attachments ||--o{ employees : "содержит сотрудников"
    attachments ||--o{ items : "содержит предметы"

    cars ||--o{ car_unload_places : "привязана к"
    car_unload_places }o--|| unload_places : "место разгрузки"
    cars ||--o{ car_history : "имеет историю"

    employees ||--o{ employee_target_tables : "привязан к"
    employee_target_tables }o--|| system_tables : "целевая таблица"
    employees }o--o| citizenships : "гражданство"

    unload_places ||--o{ unload_place_time_slots : "имеет слоты"
    unload_places ||--o{ unload_place_photos : "имеет фото"

    system_tables ||--o{ system_table_time_slots : "имеет слоты"
    system_tables ||--o{ system_table_photos : "имеет фото"

    license_plate_formats ||--o{ license_plate_format_cells : "состоит из ячеек"

    application_approvers }o--|| users : "пользователь-согласователь"
    application_viewers }o--|| users : "пользователь-наблюдатель"
```

### Связующие таблицы (many-to-many)

Помимо основных сущностей, в системе присутствуют промежуточные (junction) таблицы для связей многие-ко-многим:

| Связующая таблица | Связывает | Описание |
|---|---|---|
| `car_unload_places` | `cars` <-> `unload_places` | Привязка машин к местам разгрузки |
| `employee_target_tables` | `employees` <-> `system_tables` | Привязка сотрудников к целевым таблицам |
| `organization_unload_places` | `organizations` <-> `unload_places` | Доступные места разгрузки для организации |
| `organization_tables` | `organizations` <-> `system_tables` | Доступные таблицы для организации |
| `organization_users` | `organizations` <-> `users` | Пользователи организации |
| `company_unload_places` | `companies` <-> `unload_places` | Доступные места разгрузки для компании |
| `company_tables` | `companies` <-> `system_tables` | Доступные таблицы для компании |
| `company_users` | `companies` <-> `users` | Пользователи компании |
| `application_viewers` | `applications` <-> `users` | Наблюдатели заявок |

---

## 4. Бизнес-логика заявок

### Жизненный цикл заявки

```
                    ┌─────────────────┐
                    │   1. Создание   │
                    │   (draft)       │
                    └────────┬────────┘
                             │
                    POST /applications/
                    submit-complete-application
                             │
                             ▼
                    ┌─────────────────┐
                    │  2. Пересылка   │
                    │  ответственным  │
                    └────────┬────────┘
                             │
                    POST /applications/{id}/forward
                             │
                             ▼
                    ┌─────────────────┐
                    │ 3. Согласование │◄──── Каждый согласователь
                    │ (approval)      │      принимает или отклоняет
                    └────────┬────────┘
                             │
                    POST /applications/{id}/approve
                             │
                    ┌────────┴────────┐
                    │                 │
               Согласовано       Отклонено
                    │                 │
                    ▼                 ▼
           ┌───────────────┐  ┌──────────────┐
           │ 4. Взятие в   │  │  Возврат на  │
           │    работу      │  │  доработку   │
           └───────┬───────┘  └──────────────┘
                   │
          POST /applications/{id}/take-to-work
                   │
                   ▼
           ┌───────────────┐
           │  5. В работе  │
           └───────┬───────┘
                   │
          ┌────────┴────────┐
          │                 │
    revoke-from-work   restore-to-work
          │                 │
          ▼                 ▼
   ┌─────────────┐  ┌─────────────┐
   │  Отозвана   │  │ Восстановле-│
   │  из работы  │──│ на в работу │
   └─────────────┘  └─────────────┘
```

### Описание этапов

1. **Создание заявки** (`POST /applications/submit-complete-application`) -- пользователь формирует полную заявку с вложениями (бланками), в каждом из которых могут быть машины, сотрудники и предметы. Заявка привязывается к организации и/или компании отправителя.

2. **Пересылка ответственным** (`POST /applications/{id}/forward`) -- заявка направляется ответственным пользователям для рассмотрения. Назначаются наблюдатели.

3. **Согласование** (`POST /applications/{id}/approve`) -- каждый назначенный согласователь независимо принимает решение: согласовать или отклонить. Статус проверяется через `GET /applications/{id}/check-approval-status`. Согласование можно отозвать (`POST /applications/{id}/revoke-approval`).

4. **Взятие в работу** (`POST /applications/{id}/take-to-work`) -- после согласования ответственный сотрудник берёт заявку в работу. Это активирует пропуска для машин и сотрудников.

5. **Отзыв и восстановление** -- заявку можно отозвать из работы (`revoke-from-work`) и вернуть обратно (`restore-to-work`). При отзыве пропуска деактивируются.

### Фоновые задачи

Сервер запускает фоновую задачу (Tokio task), которая выполняется **каждые 60 секунд**:

- **Автоматическая деактивация истёкших вложений** -- проверяет поле `valid_to` у каждого активного вложения. Если срок действия истёк, вложение и все связанные сущности (машины, сотрудники) деактивируются автоматически.

---

## 5. Аутентификация и авторизация

### JWT-токены

Система использует пару токенов для аутентификации:

| Параметр | Access Token | Refresh Token |
|---|---|---|
| Время жизни | 120 минут (2 часа) | 24 часа |
| Секрет (env) | `JWT_SECRET` | `JWT_REFRESH_SECRET` |
| Содержимое (claims) | user_id, username, user_type | user_id, username |
| Хранение | Клиент (localStorage / cookie) | Клиент + SHA-256 хеш в БД |

### Процесс аутентификации

```
Клиент                          Сервер                         БД
  │                               │                             │
  │── POST /login ───────────────►│                             │
  │   {username, password}        │── проверка Argon2 ─────────►│
  │                               │◄── пользователь найден ─────│
  │                               │                             │
  │                               │── генерация токенов          │
  │                               │── SHA-256(refresh_token) ──►│
  │                               │   сохранение в БД            │
  │◄── {access_token,            │                             │
  │     refresh_token} ──────────│                             │
  │                               │                             │
  │── GET /user-data ────────────►│                             │
  │   Authorization: Bearer {at}  │── проверка JWT              │
  │◄── {user_data} ──────────────│                             │
  │                               │                             │
  │── POST /refresh-token ───────►│                             │
  │   {refresh_token}             │── SHA-256(rt) ─────────────►│
  │                               │◄── токен валиден? ──────────│
  │                               │── отзыв старого rt ────────►│
  │                               │── генерация новой пары       │
  │◄── {access_token,            │── сохранение нового rt ────►│
  │     refresh_token} ──────────│                             │
  │                               │                             │
  │── POST /logout ──────────────►│                             │
  │   {refresh_token}             │── отзыв rt ────────────────►│
  │◄── 200 OK ───────────────────│                             │
```

### Хеширование паролей

- **Алгоритм**: Argon2id (рекомендованный OWASP)
- **Соль**: генерируется случайно для каждого пароля
- **Хранение**: только хеш, исходный пароль не сохраняется

### Rate Limiting

- **Лимит**: 10 запросов за 60 секунд
- **Идентификация**: по IP-адресу или JWT-токену (если авторизован)
- **Хранение**: in-memory `DashMap` (не сохраняется между перезапусками)
- **Ответ при превышении**: HTTP 429 Too Many Requests

---

## 6. Настройка и запуск

### Предварительные требования

- **Rust** (stable, edition 2021) -- [rustup.rs](https://rustup.rs)
- **PostgreSQL** 14+ -- запущенный экземпляр с созданной базой данных
- **sqlx-cli** (опционально) -- для работы с миграциями: `cargo install sqlx-cli`

### Запуск

```bash
# Перейти в директорию бэкенда
cd backend

# Скопировать пример конфигурации
cp .env.example .env

# Настроить переменные окружения (см. таблицу ниже)
nano .env

# Запустить сервер в режиме разработки
cargo run

# Или собрать релизную версию
cargo build --release
./target/release/backend
```

### Переменные окружения

| Переменная | Описание | Значение по умолчанию | Обязательная |
|---|---|---|---|
| `DATABASE_URL` | Строка подключения к PostgreSQL | -- | Да |
| `BIND_HOST` | Хост для привязки HTTP-сервера | `127.0.0.1` | Нет |
| `BIND_PORT` | Порт HTTP-сервера | `8080` | Нет |
| `JWT_SECRET` | Секретный ключ для access token | Хардкод в auth.rs (небезопасно) | Рекомендуется |
| `JWT_REFRESH_SECRET` | Секретный ключ для refresh token | Хардкод в auth.rs (небезопасно) | Рекомендуется |
| `RUST_LOG` | Уровень логирования | `info` | Нет |

Пример `DATABASE_URL`:
```
DATABASE_URL=postgres://username:password@localhost:5432/bureau_db
```

---

## 7. Известные проблемы и рекомендации

### Критические проблемы

| Проблема | Серьёзность | Рекомендация |
|---|---|---|
| Хардкод JWT-секретов в `auth.rs` | **Высокая** | Перенести в переменные окружения через `.env`. Текущие значения по умолчанию используются, если env-переменные не заданы. В продакшене это недопустимо. |
| CORS разрешает любой origin с credentials | **Высокая** | Ограничить список разрешённых origins конкретными доменами фронтенда. Текущая конфигурация позволяет любому сайту делать авторизованные запросы. |
| `#![allow(warnings)]` подавляет все предупреждения | **Средняя** | Убрать директиву и исправить предупреждения компилятора. Они могут указывать на реальные ошибки. |

### Архитектурные проблемы

| Проблема | Серьёзность | Рекомендация |
|---|---|---|
| `applications.rs` -- 3800+ строк | **Средняя** | Разделить на подмодули: `applications/create.rs`, `applications/approve.rs`, `applications/history.rs` и т.д. |
| Нет миграций базы данных | **Средняя** | Внедрить `sqlx migrate` или аналогичный инструмент. Текущая схема, вероятно, управляется вручную. |
| Нет автоматических тестов | **Средняя** | Добавить как минимум интеграционные тесты для критических путей: авторизация, создание заявки, согласование. |
| Rate limiter in-memory | **Низкая** | Для кластерной конфигурации (несколько экземпляров) потребуется Redis или аналог. Для одного экземпляра текущее решение допустимо. |

### Рекомендации по развитию

1. **Валидация входных данных** -- добавить строгую валидацию на уровне моделей (длина строк, форматы, обязательные поля).
2. **Пагинация** -- убедиться, что все эндпоинты, возвращающие списки, поддерживают пагинацию с лимитами.
3. **Логирование** -- расширить структурированное логирование (tracing crate) для отладки в продакшене.
4. **Документация API** -- подключить автогенерацию OpenAPI/Swagger через `utoipa` или `paperclip`.
5. **Health check** -- добавить эндпоинт `/health` для мониторинга и балансировщиков нагрузки.
6. **Graceful shutdown** -- реализовать корректное завершение фоновых задач при остановке сервера.

---

*Документация актуальна на 28 марта 2026 года.*
