# Backend — Архитектура

Go 1.26, Echo v4, GORM, PostgreSQL 16.

## Запуск приложения

Порядок инициализации в `cmd/server/main.go`:

1. Загрузка конфигурации (`config.Load()` из env)
2. Настройка slog по `LOG_LEVEL`
3. Инициализация шифрования (`crypto.SetGlobalKey`)
4. Подключение к PostgreSQL через GORM
5. AutoMigrate (44+ таблиц, идемпотентно)
6. Seed (6 UserTypes + 4 Permission по умолчанию)
7. Создание Echo + глобальные middleware
8. Инстанцирование 18 сервисов → 21 хендлер
9. Регистрация маршрутов (22 группы + публичные)
10. Swagger UI (только при `LOG_LEVEL=debug`)
11. Graceful shutdown (SIGINT/SIGTERM, 10с таймаут)

## Структура

```
internal/
├── api/           # Пагинация (SetMaxLimit, ApplyPagination)
├── config/        # Конфиг из env (caarlos0/env), валидация
├── crypto/        # AES-256-GCM + HMAC-SHA256 для 152-ФЗ
├── database/      # AutoMigrate + Seed
├── handlers/      # HTTP-хендлеры (21 файл)
├── middleware/     # JWT, CORS, RateLimit, PDAudit, RequestID
├── models/        # GORM-модели (25+ типов)
├── router/        # Маршрутизация (router.Setup)
├── services/      # Бизнес-логика (18 сервисов)
├── upload/        # Валидация файлов (magic bytes + MimeToExt)
└── validator/     # Валидатор запросов (echo.Validator)
```

## Middleware pipeline

Порядок для всех запросов:

```
RequestID → Logger → Recover → CORS → RateLimit → PDAudit → [JWTAuth для защищённых]
```

| Middleware | Файл | Назначение |
|-----------|------|------------|
| RequestID | `request_id.go` | X-Request-ID для трейсинга |
| CORS | `cors.go` | Настраиваемые origins, credentials |
| RateLimit | `ratelimit.go` | Лимит по IP/токену, cleanup каждые 60с |
| PDAudit | `pd_audit.go` | Аудит-лог доступа к ПД (async, /employees, /unique-employees, /attachments) |
| JWTAuth | `jwt.go` | Валидация Bearer-токена, username/user_id/type_id в контекст |
| RequirePermission | `permission.go` | Проверка прав (опционально на отдельных маршрутах) |

## Маршруты

**Публичные:** `/register`, `/login`, `/refresh-token`, `/user-types`, `/health`

**Защищённые (JWT):** 22 группы — applications (24 маршрута), cars (18), organizations (15), companies (12), users, employees, system-tables, unload-places, unique-cars, unique-employees, attachments, feedback, permissions, consents, settings и др.

Полный список — Swagger UI (`/swagger/index.html` в debug-режиме).

## Валидация входа (boundary)

Хендлер — граница приложения: тут данные из HTTP превращаются в типизированный вход.
Чтобы разрозненный разбор не расползался, есть единая точка `handlers.BindAndValidate(c, &dto)`
(`internal/handlers/helpers.go`): bind + валидация + `400` при ошибке.

**Правило для нового кода:** хендлер, читающий ТЕЛО запроса (POST/PUT/PATCH), использует
`BindAndValidate` с DTO, поля которого размечены `validate`-тегами (`required`, `gte`, `oneof` и т.п.).
Не плодить ad-hoc `c.Bind` + ручные проверки в каждом месте.

**Когда голый `c.Bind` уместен** (с комментарием почему — иначе на ревью читается как пропущенная валидация):

| Случай | Почему без валидации |
|--------|----------------------|
| Query-параметры фильтров/пагинации (GET) | Поля опциональны, валидировать нечего |
| Намеренно опциональное тело | Пустое = валидный кейс (напр. `user_ban`: бан без причины) |
| Бинд в слайс `[]T` | Echo-валидатор работает по структурам, не слайсам |
| Условная/доменная валидация | Обязательность зависит от другого поля (напр. `statistics.RunReport` — от `mode`); остаётся в хендлере/сервисе, но через явный `400` |

