# Безопасность и 152-ФЗ

Документ описывает меры защиты персональных данных и общую безопасность приложения.

## Шифрование персональных данных (at rest)

**Алгоритм:** AES-256-GCM (аутентифицированное шифрование).

**Что шифруется:**
- Серия и номер паспорта (`passport_series_number`)
- Номер патента (`patent_number`)
- На моделях: Employee, UniqueEmployee, ApplicationEmployee

**Механизм:** GORM-хуки `BeforeSave` / `AfterFind` — шифрование/расшифровка прозрачна для бизнес-логики. Ключ задаётся через `DATA_ENCRYPTION_KEY` (hex, 64 символа = 32 байта).

**Поиск по зашифрованным полям:** HMAC-SHA256 — для каждого зашифрованного поля хранится детерминированный хеш в отдельной колонке с индексом (`passport_series_number_hmac`). WHERE-условия используют HMAC вместо plaintext.

**Passthrough-режим:** При пустом `DATA_ENCRYPTION_KEY` шифрование отключено (для разработки).

## Аудит доступа к ПД

**Middleware:** `PDAudit` — логирует все HTTP-запросы к эндпоинтам с персональными данными.

**Покрытие:** `/employees`, `/unique-employees`, `/attachments`

**Записываемые данные:**
- Имя пользователя
- Действие (view / create / update / delete — по HTTP-методу)
- Ресурс (employee / unique_employee / attachment)
- IP-адрес
- HTTP-метод, путь, статус-код
- Временная метка

**Хранение:** Таблица `pd_audit_logs`, async-запись в горутине с логированием ошибок.

## Согласия на обработку ПД

**API:** `/consents`
- `POST /consents` — дать согласие (тип: `pd_processing`, `pd_transfer`)
- `DELETE /consents/:type` — отозвать согласие
- `GET /consents` — список согласий пользователя
- `GET /consents/check/:type` — проверка активного согласия

**Модель:** `PDConsent` — user_id, consent_type, granted, ip_address, user_agent, granted_at, revoked_at.

## Аутентификация

**JWT access-токен:** 120 минут, содержит username, user_id, type_id.

**Refresh-токен:** 24 часа, хранится в БД как SHA-256 хеш. One-time use — после использования помечается `is_revoked`, выдаётся новая пара.

**Пароли:** Argon2id (m=19456, t=2, p=1) — совместимость с PHC-форматом.

**JWT-секреты:** Обязательны (`required`), минимум 32 символа, дефолтных значений нет.

## Авторизация

**Типы пользователей:** 6 типов, `type_id=6` (buropropuskov) = админ.

**IDOR-защита:** `CanAccessApplication(ctx, id, username, typeID)` проверяет на всех 15 эндпоинтах заявок, что пользователь является:
- Отправителем заявки
- Ответственным пользователем
- Просматривающим
- Админом (type_id=6)

**Permission-система:** Таблица `user_permissions` (key → allow/deny). Фронтенд: директива `v-permission`, бэкенд: middleware `RequirePermission`.

**Валидация входных данных:**
- `BindAndValidate` — generic-ошибки без утечки внутренних деталей
- Enum-whitelist для Status, Confirmation, consent type
- File upload: magic bytes + MimeToExt (расширение из MIME, не из имени файла)

## Rate limiting

**Механизм:** In-memory, per IP (без токена) или per token (с токеном).

**Настройки:** `RATE_LIMIT_PER_MINUTE` (default 200), `RATE_LIMIT_WINDOW_SEC` (default 60).

**Cleanup:** Фоновая горутина каждые 60 секунд удаляет устаревшие записи.

## Security headers (nginx)

Production nginx (`nginx/nginx.conf`):

```
X-Frame-Options: SAMEORIGIN
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; ...
```

Frontend nginx (`frontend/nginx.conf`): аналогичные headers без CSP.

## CORS

Настраивается через `CORS_ALLOWED_ORIGINS` (default: `http://localhost:8081`).
`AllowCredentials: true`, методы: GET, POST, PUT, DELETE, OPTIONS, PATCH.
Headers whitelist: Authorization, Content-Type, X-Request-ID, Accept.

## CI/CD Security

GitHub Actions workflows:
- **govulncheck** — сканирование Go-зависимостей на известные CVE
- **npm audit** — сканирование frontend-зависимостей
- **trivy** — сканирование Docker-образов
- Запуск: при push в main, при PR, еженедельно (понедельник)

## Swagger UI

Доступен только при `LOG_LEVEL=debug`. В production отключён.

## pgAdmin

Включён только в `docker-compose.yml` (dev). В `docker-compose.prod.yml` отсутствует. На production pgAdmin не должен быть доступен — это прямой доступ к БД с web-интерфейсом.

## Обработка ошибок

Все внутренние ошибки (GORM, PostgreSQL) логируются через `slog.Error()` и не передаются клиенту. Клиент получает generic-сообщения: "Invalid request body", "Operation failed", "Internal server error".

## Известные ограничения

| Ограничение | Описание |
|-------------|----------|
| v-html без DOMPurify | TablesComponent и TextConstructor используют regex-санитайзер, который обходится. Нужен `npm install dompurify` |
| JWT type_id persistence | После демоции пользователя старый токен с админ-правами действует до 120 мин |
| Нет account lockout | Нет блокировки после N неудачных логинов |
| Нет CAPTCHA на регистрации | Открытая регистрация без anti-bot защиты |
