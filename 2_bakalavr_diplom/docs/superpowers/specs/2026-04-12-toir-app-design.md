# Спецификация: Веб-приложение для управления ТОиР

## Контекст

Приложение разрабатывается в рамках бакалаврской ВКР и должно точно соответствовать тому, что описано в главах 2 и 3 диплома. Реализация по TDD: сначала тест, потом код.

## Стек

- **Backend**: Go 1.22+, Echo v4, GORM, golang-migrate, JWT (golang-jwt), bcrypt, testify
- **Frontend**: Vue 3, TypeScript, Vite, Pinia, Vue Router, Axios
- **DB**: PostgreSQL 16
- **Infra**: Docker, Docker Compose, Nginx

## Директория

`/home/washka/project/diplom/diplom-toir-app/`

## Структура монорепо

```
diplom-toir-app/
├── backend/
│   ├── cmd/
│   │   ├── server/main.go
│   │   └── seed/main.go
│   ├── internal/
│   │   ├── config/config.go
│   │   ├── database/database.go
│   │   ├── models/
│   │   │   ├── user.go
│   │   │   ├── equipment.go
│   │   │   ├── repair_request.go
│   │   │   ├── maintenance_schedule.go
│   │   │   ├── work_order.go
│   │   │   ├── maintenance_log.go
│   │   │   ├── part.go
│   │   │   └── work_order_part.go
│   │   ├── repository/
│   │   │   ├── user_repository.go
│   │   │   ├── user_repository_test.go
│   │   │   ├── equipment_repository.go
│   │   │   ├── equipment_repository_test.go
│   │   │   ├── repair_request_repository.go
│   │   │   ├── repair_request_repository_test.go
│   │   │   ├── maintenance_schedule_repository.go
│   │   │   └── ... (аналогично для остальных)
│   │   ├── services/
│   │   │   ├── auth_service.go
│   │   │   ├── auth_service_test.go
│   │   │   ├── equipment_service.go
│   │   │   ├── equipment_service_test.go
│   │   │   ├── repair_request_service.go
│   │   │   ├── repair_request_service_test.go
│   │   │   └── ...
│   │   ├── handlers/
│   │   │   ├── auth_handler.go
│   │   │   ├── auth_handler_test.go
│   │   │   ├── equipment_handler.go
│   │   │   ├── equipment_handler_test.go
│   │   │   └── ...
│   │   ├── middleware/
│   │   │   ├── jwt.go
│   │   │   ├── jwt_test.go
│   │   │   ├── rbac.go
│   │   │   └── rbac_test.go
│   │   └── validator/validator.go
│   ├── pkg/
│   │   └── response/response.go
│   ├── migrations/
│   │   ├── 000001_create_users.up.sql
│   │   ├── 000001_create_users.down.sql
│   │   ├── 000002_create_equipment.up.sql
│   │   └── ... (по одной миграции на сущность)
│   ├── Dockerfile
│   ├── go.mod
│   └── .env.example
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   │   └── client.ts
│   │   ├── stores/
│   │   │   ├── auth.ts
│   │   │   ├── equipment.ts
│   │   │   ├── requests.ts
│   │   │   └── dashboard.ts
│   │   ├── views/
│   │   │   ├── LoginView.vue
│   │   │   ├── DashboardView.vue
│   │   │   ├── EquipmentListView.vue
│   │   │   ├── EquipmentDetailView.vue
│   │   │   ├── RequestListView.vue
│   │   │   ├── RequestCreateView.vue
│   │   │   ├── RequestDetailView.vue
│   │   │   ├── ScheduleListView.vue
│   │   │   └── UsersView.vue
│   │   ├── components/
│   │   │   ├── AppLayout.vue
│   │   │   ├── AppSidebar.vue
│   │   │   ├── DashboardCard.vue
│   │   │   ├── DataTable.vue
│   │   │   └── StatusBadge.vue
│   │   ├── router/
│   │   │   └── index.ts
│   │   ├── App.vue
│   │   └── main.ts
│   ├── Dockerfile
│   ├── nginx.conf
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── docker-compose.yml
├── .env.example
└── README.md
```

## БД: 8 сущностей

### users
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| username | VARCHAR(50) UNIQUE NOT NULL | Логин |
| email | VARCHAR(255) UNIQUE NOT NULL | |
| password_hash | VARCHAR(255) NOT NULL | bcrypt |
| full_name | VARCHAR(255) NOT NULL | |
| role | VARCHAR(20) NOT NULL | operator/technician/engineer/admin |
| is_active | BOOLEAN DEFAULT true | |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### equipment
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| name | VARCHAR(255) NOT NULL | Наименование |
| inventory_number | VARCHAR(50) UNIQUE NOT NULL | Инвентарный номер |
| type | VARCHAR(100) | Тип оборудования |
| manufacturer | VARCHAR(255) | Производитель |
| model | VARCHAR(255) | Модель |
| serial_number | VARCHAR(100) | Серийный номер |
| location | VARCHAR(255) | Местоположение |
| status | VARCHAR(20) DEFAULT 'active' | active/maintenance/decommissioned |
| installation_date | DATE | Дата ввода |
| last_maintenance_date | DATE | Дата последнего ТО |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### repair_requests
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| equipment_id | INT FK -> equipment | |
| title | VARCHAR(255) NOT NULL | |
| description | TEXT | |
| priority | VARCHAR(20) NOT NULL | low/medium/high/critical |
| status | VARCHAR(20) DEFAULT 'new' | new/assigned/in_progress/waiting_parts/completed/closed |
| created_by | INT FK -> users | Автор заявки |
| assigned_to | INT FK -> users | Назначенный техник |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |
| completed_at | TIMESTAMP | |

