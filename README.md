# Systemburo — Система управления пропусками

Веб-приложение для управления пропусками на территорию: заявки, сотрудники, автомобили, согласование.

## Стек

| Слой | Технологии |
|------|------------|
| Backend | Go (go.mod: 1.26.10; Docker: 1.27.3), Echo v4, GORM, PostgreSQL 16 |
| Frontend | Vue 3, Vue Router, Pinia |
| Инфраструктура | Docker Compose, nginx, GitHub Actions CI/CD |

## Быстрый старт (dev)

```bash
git clone <repo> && cd systemburo
cp .env.example .env
# Заполнить .env: отдельные JWT-секреты, ключ шифрования, параметры локального HTTP
make init   # git hooks
make up     # запустить все сервисы
make seed   # создать тестового админа (buropropuskov / admin123)
```

| Сервис | URL |
|--------|-----|
| Backend | http://localhost:8080/health |
| Frontend | http://localhost:8081 |
| pgAdmin | http://localhost:8082 |
| Swagger | http://localhost:8080/swagger/index.html |

### Тестовый админ

После `make seed` доступны креды:

| Поле | Значение |
|------|----------|
| Логин | `buropropuskov` |
| Пароль | `admin123` |

Кастомный пароль: `make seed PASS=mypassword`.

## Staging / Production

```bash
make staging-up     # поднять staging (docker-compose.staging.yml)
make staging-seed   # создать админа на staging

make deploy-up      # поднять production (docker-compose.prod.yml)
make deploy-seed    # создать админа на production
```

Для staging/prod сидер собирается в образ как отдельный бинарь `/app/seed` (см. `Dockerfile`, stage `production`).

## Команды

| Команда | Описание |
|---------|----------|
| `make up / down` | Запустить / остановить сервисы |
| `make test` | Go тесты |
| `make lint` | Go vet |
| `make bash` | Shell в контейнере бэкенда |
| `make db-shell` | psql к БД |
| `make seed [PASS=...]` | Создать тестового админа (dev) |
| `make seed-demo [PASS=...]` | Создать админа + полный демо-набор (12 заявок в разных статусах, организации, машины, сотрудники, история, объявления) |
| `make staging-seed [PASS=...]` | Создать админа на staging |
| `make staging-seed-demo [PASS=...]` | Демо-набор на staging |
| `make deploy-seed [PASS=...]` | Создать админа на production |
| `make prod-build` | Собрать production образ |
| `make security` | govulncheck + npm audit |

## Структура

```
├── cmd/server/           # Точка входа
├── internal/             # Go-бэкенд (handlers, services, models, middleware, crypto)
├── frontend/             # Vue 3 SPA (components, stores, api, views)
├── nginx/                # Production nginx
├── .github/workflows/    # CI/CD
├── docker-compose.yml    # Dev
└── docker-compose.prod.yml
```

## Документация

| Документ | Описание |
|----------|----------|
| [Backend](docs/BACKEND.md) | Архитектура Go, middleware, модели, сервисы, шифрование |
| [Frontend](docs/FRONTEND.md) | Архитектура Vue, stores, API-клиент, компоненты, skeleton |
| [Deployment](docs/DEPLOYMENT.md) | Docker, ручной деплой, production, бэкапы |
| [Security](docs/SECURITY.md) | Технические меры: шифрование, аудит, сессии, права и ограничения |
| [Access Control](docs/access-control/) | Система прав: роли, группы, журнал отказов, ban, миграция type_id=6 |
| [ADR](docs/adr/) | Архитектурные решения (Go vs Rust, GORM, AES-GCM, Pinia и др.) |
| [Contributing](CONTRIBUTING.md) | Рабочий процесс: ветки, коммиты, PR, issues, CI/CD |

## Конфигурация

Все настройки через переменные окружения; валидируемые значения и умолчания — `internal/config/config.go`, стартовый пример — `.env.example`. Шифрование требуется по умолчанию. Перед первым входом по HTTP на localhost проверить `COOKIE_SECURE=false`; на staging/production оставить true.

Сборка клиентских руководств и сверка фактов: [инструкция](Документация/src/README.md). Исходники Markdown находятся рядом с документами; PDF/DOCX принимаются после сверки и визуальной проверки, а не по одному наличию файла.

## Лицензия

Проприетарный проект.
