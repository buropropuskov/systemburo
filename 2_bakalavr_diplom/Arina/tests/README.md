# tests/ — тестовый комплект сервиса Dokkee

Комплект тестов для клиентской и серверной частей, подготовленный в рамках ВКР «Тестирование стартапа Dokkee — веб-сервиса автоматизированного анализа документов».

## Структура

```
tests/
├── backend/
│   ├── unit/             — модульные тесты на Go (testify + go-sqlmock)
│   │   ├── auth_handler_test.go       ← расширенные тесты /auth/* (реальный код)
│   │   ├── auth_service_test.go       ← тесты сервиса (хэширование, JWT)
│   │   ├── auth_repository_test.go    ← тесты репозитория (sqlmock)
│   │   └── documents_handler_test.go  ← планируемый CRUD документов //возможны изменения//
│   ├── integration/      — интеграционные тесты с реальной PostgreSQL
│   └── mocks/            — сгенерированные моки (testify/mock)
│
├── frontend/
│   └── unit/             — модульные тесты компонентов Vue 3 (Vitest)
│       ├── DropMenu.spec.js           ← drag-and-drop область (реальный код)
│       └── UploadDocuments.spec.js    ← форма загрузки файлов (реальный код)
│
├── api/
│   └── dokkee.postman_collection.json ← коллекция для Postman/Newman (34 сценария)
│
├── e2e/
│   ├── auth.spec.js                   ← сквозной сценарий авторизации
│   ├── document-analysis.spec.js      ← загрузка + анализ + отчёт
│   └── playwright.config.js           ← конфиг Playwright
│
└── ci/
    └── ci.yml                         ← GitHub Actions: 6 jobs + staging deploy
```

## Соответствие коду проекта

Тесты написаны на основе следующих репозиториев:

- Бэкенд: <https://github.com/airvt1x/dokkee-backend>
- Фронтенд: <https://github.com/keeq0/dokkee-frontend>

Маркировка `//возможны изменения//` внутри файлов указывает на разделы, написанные по плану тестирования для ещё не реализованных модулей (загрузка документов, анализ, заметки, аналитика). После реализации соответствующего функционала тесты нужно актуализировать по селекторам и контрактам API.

## Технологический стек тестов

| Уровень | Инструмент | Причина выбора |
|---|---|---|
| Модульные (бэкенд) | Go `testing` + `testify` + `go-sqlmock` | Совпадает со стеком проекта (go.mod), даёт моки с читаемым синтаксисом |
| Модульные (фронтенд) | Vitest + `@vue/test-utils` + jsdom | Нативная поддержка Vue 3, быстрый параллельный прогон |
| API-тесты | Postman + Newman | Коллекции декларативные, одинаково работают в IDE и CI |
| E2E | Playwright | Встроенные трассы и скриншоты, два браузера в одном прогоне |
| CI/CD | GitHub Actions | Бесплатно для open-source, нативно для GitHub-репозиториев |
| Безопасность | OWASP ZAP Baseline | Автоматический baseline-скан в отдельной job |

## Команды

```bash
# Backend unit
go test ./tests/backend/unit/... -race -cover

# Backend integration (требует docker compose up postgres)
go test -tags=integration ./tests/backend/integration/...

# Frontend unit
npm run test:unit --prefix ../dokkee-frontend -- tests/frontend/unit

# API-коллекция
newman run tests/api/dokkee.postman_collection.json \
  --env-var baseUrl=http://localhost:8000

# E2E
cd tests/e2e
npm install
npx playwright install
DOKKEE_URL=http://localhost:8080 API_URL=http://localhost:8000 \
  npx playwright test
```

## Планируемое покрытие

| Слой | Метрика | Цель |
|---|---|---|
| Backend | Покрытие строк | ≥ 80 % |
| Backend | Покрытие ветвлений (handlers) | ≥ 75 % |
| Frontend | Покрытие строк компонентов | ≥ 70 % |
| API | Сценарии Postman | 34 (6 auth + 9 docs + 7 analysis + 5 reports + 7 negative) |
| E2E | Критические пользовательские сценарии | 4 (auth, upload+analyze, export, error handling) |