### maintenance_schedules
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| equipment_id | INT FK -> equipment | |
| type | VARCHAR(100) | Вид ТО |
| interval_days | INT NOT NULL | Периодичность |
| last_performed | DATE | |
| next_date | DATE NOT NULL | Следующая дата |
| description | TEXT | |
| is_active | BOOLEAN DEFAULT true | |
| created_by | INT FK -> users | |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### work_orders
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| repair_request_id | INT FK -> repair_requests | Может быть NULL для плановых |
| schedule_id | INT FK -> maintenance_schedules | Может быть NULL для аварийных |
| description | TEXT | |
| planned_start | TIMESTAMP | |
| planned_end | TIMESTAMP | |
| actual_start | TIMESTAMP | |
| actual_end | TIMESTAMP | |
| status | VARCHAR(20) DEFAULT 'pending' | pending/in_progress/completed |
| assigned_to | INT FK -> users | |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### maintenance_logs
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| equipment_id | INT FK -> equipment | |
| work_order_id | INT FK -> work_orders | |
| type | VARCHAR(20) | repair/maintenance |
| description | TEXT | |
| performed_by | INT FK -> users | |
| performed_at | TIMESTAMP | |
| duration_hours | DECIMAL(5,2) | |
| created_at | TIMESTAMP | |

### parts
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| name | VARCHAR(255) NOT NULL | |
| article_number | VARCHAR(100) | |
| quantity | INT DEFAULT 0 | На складе |
| unit | VARCHAR(20) | шт/кг/л |
| min_quantity | INT DEFAULT 0 | Минимальный остаток |
| location | VARCHAR(255) | Место хранения |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### work_order_parts
| Поле | Тип | Описание |
|------|-----|----------|
| id | SERIAL PK | |
| work_order_id | INT FK -> work_orders | |
| part_id | INT FK -> parts | |
| quantity_used | INT NOT NULL | |

## API эндпоинты

### Auth
| Метод | Путь | Описание | Роли |
|-------|------|----------|------|
| POST | /api/auth/login | JWT login | public |
| POST | /api/auth/refresh | Refresh token | authenticated |

### Users
| Метод | Путь | Описание | Роли |
|-------|------|----------|------|
| GET | /api/users | Список | admin |
| POST | /api/users | Создать | admin |
| PUT | /api/users/:id | Обновить | admin |

### Equipment
| Метод | Путь | Описание | Роли |
|-------|------|----------|------|
| GET | /api/equipment | Список (пагинация, фильтры) | all auth |
| POST | /api/equipment | Создать | engineer, admin |
| GET | /api/equipment/:id | Детали + история | all auth |
| PUT | /api/equipment/:id | Обновить | engineer, admin |
| DELETE | /api/equipment/:id | Удалить | admin |

### Repair Requests
| Метод | Путь | Описание | Роли |
|-------|------|----------|------|
| GET | /api/repair-requests | Список | all auth |
| POST | /api/repair-requests | Создать | all auth |
| GET | /api/repair-requests/:id | Детали | all auth |
| PUT | /api/repair-requests/:id | Обновить статус/назначить | engineer, technician |

### Maintenance Schedules
| Метод | Путь | Описание | Роли |
|-------|------|----------|------|
| GET | /api/maintenance-schedules | Список | engineer, admin |
| POST | /api/maintenance-schedules | Создать | engineer |
| PUT | /api/maintenance-schedules/:id | Обновить | engineer |

### Work Orders
| Метод | Путь | Описание | Роли |
|-------|------|----------|------|
| GET | /api/work-orders | Список | engineer, technician |
| POST | /api/work-orders | Создать | engineer |
| PUT | /api/work-orders/:id | Обновить | engineer, technician |

### Dashboard
| Метод | Путь | Описание | Роли |
|-------|------|----------|------|
| GET | /api/dashboard | Метрики | engineer, admin |

## JSON envelope

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": { "page": 1, "per_page": 20, "total": 42 }
}
```

## JWT

- Access token: 15 минут, в Authorization: Bearer header
- Refresh token: 7 дней, в теле ответа
- Claims: user_id, role, exp
- Middleware извлекает claims и кладёт в echo.Context

## RBAC

Middleware проверяет роль из JWT claims. Матрица:

| Ресурс | operator | technician | engineer | admin |
|--------|----------|-----------|----------|-------|
| equipment (read) | + | + | + | + |
| equipment (write) | - | - | + | + |
| repair_requests (create) | + | + | + | + |
| repair_requests (assign) | - | - | + | + |
| repair_requests (update status) | - | + | + | + |
| maintenance_schedules | - | - | + | + |
| work_orders | - | + | + | + |
| dashboard | - | - | + | + |
| users | - | - | - | + |

## Docker Compose

3 сервиса:
- **db**: postgres:16-alpine, volume для данных, healthcheck
- **api**: Go multi-stage build (builder -> alpine), depends_on db healthy, порт 8080
- **frontend**: Node build -> nginx:alpine, depends_on api, порт 80

## TDD подход

Порядок для каждого модуля:
1. Написать тест (RED)
2. Минимальная реализация (GREEN)
3. Рефакторинг (REFACTOR)
4. Прогнать все тесты

Слои тестирования:
- **Unit**: services (моки репозиториев через интерфейсы), middleware
- **Integration**: repository (тестовая PostgreSQL в Docker), handlers (httptest)
- **E2E**: основные сценарии через API

## Seed данные

cmd/seed/main.go создаёт:
- 4 пользователя (по одному на роль), пароль: `password123`
- 10-15 единиц оборудования
- 5-7 заявок на ремонт (разные статусы)
- 3-4 графика ТО
- Запчасти
