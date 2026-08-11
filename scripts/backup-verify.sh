#!/usr/bin/env bash
# Проверка восстановимости резервной копии.
#
#   ./scripts/backup-verify.sh [local|staging|production] [файл выгрузки]
#
# Копия, из которой ни разу не восстанавливались, резервной копией не является:
# повреждение обнаружится в тот момент, когда она окажется единственной надеждой.
# Скрипт берёт последнюю суточную копию (или указанную), разворачивает её во
# ВРЕМЕННУЮ базу рядом с рабочей, сверяет наполнение ключевых таблиц и удаляет
# временную базу за собой. Рабочая база при этом не затрагивается.
set -euo pipefail

# Та же маска, что в backup.sh, и по тем же причинам. Здесь она закрывает две вещи:
# расшифрованную выгрузку во временном каталоге - это вся база в открытом виде - и
# журнал, если проверка создаст его раньше первого копирования. Без неё журнал лёг
# бы с правами 644, то есть открытым всем на сервере, а не одному оператору.
umask 077

ENVIRONMENT="${1:-local}"
DB_ARCHIVE="${2:-}"
cd "$(dirname "$0")/.."

case "$ENVIRONMENT" in
  local)      COMPOSE=(docker compose) ;;
  staging)    COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.staging.yml) ;;
  production) COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.prod.yml) ;;
  *)
    echo "Использование: $0 [local|staging|production] [файл выгрузки]" >&2
    exit 1
    ;;
esac

if [ ! -f .env ]; then
  echo "Файл параметров .env не найден в $(pwd)" >&2
  exit 1
fi
set -a; . ./.env; set +a

BACKUP_DIR="${BACKUP_DIR:-/var/backups/systemburo}"
DB_USER="${DB_USER:-postgres}"
CHECK_DB="restore_check_$(date +%s)"
AGE_IDENTITY="${AGE_IDENTITY:-}"
LOG_FILE="${BACKUP_DIR}/backup.log"

log() {
  local line
  line="$(date '+%Y-%m-%d %H:%M:%S') проверка: $*"
  echo "$line"
  if [ -w "$(dirname "$LOG_FILE")" ] 2>/dev/null; then echo "$line" >> "$LOG_FILE"; fi
}

if [ -z "$DB_ARCHIVE" ]; then
  # find, а не ls по шаблону: на пустом каталоге ls возвращает ошибку, и с pipefail
  # она обрывает проверку вместо понятного сообщения «копий не найдено».
  DB_ARCHIVE="$(find "${BACKUP_DIR}/daily" -maxdepth 1 -name 'buro-db-*' -type f 2>/dev/null \
    | sort -r | head -1)"
fi
if [ -z "$DB_ARCHIVE" ] || [ ! -f "$DB_ARCHIVE" ]; then
  log "ОШИБКА: копий для проверки не найдено в ${BACKUP_DIR}/daily"
  exit 1
fi
log "проверяю $(basename "$DB_ARCHIVE")"

WORK_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$WORK_DIR"
  # Временную базу убираем при любом исходе, иначе неудачная проверка оставит
  # мусор, который со временем займёт место молча.
  "${COMPOSE[@]}" exec -T db psql -U "$DB_USER" -d postgres \
    -c "DROP DATABASE IF EXISTS ${CHECK_DB};" >/dev/null 2>&1 || true
}
trap cleanup EXIT

case "$DB_ARCHIVE" in
  *.age)
    if [ -z "$AGE_IDENTITY" ]; then
      log "ОШИБКА: копия зашифрована, но AGE_IDENTITY не задан"
      exit 1
    fi
    if command -v age >/dev/null 2>&1; then
      age -d -i "$AGE_IDENTITY" -o "${WORK_DIR}/db.dump" "$DB_ARCHIVE"
    else
      docker run --rm -i -v "$(realpath "$AGE_IDENTITY")":/key:ro alpine:3.20 sh -c \
        'apk add --no-cache -q age && age -d -i /key' < "$DB_ARCHIVE" > "${WORK_DIR}/db.dump"
    fi
    ;;
  *) cp "$DB_ARCHIVE" "${WORK_DIR}/db.dump" ;;
esac

"${COMPOSE[@]}" exec -T db psql -U "$DB_USER" -d postgres -c "CREATE DATABASE ${CHECK_DB};" >/dev/null
"${COMPOSE[@]}" exec -T db pg_restore -U "$DB_USER" -d "${CHECK_DB}" --no-owner \
  < "${WORK_DIR}/db.dump" >/dev/null

# Успешное восстановление ещё не значит пригодную копию: пустые таблицы
# восстанавливаются без единой ошибки. Поэтому смотрим на наполнение.
COUNTS="$("${COMPOSE[@]}" exec -T db psql -U "$DB_USER" -d "${CHECK_DB}" -At -F' ' -c \
  "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM applications), (SELECT count(*) FROM system_tables);")"
USERS="$(echo "$COUNTS" | awk '{print $1}')"
APPS="$(echo "$COUNTS" | awk '{print $2}')"
TABLES="$(echo "$COUNTS" | awk '{print $3}')"

log "во временной копии: учётных записей $USERS, заявок $APPS, таблиц постов $TABLES"

if [ "${USERS:-0}" -lt 1 ]; then
  log "ОШИБКА: в копии нет ни одной учётной записи - для восстановления она непригодна"
  exit 1
fi

log "копия пригодна для восстановления"
