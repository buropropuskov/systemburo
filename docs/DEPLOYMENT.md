# Инструкция по разворачиванию Systemburo

## 1. С Docker (рекомендуемый способ)

### Требования
- Docker 24+
- Docker Compose v2
- 2GB RAM минимум (4GB рекомендуется)

### Шаги

1. Клонировать репозиторий:
```bash
git clone <repo> && cd systemburo
```

2. Создать файл окружения:
```bash
cp .env.example .env
# Отредактировать .env — установить безопасные значения для:
# - DB_PASSWORD (не использовать dev-password-change-me)
# - JWT_SECRET (минимум 32 символа)
# - JWT_REFRESH_SECRET (минимум 32 символа)
# - DATA_ENCRYPTION_KEY (сгенерировать: openssl rand -hex 32)
```

3. Запустить:
```bash
make up
# или: docker compose up -d
```

4. Проверить:
```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### Сервисы
| Сервис | Порт | Назначение |
|--------|------|-----------|
| go-backend | 8080 | Go API |
| frontend | 8081 | Vue 3 dev-сервер |
| db | 5432 | PostgreSQL 16 |
| pgadmin | 8082 | Управление БД |

## 2. Без Docker

### Требования
- Go 1.25+
- Node.js 18+
- PostgreSQL 16
- npm

### Backend

```bash
# Установить зависимости
go mod download

# Сгенерировать Swagger
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o docs

# Настроить env
export DATABASE_URL=postgres://user:pass@localhost:5432/auto_registry
export JWT_SECRET=your-secret-min-32-chars-here!!!!!
export JWT_REFRESH_SECRET=your-refresh-secret-min-32-chars!
export BIND_PORT=8080

# Запустить
go run ./cmd/server
```

### Frontend

```bash
cd frontend
npm ci
npm run serve  # dev-сервер на :8081
```

### База данных

```bash
createdb auto_registry
# Миграции выполняются автоматически при старте backend (GORM AutoMigrate)
```

## 3. Production деплой

### Требования
- VPS с 2+ CPU, 4GB+ RAM
- Docker + Docker Compose
- Домен с DNS A-записью

### Шаги

1. На VPS:
```bash
git clone <repo> /opt/systemburo
cd /opt/systemburo
cp .env.example .env
# Заполнить .env с production-значениями
```

2. Собрать и запустить:
```bash
make deploy-build
make deploy-up
```

3. Настроить SSL (certbot):
```bash
apt install certbot python3-certbot-nginx
certbot --nginx -d your-domain.com
```

4. Раскомментировать HTTPS-редирект в `nginx/nginx.conf`:
```nginx
return 301 https://$host$request_uri;
```

5. Добавить HSTS header (после настройки SSL):
```nginx
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

### Бэкапы

```bash
# Бэкап PostgreSQL
docker compose -f docker-compose.prod.yml exec db pg_dump -U postgres auto_registry > backup.sql

# Восстановление
docker compose -f docker-compose.prod.yml exec -i db psql -U postgres auto_registry < backup.sql
```

### Мониторинг

```bash
# Статус сервисов
docker compose -f docker-compose.prod.yml ps

# Логи
make deploy-logs

# Health check
curl http://localhost/health
```