Это конвенция, не догма: индивидуальные случаи допустимы, но фиксируются комментарием у места.
Ретроактивно навешивать `required` на существующие эндпоинты — осторожно: поле, формально
обязательное по Swagger, может реально приходить пустым из живого фронта (напр. марка машины
по «по факту»), и новый `required` обернётся `400`-регрессией. Менять валидацию старого эндпоинта —
сверять с реальными вызовами FE.

## Модели

Ключевые сущности и связи:

| Модель | Связи | GORM-хуки |
|--------|-------|-----------|
| User | belongs-to Organization, Company, UserType | — |
| Application | has-many History, ResponsibleUsers, Approvers, Viewers, Reads | — |
| Employee | belongs-to Attachment, Citizenship | BeforeSave: шифрование + HMAC; AfterFind: расшифровка |
| UniqueEmployee | belongs-to Organization, Company | BeforeSave/AfterFind: шифрование |
| Car | belongs-to Attachment; has-many CarHistory, CarUnloadPlaces | — |
| Attachment | has-many Cars, Employees, Items | — |
| SystemTable | has-many Photos, TimeSlots, Fields | — |
| PDConsent | belongs-to User | — |
| PDAuditLog | — | Async-запись через middleware |

## Сервисы

| Сервис | Ключевые методы |
|--------|----------------|
| AuthService | Register, Login, RefreshToken, Logout |
| ApplicationService | GetApplications, Create, Update, Forward, Approve, TakeToWork, CanAccessApplication |
| UserService | GetAll, UpdateType, UpdatePassword, Delete |
| SettingsService | GetAll, Update, GetUploadSettings (in-memory cache) |
| ConsentService | Grant, Revoke, List, HasActive |
| PermissionService | GetMyPermissions, UpdateUserPermissions, HasPermission |
| SystemTableService | CRUD + TimeSlots + Photos (upload с валидацией) |
| CarService | CRUD + History + TerritoryStatus |
| EmployeeService | Create, GetActiveForTable |
| FeedbackService | Create, GetAll, UpdateStatus |

## Шифрование (152-ФЗ)

Модуль `internal/crypto/`:

| Функция | Назначение |
|---------|------------|
| `SetGlobalKey(key)` | Установить 32-байт ключ при старте (nil = passthrough) |
| `Encrypt / Decrypt` | AES-256-GCM, base64(nonce+ciphertext) |
| `ComputeHMAC` | HMAC-SHA256 для детерминированного поиска |
| `EncryptOptional / DecryptOptional` | Обёртки для GORM-хуков (nil-safe) |

Шифруются: `passport_series_number`, `patent_number` у Employee/UniqueEmployee/ApplicationEmployee.
Поиск по зашифрованным полям — через HMAC-колонки с индексами.

## Конфигурация

Все настройки через env. Ключевые:

| Переменная | Обязательна | Default |
|------------|-------------|---------|
| `DATABASE_URL` | Да | — |
| `JWT_SECRET` | Да | — |
| `JWT_REFRESH_SECRET` | Да | — |
| `BIND_PORT` | Нет | 8090 |
| `LOG_LEVEL` | Нет | info |
| `CORS_ALLOWED_ORIGINS` | Нет | http://localhost:8081 |
| `DATA_ENCRYPTION_KEY` | Нет | "" (шифрование выключено) |
| `RATE_LIMIT_PER_MINUTE` | Нет | 200 |
| `UPLOAD_MAX_FILE_SIZE` | Нет | 10MB |

Полный список — `.env.example`.

## Авторизация

- JWT access-токен (120 мин) + refresh-токен (24ч)
- `CanAccessApplication(ctx, id, username, typeID)` — проверка доступа к заявке (отправитель, ответственный, просматривающий, админ)
- Админ-проверки через `type_id == 6` из JWT
- Enum-валидация Status/Confirmation при обновлении заявок

## База данных

- GORM AutoMigrate при старте (идемпотентно, 44+ таблиц)
- Совместимость со схемой Rust-бэкенда (детектирует существующие таблицы)
- Seed: 6 типов пользователей, 4 дефолтных permission
