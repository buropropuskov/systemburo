# Systemburo — Система управления пропусками

Веб-приложение для управления пропусками на территорию: заявки, сотрудники, автомобили, согласование.

## Стек технологий

- **Backend:** Go 1.25 (Echo v4, GORM, PostgreSQL 16)
- **Frontend:** Vue 3, Vue Router, Pinia, CSS custom properties
- **Infrastructure:** Docker Compose, nginx, GitHub Actions CI/CD
- **Testing:** Go testify, Vitest (unit), Playwright (E2E)

## Быстрый старт

```bash
# 1. Клонировать и настроить
git clone <repo>
cd systemburo
cp .env.example .env
make init  # настроить git hooks

# 2. Запустить
make up

# 3. Проверить
# Backend:  http://localhost:8080/health
# Frontend: http://localhost:8081
# pgAdmin:  http://localhost:8082
# Swagger:  http://localhost:8080/swagger/index.html
```

## Команды

| Команда | Описание |
|---------|----------|
| `make up` | Запустить все сервисы |
| `make down` | Остановить все сервисы |
| `make test` | Запустить Go тесты |
| `make lint` | Go vet |
| `make bash` | Shell в контейнере бэкенда |
| `make db-shell` | psql к базе данных |
| `make prod-build` | Собрать production образ |
| `make security` | govulncheck + npm audit |

## Структура проекта

```
├── cmd/server/          # Точка входа Go-приложения
├── internal/
│   ├── api/             # Пагинация, общие API-утилиты
│   ├── config/          # Конфигурация из env
│   ├── crypto/          # AES-256-GCM шифрование (152-ФЗ)
│   ├── database/        # Миграции и seed
│   ├── handlers/        # HTTP-хендлеры (Echo)
│   ├── middleware/       # JWT, CORS, rate limit, PD audit
│   ├── models/          # GORM-модели
│   ├── router/          # Маршрутизация
│   ├── services/        # Бизнес-логика
│   ├── upload/          # Валидация загрузки файлов
│   └── validator/       # Валидатор запросов
├── frontend/            # Vue 3 SPA
│   ├── src/api/         # API-клиент
│   ├── src/components/  # Vue-компоненты
│   ├── src/composables/ # Composables (валидация, toast, etc.)
│   ├── src/stores/      # Pinia stores (auth, ui, permissions)
│   └── src/views/       # Страницы
├── nginx/               # Production nginx конфиг
├── .github/workflows/   # CI/CD (tests, security, deploy)
├── docker-compose.yml   # Dev-окружение
└── docker-compose.prod.yml  # Production
```

## Конфигурация

Все настройки через переменные окружения. См. `.env.example`.

Ключевые секции:
- **База данных:** DATABASE_URL, DB_NAME, DB_USER, DB_PASSWORD
- **JWT:** JWT_SECRET, JWT_REFRESH_SECRET (минимум 32 символа)
- **CORS:** CORS_ALLOWED_ORIGINS
- **Шифрование ПД (152-ФЗ):** DATA_ENCRYPTION_KEY (hex, 64 символа)
- **Upload:** UPLOAD_MAX_FILE_SIZE, UPLOAD_ALLOWED_IMAGE_TYPES
- **Rate limit:** RATE_LIMIT_PER_MINUTE, RATE_LIMIT_WINDOW_SEC

## API

Swagger UI доступен по адресу `/swagger/index.html` в dev-режиме.

## Безопасность

- Шифрование паспортных данных at rest (AES-256-GCM + HMAC-SHA256)
- JWT аутентификация с refresh tokens
- Rate limiting по IP/токену
- Аудит-лог доступа к персональным данным
- Модель согласия на обработку ПД (152-ФЗ)
- Security headers в nginx (CSP, HSTS, X-Frame-Options)
- Сканирование уязвимостей в CI (govulncheck, npm audit, trivy)

## Деплой

См. [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) для полной инструкции.

Краткий вариант:
```bash
make deploy-build  # собрать production-образы
make deploy-up     # запустить
```

## Лицензия

Проприетарный проект.
