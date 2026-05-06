# Система прав: руководство разработчика

Гайд по добавлению новых страниц/действий/компонентов в систему прав. Покрывает naming convention, защиту бэкенд-эндпоинтов, защиту фронт-элементов, интеграцию автодискавера.

## Naming convention

Формат: `<scope>.<entity>.<verb>` (последний сегмент опционален).

| Префикс | Назначение | Примеры |
|---|---|---|
| `page.<route>` | Страницы навигационного меню | `page.center`, `page.cars` |
| `tab.<name>` | Вкладки внутри страниц | `tab.applications` |
| `component.<name>.<verb>` | Компоненты | `component.vehicle_history.export` |
| `action.<verb>.<entity>` | Действия | `action.delete.employee`, `action.ban.user` |
| `entity.<name>.<crud>` | CRUD сущности | `entity.cars.read \| write \| delete` |
| `table.<slug>.<verb>` | Динамические таблицы (auto-generate) | `table.checkpoint_a.view` |
| `permission.audit.*` | Журналы и аудит | `permission.audit.read \| manage` |

Константы зашиты в [`internal/services/permission_keys.go`](../../internal/services/permission_keys.go) и продублированы для UI-дерева в [`frontend/src/constants/permissionKeys.js`](../../frontend/src/constants/permissionKeys.js).

## Структура данных

```
roles                       # бизнес-роли: tenant, contractor, manager
  id, code, name, description

role_default_groups         # M:N: каждой роли N дефолтных групп
  role_id, group_id

permission_groups           # именованные наборы прав
  id, name, description

permission_group_grants     # права в каждой группе
  group_id, permission_key, value=allow|deny

users
  id, role_id, is_super_admin, is_banned, banned_at, banned_by

user_groups                 # явные группы юзера поверх роли
  user_id, group_id, granted_by, granted_at

user_permission_overrides   # точечные override (deny приоритетнее всего)
  user_id, permission_key, value=allow|deny

access_denials              # журнал отказов (3 мес активно)
access_denials_archive      # архив старше
```

## Permission resolver (backend)

`internal/services/permission_resolver.go`:

```go
resolver := services.NewPermissionResolver(db)
set, err := resolver.Resolve(ctx, userID)
if set.IsBanned()    { /* 0 прав */ }
if set.IsSuperAdmin(){ /* все права */ }
if set.Has("page.center") { /* allowed */ }
```

Кэш: in-memory `sync.Map`, TTL 30s. Инвалидируется при изменении ролей/групп/override.

## Защитить новый API endpoint

```go
// internal/router/router.go
import mw "systemburo/internal/middleware"

myMW := mw.RequirePermissionV2(permResolver, denialLog, services.KeyEntityCarsWrite)
protected.POST("/cars", carHandler.Create, myMW)
```

`RequirePermissionV2` сама логирует отказ в `access_denials` через goroutine и возвращает 403 JSON `{error, required_permission, banned}`.

Для super-admin-only эндпоинтов используйте `IsSuperAdmin(c)`:

```go
func (h *X) AdminOnly(c echo.Context) error {
    if !handlers.IsSuperAdmin(c) {
        return echo.NewHTTPError(http.StatusForbidden, "Доступ только для супер-администратора")
    }
    ...
}
```

## Защитить страницу (frontend)

```js
// frontend/src/router.js
{
  path: '/admin/audit',
  name: 'AdminAudit',
  component: () => import('./views/admin/AdminAudit.vue'),
  meta: { requiresAuth: true, permission: 'permission.audit.read' }
}
```

Глобальный `beforeEach` guard:

1. Проверит `is_super_admin` (bypass).
2. Загрузит permissions (если кэш stale).
3. Если право отсутствует → редирект на `/403?permission=...`.

## Защитить кнопку или компонент

```vue
<button v-permission-scope="'action.delete.employee'" @click="del">
  Удалить
</button>

<button v-permission-scope:disable="'entity.cars.write'" @click="save">
  Сохранить
</button>

<PermissionScope :key="'page.admin.users'" mode="hide">
  <RouterLink to="/admin/users">Пользователи</RouterLink>
</PermissionScope>
```

Modes:
- `hide` (default) — `display: none`.
- `disable` — `pointer-events: none + opacity: 0.4 + aria-disabled`.

## Vite-plugin: автогенерация списка ключей

При `npm run build` плагин `frontend/build/vite-plugin-permissions.js` сканирует `*.vue/.ts/.js` на статические ключи в `v-permission-scope` и `<PermissionScope key="...">` и пишет `frontend/src/generated/permission-keys.json`:

```json
{
  "generated": "2026-05-06T16:57:42.792Z",
  "keys": ["action.delete.employee", "permission.audit.manage"]
}
```

Этот файл — источник правды для проверки, какие ключи реально используются. Плагин не парсит динамические выражения; такие ключи нужно явно перечислить в `constants/permissionKeys.js`.

## Журнал отказов

При каждом 403 от `RequirePermissionV2` пишется запись в `access_denials` (async через goroutine):

```go
denialLog.Log(services.LogParams{
    UserID:        &userID,
    Resource:      method + " " + path,
    PermissionKey: &key,
    Reason:        models.DenialReasonPermission, // или DenialReasonBanned
    IPAddress:     &ip,
    UserAgent:     &ua,
})
```

Cron в `cmd/server/main.go::startAccessDenialsArchiver` раз в сутки переносит записи старше 90 дней в архив.

## Тестирование

### Backend

```go
// Создаём изолированный context чтобы не race-иться с другими тестами.
ut := models.UserType{Name: uniq("type"), Code: uniq("c")}
db.Create(&ut)
user := models.User{TypeID: ut.ID, IsSuperAdmin: false}
db.Create(&user)

resolver := services.NewPermissionResolver(db)
has, err := resolver.HasPermission(ctx, user.ID, "page.center")
```

Тесты-интеграции с CleanDB должны лежать в `internal/handlers/` (там тесты бегут sequentially внутри пакета на shared БД).

### Frontend

```js
authStore.setTokens(createMockJWT({ type_id: 6, is_super_admin: true }));
// Все права доступны через bypass

const store = usePermissionsStore();
await store.fetchPermissions();
expect(store.hasPermission('page.center')).toBe(true);
```

## Чек-лист добавления новой защищённой возможности

1. Придумать ключ согласно naming convention.
2. Если статический — добавить в `internal/services/permission_keys.go` и `frontend/src/constants/permissionKeys.js`.
3. Защитить backend endpoint через `RequirePermissionV2`.
4. Защитить frontend (route `meta.permission` или `v-permission-scope`).
5. Запустить `npm run build` — plugin подхватит ключ в JSON.
6. Зайти в Админка → Группы прав, добавить ключ в нужные группы, либо создать новую группу.
7. Тесты: integration-тест на 403 без права + 200 с правом.

См. [admin-guide.md](admin-guide.md) для UI-флоу администратора.
