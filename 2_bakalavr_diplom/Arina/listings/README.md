# Листинги для главы 3

В каталоге собраны исходные файлы всех листингов, процитированных в главе 3 ВКР. В тексте диссертации приведены сокращённые фрагменты (10–25 строк), полные версии — здесь.

| Листинг | Имя файла | Описание | Связь с кодом проекта |
|---|---|---|---|
| 3.1 | `listing_3.1_vite.config.js` | Конфигурация Vitest с псевдонимами путей | Расширение существующего `vite.config.js` |
| 3.2 | `listing_3.2_DropMenu.spec.js` | Модульный тест компонента DropMenu.vue | Основан на `src/components/DropMenu.vue` |
| 3.3 | `listing_3.3_auth_handler_test.go` | Тест обработчика /auth/sign-up | Расширяет `internal/handler/auth_test.go` |
| 3.4 | `listing_3.4_postman_snippet.json` | Фрагмент Postman-коллекции — изоляция пользователей | Планируемый маршрут `/api/v1/documents/:id` |
| 3.5 | `listing_3.5_document_analysis.spec.js` | Playwright-сценарий загрузки и анализа | Планируемая интеграция фронт+бэк |
| 3.6 | `listing_3.6_ci.yml` | Полный CI/CD-пайплайн GitHub Actions | Новый файл `.github/workflows/ci.yml` |

## Соответствие проекту

Все листинги написаны в соответствии с технологическим стеком из репозиториев:

- [github.com/airvt1x/dokkee-backend](https://github.com/airvt1x/dokkee-backend) — Go 1.25, Gin, sqlx, testify, go-sqlmock, jwt-go
- [github.com/keeq0/dokkee-frontend](https://github.com/keeq0/dokkee-frontend) — Vue 3 (Options API), axios, pdfjs-dist, openai, Vue CLI

Для модулей, не реализованных на момент написания диплома (загрузка документов, анализ, заметки, аналитика), в коде листингов стоит пометка `//возможны изменения//`.
