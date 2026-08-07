#!/bin/bash
set -euo pipefail

# Генерация .env для staging/production
# Использование: ./scripts/init-env.sh <staging|production> <domain>
# Пример:       ./scripts/init-env.sh staging stagingburo.washka17.ru

ENV="${1:-}"
DOMAIN="${2:-}"

if [[ -z "$ENV" || -z "$DOMAIN" ]]; then
    echo "Использование: $0 <staging|production> <domain>"
    echo "Пример:        $0 staging stagingburo.washka17.ru"
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

# === Уведомления в браузер при закрытом сайте (Web Push) ===
# Пара ключей подписи: сгенерировать командой `make staging-vapid` (или
# `make deploy-vapid`) и вставить сюда. Пока обе строки пусты, доставка вне
# системы выключена: интерфейс честно сообщает об этом, остальное работает.
# Заполнять ОБЕ или ни одной - одна без второй считается опечаткой, и сервер
# откажется стартовать.
VAPID_PUBLIC_KEY=
VAPID_PRIVATE_KEY=
# Контакт бюро для служб доставки (Google, Mozilla, Apple): по нему они пишут,
# если с уведомлениями что-то не так. Пользователи его не видят. Формат
# mailto:адрес или https://сайт. При заданных ключах обязателен - служба Google
# отвергает уведомления с пустым контактом.
VAPID_SUBJECT=mailto:admin@${DOMAIN}
# Через сколько суток удаляется подписка устройства, которому ни разу не удалось
# доставить уведомление.
PUSH_SUBSCRIPTION_RETENTION_DAYS=180

# === pgAdmin ===
PGADMIN_EMAIL=admin@${DOMAIN}
PGADMIN_PASSWORD=${PGADMIN_PASSWORD}

# === Basic Auth (staging) ===
BASIC_AUTH_USER=${BASIC_AUTH_USER}
BASIC_AUTH_PASS=${BASIC_AUTH_PASS}

# === Frontend ===
VITE_API_BASE_URL=${API_URL}

# === Резервное копирование ===
# Сроки хранения и режим архивации файлов описаны в .env.example.
# Ключ шифрования копий заводится отдельно: age-keygen -o buro-backup.key,
# сюда вписывается ОТКРЫТАЯ часть, закрытая хранится вне сервера.
BACKUP_DIR=/var/backups/systemburo
BACKUP_AGE_RECIPIENT=
BACKUP_KEEP_DAILY=7
BACKUP_KEEP_WEEKLY=4
BACKUP_KEEP_MONTHLY=6
BACKUP_UPLOADS_MODE=weekly
BACKUP_S3_REMOTE=
BACKUP_S3_BUCKET=
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
echo "Резервное копирование настраивается отдельно:"
echo "  1. age-keygen -o buro-backup.key   (закрытый ключ унести с сервера!)"
echo "  2. вписать открытую часть в BACKUP_AGE_RECIPIENT в .env"
echo "  3. sudo ./scripts/backup-install.sh ${ENV}"
echo ""
echo "Запуск: make ${ENV}-up"
