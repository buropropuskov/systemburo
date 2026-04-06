#!/bin/bash
set -euo pipefail

# Генерация .env для staging/production
# Использование: ./scripts/init-env.sh <staging|production> <domain>
# Пример:       ./scripts/init-env.sh staging stagingburo.washka17.site

ENV="${1:-}"
DOMAIN="${2:-}"

if [[ -z "$ENV" || -z "$DOMAIN" ]]; then
    echo "Использование: $0 <staging|production> <domain>"
    echo "Пример:        $0 staging stagingburo.washka17.site"
    exit 1
fi

if [[ "$ENV" != "staging" && "$ENV" != "production" ]]; then
    echo "Ошибка: окружение должно быть 'staging' или 'production'"
    exit 1
fi

ENV_FILE=".env"

if [[ -f "$ENV_FILE" ]]; then
    echo "Файл $ENV_FILE уже существует. Удалите его вручную, если хотите перегенерировать."
    echo "ВНИМАНИЕ: перегенерация сбросит JWT-токены и ключ шифрования!"
    exit 1
fi

# --- Генерация секретов ---
DB_PASSWORD=$(openssl rand -base64 24 | tr -d '/+=')
JWT_SECRET=$(openssl rand -base64 48 | tr -d '/+=')
JWT_REFRESH_SECRET=$(openssl rand -base64 48 | tr -d '/+=')
DATA_ENCRYPTION_KEY=$(openssl rand -hex 32)
PGADMIN_PASSWORD=$(openssl rand -base64 12 | tr -d '/+=')

# Staging-специфичные
BASIC_AUTH_USER="admin"
BASIC_AUTH_PASS=$(openssl rand -base64 12 | tr -d '/+=')

# Определяем параметры по окружению
if [[ "$ENV" == "staging" ]]; then
    LOG_LEVEL="debug"
    CORS_ORIGINS="https://${DOMAIN}"
    API_URL="https://${DOMAIN}"
else
    LOG_LEVEL="info"
    CORS_ORIGINS="https://${DOMAIN}"
    API_URL="https://${DOMAIN}"
fi

cat > "$ENV_FILE" <<EOF
# =============================================================
# Systemburo — ${ENV} (${DOMAIN})
# Сгенерировано $(date +%Y-%m-%d) скриптом init-env.sh
# =============================================================

# === База данных ===
DB_NAME=auto_registry
DB_USER=postgres
DB_PASSWORD=${DB_PASSWORD}
DATABASE_URL=postgres://postgres:${DB_PASSWORD}@db/auto_registry

# === Go Backend ===
BIND_HOST=0.0.0.0
BIND_PORT=8080
LOG_LEVEL=${LOG_LEVEL}

# === JWT ===
JWT_SECRET=${JWT_SECRET}
JWT_REFRESH_SECRET=${JWT_REFRESH_SECRET}

# === CORS ===
CORS_ALLOWED_ORIGINS=${CORS_ORIGINS}

# === Загрузка файлов ===
UPLOAD_MAX_FILE_SIZE=10485760
UPLOAD_ALLOWED_IMAGE_TYPES=image/jpeg,image/png,image/webp
UPLOAD_ALLOWED_DOC_TYPES=application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document
UPLOAD_PATH=/app/uploads

# === Шифрование ПД (152-ФЗ) ===
DATA_ENCRYPTION_KEY=${DATA_ENCRYPTION_KEY}
REQUIRE_ENCRYPTION=false

# === Rate Limiting ===
RATE_LIMIT_PER_MINUTE=200
RATE_LIMIT_WINDOW_SEC=60

# === Pagination ===
PAGINATION_MAX_LIMIT=100

# === pgAdmin ===
PGADMIN_EMAIL=admin@${DOMAIN}
PGADMIN_PASSWORD=${PGADMIN_PASSWORD}

# === Basic Auth (staging) ===
BASIC_AUTH_USER=${BASIC_AUTH_USER}
BASIC_AUTH_PASS=${BASIC_AUTH_PASS}

# === Frontend ===
VITE_API_BASE_URL=${API_URL}
EOF

echo ""
echo "=== .env создан для ${ENV} (${DOMAIN}) ==="
echo ""
echo "Сохраните эти данные:"
echo "  DB_PASSWORD:     ${DB_PASSWORD}"
echo "  PGADMIN:         admin@${DOMAIN} / ${PGADMIN_PASSWORD}"
if [[ "$ENV" == "staging" ]]; then
    echo "  Basic Auth:      ${BASIC_AUTH_USER} / ${BASIC_AUTH_PASS}"
    echo "  pgAdmin URL:     https://${DOMAIN}/pgadmin"
fi
echo ""
echo "Запуск: make ${ENV}-up"
