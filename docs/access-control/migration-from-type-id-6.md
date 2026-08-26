# Миграция с `type_id = 6` на новую систему прав

Документирует переход хардкода `user_type_id = 6` ("buropropuskov") на новую систему прав (#187), реализованный в PR #229–#239.

## Что было

Супер-админ идентифицировался по магическому числу:

```go
if c.Get("type_id").(int) == 6 { /* всё разрешено */ }
```

Жёсткая привязка к 11 местам в коде:

- `internal/middleware/maintenance.go`
- `internal/handlers/maintenance.go`
- `internal/handlers/auth.go`
- `internal/services/permission_service.go`
- `internal/services/application_helpers.go`
- `internal/services/settings_service.go`
- `frontend/src/stores/auth.js`
- `frontend/src/router.js`
- `frontend/src/stores/permissions.js`
- `cmd/seed/main.go`
- `internal/services/maintenance_service.go`

## Что стало

`users.is_super_admin BOOLEAN` — явный признак, не зависит от `user_types.code`.

Все 11 проверок переписаны на:

```go
// backend
if handlers.IsSuperAdmin(c) { /* bypass */ }
```

```js
// frontend
if (auth.isSuperAdmin) { /* bypass */ }
```

JWT Claims расширены полем `is_super_admin` — middleware кладёт его в context без дополнительного запроса в БД.

## Data migration

`internal/database/migrate.go::EnforceSingleSuperAdmin`:

Поддерживает инвариант «ровно один супер-админ». Канонический супер-админ —
аккаунт `username='buropropuskov'` (иначе самый ранний существующий супер-админ,
иначе первый пользователь). Канонику флаг гарантируется, у всех остальных
снимается, а сами они становятся обычными администраторами (`is_admin=true`),
чтобы не потерять доступ. Имя пустого системного аккаунта нормализуется в
«Системный Администратор».

Запускается в `AutoMigrate()` при старте сервера. Идемпотентна (повторный запуск — noop).

Прежняя версия (`backfillSuperAdmin`) делала супером всех пользователей типа
`buropropuskov` — при нескольких buro-аккаунтах появлялось несколько супер-админов.

## Что осталось от `user_types`

Колонка `users.type_id` сохранена, но больше не используется для определения super-admin. Вместо этого она несёт **бизнес-семантику** ролей в старой логике (арендатор, охранник, руководитель). В рамках новой системы прав появилась модель `Role` (#229) с теми же сущностями — миграция бизнес-ролей с `user_types` на `roles` возможна как отдельная задача (вне scope #187).

`user_types.code = 'buropropuskov'` теперь — обычный тип без специального handling. В seed создаётся для совместимости с тестами.

## Backward compatibility

- API-контракт `/login` и `/users/me` расширен полями `is_super_admin` и `is_banned`. Старые поля `type_id`, `user_type` сохранены — клиенты не сломаются.
- `auth.isAdmin` геттер во фронте помечен `@deprecated` и делегирует на `isSuperAdmin` — старый код продолжает работать.
- ~~Legacy функции `auth.CheckAdminByTypeID` и `auth.CheckBuroByUsername`~~ удалены в Ф5 (пакет `internal/auth/` целиком) — были мёртвым кодом без вызовов.

## Что можно удалить позже

Если убедимся, что миграция стабильна на проде:

1. ~~`internal/auth/permissions.go`~~ — удалён в Ф5 (мёртвый код).
2. `frontend/src/stores/auth.js::isAdmin` getter (deprecated alias).
3. `internal/database/migrate.go::EnforceSingleSuperAdmin` — оставить: поддерживает инвариант единственного супер-админа на каждом старте.
4. (Опционально, риск) `users.type_id` колонка — только если бизнес-роли полностью переехали в `users.role_id`. На момент написания (#187f) — нет, поле ещё используется.

## Восстановление при сбое миграции

Если после деплоя `is_super_admin` не выставился у buropropuskov:

```bash
docker compose exec db psql -U postgres -d auto_registry -c \
  "UPDATE users SET is_super_admin = true WHERE username = 'buropropuskov'"
```

или пересеместь:

```bash
make staging-seed PASS=newpass
```

Cmd `seed/main.go` вписан с `is_super_admin = true` для `buropropuskov` начиная с #229.
