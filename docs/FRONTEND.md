# Frontend — Архитектура

Vue 3 (Options API), Vue Router 4, Pinia, CSS custom properties.

## Инициализация

`main.js`: createApp → Pinia → Router → `v-permission` directive → mount.

## Структура

```
frontend/src/
├── main.js                  # Точка входа
├── App.vue                  # Корневой компонент (auth, session, page transitions)
├── router.js                # 14 маршрутов, beforeEach guard
├── assets/tokens.css        # Design tokens (цвета, отступы, радиусы)
├── stores/
│   ├── auth.js              # JWT-токены, isAuthenticated, isAdmin
│   ├── permissions.js       # Права доступа (key → allow/deny)
│   └── ui.js                # Toast-уведомления, sidebar
├── api/
│   ├── client.js            # apiRequest / apiRequestRaw
│   ├── applications.js      # 15 функций
│   ├── cars.js, employees.js, users.js, ...
│   └── settings.js
├── components/
│   ├── ui/                  # Переиспользуемые примитивы (12 шт.)
│   ├── CreateApplication/   # Форма создания заявки
│   ├── ApplicationDetail/   # Детали заявки
│   ├── UnloadPlaces/        # Места выгрузки
│   └── NavMenu.vue, TheHeader.vue, LoginComponent.vue, ...
├── views/                   # Страницы (7 шт.)
├── composables/             # useFormValidation, useToast, useDropdownState, useEntitySelection
└── directives/permission.js # v-permission
```

## Маршрутизация

| Путь | Компонент | Auth | Admin |
|------|-----------|------|-------|
| `/` | LoginComponent | — | — |
| `/personal-cabinet` | AccountComponent | Да | — |
| `/submit-form` | CreateApplication | Да | — |
| `/center` | ApplicationsCenter | Да | — |
| `/carsview` | CarsView | Да | — |
| `/employeesview` | EmployeeView | Да | — |
| `/news` | NewsAndReview | Да | — |
| `/table`, `/table/:name` | TablesComponent | Да | Да |
| `/table-constructor` | TableConstructor | Да | Да |
| `/number-format` | NumberFormat | Да | Да |
| `/admin/feedback` | FeedbackPage | Да | Да |
| `/admin/settings` | AdminSettings | Да | Да |
| `/admin/users` | AdminUsers | Да | Да |

Guard: `authStore.isAuthenticated` + `authStore.isAdmin`. Неавторизованные → `/`, не-админы → `/personal-cabinet`.

## Stores

### auth.js

Единственный источник правды для токенов. Все компоненты читают/пишут через store, не через localStorage напрямую.

- **State:** `token`, `refreshToken` (инициализируются из localStorage)
- **Getters:** `isAuthenticated` (проверка exp), `isAdmin` (type_id === 6), `username`, `userPayload`
- **Actions:** `setTokens(token, refresh)` → пишет в state + localStorage, `clearTokens()` → чистит оба

### permissions.js

- **State:** `permissions` (объект key→value), `loaded`
- **Getter:** `hasPermission(key)` — админы всегда true, остальные проверяют `permissions[key] === 'allow'`
- **Actions:** `fetchPermissions()`, `clearPermissions()`

### ui.js

- Toast-уведомления: `success(msg)`, `error(msg)`, `warning(msg)`
- Состояние sidebar

## API-клиент

`api/client.js` — центральная точка для всех HTTP-запросов:

1. Берёт токен из `useAuthStore().token`
2. Добавляет `Authorization: Bearer ${token}` и `Content-Type: application/json`
3. Таймаут 10 секунд (AbortController)
4. **Envelope unwrapping:** `{success: true, data: {...}}` → возвращает `data`; `{success: false, error: "msg"}` → возвращает `{message: "msg"}`

`apiRequestRaw()` — без unwrapping, для запросов с пагинацией (meta + data).

## UI-компоненты

`components/ui/` — 12 переиспользуемых примитивов:

| Компонент | Назначение |
|-----------|------------|
| FormField | Обёртка для input с label, required-индикатором, inline-ошибкой |
| BaseModal | Модальное окно с overlay и scale-анимацией |
| BaseDropdown | Выпадающий список |
| FilterTabs | Вкладки-фильтры |
| StatusBadge | Бейдж статуса |
| GridSelector | Сеточный мультиселект |
| SelectionModal | Модал для выбора элементов |
| SkeletonLine | Линия-заглушка (shimmer) |
| SkeletonBlock | Блок-заглушка |
| SkeletonTable | Таблица-заглушка (header + rows) |
| SkeletonCard | Карточка-заглушка |
| SkeletonTransition | Обёртка skeleton→content с fade-переходом |

## Skeleton-система

`SkeletonTransition` управляет показом заглушек:

- **delay** (200ms) — не показывать skeleton, если данные пришли быстро
- **minDuration** (400ms) — если показали, держать минимум 400ms (без мерцания)
- **Transition:** opacity fade, `cubic-bezier(0.4, 0, 0.2, 1)`

Интегрирован в 8 страниц: ApplicationsCenter, AdminUsers, CarsView, EmployeeView, AdminSettings, FeedbackPage, TheHeader, AccountComponent.

## Composables

| Composable | Назначение |
|------------|------------|
| `useFormValidation` | Правила валидации, field-level ошибки, `validateField()`, `validateAll()`, `getFieldError()` |
| `useToast` | Обёртка над UIStore для toast-уведомлений |
| `useDropdownState` | Открытие/закрытие dropdown с click-outside |
| `useEntitySelection` | Мультиселект с temp/confirmed состояниями |

## Директива v-permission

```vue
<button v-permission="'can_edit_tables'">Редактировать</button>
```

Проверяет `permissionsStore.hasPermission(key)`. Если нет доступа — `display: none`. Админы проходят всегда.

## Page transitions

`App.vue` оборачивает `<router-view>` в `<transition name="page-fade" mode="out-in">` — плавный fade при смене страниц (0.25s enter, 0.15s leave).

## Design tokens

`assets/tokens.css`:

```css
--color-primary: #4F5BDF
--color-border: #e6e6e6
--color-bg: #f8f9ff
--color-skeleton: #e9eaef
--color-skeleton-shine: #f4f5f9
--radius-sm: 8px / --radius-md: 15px
--spacing-sm: 10px / --spacing-md: 20px
```
