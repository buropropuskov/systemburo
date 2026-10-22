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

Фиксированные ключи определены в [`internal/services/permission_keys.go`](../../internal/services/permission_keys.go); каталог формируется сервером и читается через API. Общие операции UI-дерева и подписи действий таблиц — [`frontend/src/utils/permissionCatalog.js`](../../frontend/src/utils/permissionCatalog.js).

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

role_permission_grants      # прямые права роли

user_permission_overrides   # личные allow/deny поверх групп
  user_id, permission_key, value=allow|deny

access_denials              # журнал отказов (3 мес активно)
access_denials_archive      # архив старше
```

## Permission resolver (backend)

`internal/services/permission_resolver.go`:

```go
resolver := services.NewPermissionResolver(db)
set, err := resolver.Resolve(ctx, userID)
if err != nil { return err } // ошибка проверки не разрешает действие
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

Разрешение операции не заменяет проверку видимости объекта. `RequirePermissionV2` сама логирует отказ в `access_denials` через goroutine и возвращает 403 JSON `{error, required_permission, banned}`.

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

Глобальный beforeEach ждёт состояние согласия для защищённой страницы, обновляет stale permissions и использует hasPermission с режимами super/admin/normal/banned. Отсутствие права ведёт на /403; заблокированный направляется в кабинет. Признак type_id не является bypass.

## Защитить кнопку или компонент

```vue
<button v-permission-scope="'action.delete.employee'" @click="del">
  Удалить
</button>

<button v-permission-scope:disable="'entity.cars.write'" @click="save">
  Сохранить
</button>

<RouterLink v-permission-scope="'page.admin.users'" to="/admin/users">
  Пользователи
</RouterLink>
```

Modes:
- `hide` (default) — `display: none`.
- `disable` — `pointer-events: none + opacity: 0.4 + aria-disabled`.

## Vite-plugin: автогенерация списка ключей

При `npm run build` плагин `frontend/build/vite-plugin-permissions.js` сканирует `*.vue/.ts/.js` на статические ключи в `v-permission-scope` (в плагине также остался шаблон распознавания PermissionScope; наличие шаблона не означает наличие компонента в приложении) и пишет `frontend/src/generated/permission-keys.json`:

```json
{
  "generated": "2026-05-06T16:57:42.792Z",
  "keys": ["action.delete.employee", "permission.audit.manage"]
}
```

Этот файл — результат статического обнаружения части использований, не полный каталог прав и не доказательство серверной защиты. Плагин не доказывает полноту динамических выражений; их серверную регистрацию и фактическое использование проверяют отдельно. Не создавать отсутствующий constants/permissionKeys.js по старой инструкции.

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

Интеграционные тесты находятся и в `internal/handlers`, и в `internal/services`. Прогоны с общей БД и `CleanDB` нельзя запускать конкурентно. Пять групп Go CI используют отдельные исполнители и базы; распределитель проверяет полноту и непересечение набора. Подробности — [ci-cd.md](../ci-cd.md).

### Frontend

В тесте согласуйте фикстуры профиля и ответа текущих прав `/api/permissions/my`, затем проверьте маршрут и действие в режимах обычного пользователя, администратора, суперадминистратора и блокировки. Один самостоятельно сформированный JWT не доказывает актуальные серверные права. Проверяйте личный запрет, чужую запись и ошибку источника прав отдельно от успешного сценария.

## Чек-лист добавления новой защищённой возможности

1. Найти подходящий существующий ключ; новый вводить только при самостоятельном новом действии согласно naming convention.
2. Зарегистрировать статический ключ в internal/services/permission_keys.go и серверном каталоге; для динамического — проверить существующий механизм генерации. При новом действии таблицы синхронизировать подписи permissionCatalog.js.
3. Защитить backend endpoint через `RequirePermissionV2`.
4. Защитить frontend (route `meta.permission` или `v-permission-scope`).
5. Запустить `npm run build` — plugin подхватит ключ в JSON.
6. Зайти в Админка → Группы прав, добавить ключ в нужные группы, либо создать новую группу.
7. Тесты: integration-тест на 403 без права + 200 с правом.

См. [admin-guide.md](admin-guide.md) для UI-флоу администратора.
