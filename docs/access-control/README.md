# Система прав доступа

Документация по системе прав, реализованной в эпике [#187](https://github.com/buropropuskov/systemburo/issues/187) (PR #229–#239).

## Содержание

| Документ | Для кого |
|---|---|
| [admin-guide.md](admin-guide.md) | Супер-админ: роли, группы, журнал, ban, recovery |
| [developer-guide.md](developer-guide.md) | Разработчик: naming convention, защита API/UI, тесты |
| [migration-from-type-id-6.md](migration-from-type-id-6.md) | История перехода с хардкода `type_id = 6` |

## Краткая модель

```
Role (бизнес-роль) ──many──< RoleDefaultGroup >──many── PermissionGroup ──many──< PermissionGroupGrant
                                                              ^
                                                              │ many
                                                              │
User ──many──< UserGroup ─────────────────────────────────────┘
  │
  └──many──< UserPermissionOverride (deny приоритетнее всего)
```

- **Один user → одна Role**, **много PermissionGroup**.
- `is_super_admin BOOLEAN` — bypass всех проверок.
- `is_banned BOOLEAN` — 0 прав, refresh-токены revoked.
- Резолвер: `internal/services/permission_resolver.go`. Кэш TTL 30s.

## Naming convention

```
page.<route>                   страницы
tab.<name>                     вкладки
component.<name>.<verb>        компоненты
action.<verb>.<entity>         действия
entity.<name>.<crud>           CRUD сущности (read|write|delete)
table.<slug>.<verb>            динамические таблицы (auto-generate)
permission.audit.<verb>        журналы и аудит
```

См. константы в [`internal/services/permission_keys.go`](../../internal/services/permission_keys.go) и [`frontend/src/constants/permissionKeys.js`](../../frontend/src/constants/permissionKeys.js).
