#!/usr/bin/env bash
# Аварийное снятие режима технических работ, когда войти в интерфейс нельзя:
# потерян доступ супер-администратора или не открывается форма входа.
# Правит флаг прямо в базе; прикладной сервер перезапускать не нужно - статус
# режима кэшируется не дольше 10 секунд.
#
# Использование: bash scripts/maintenance-off.sh [local|staging|production]
set -euo pipefail

ENVIRONMENT="${1:-local}"

case "$ENVIRONMENT" in
  local)
    COMPOSE=(docker compose)
    ;;
  staging)
    COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.staging.yml)
    ;;
  production)
    COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.prod.yml)
    ;;
  *)
    echo "Использование: $0 [local|staging|production]" >&2
    exit 1
    ;;
esac

"${COMPOSE[@]}" exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
UPDATE system_settings SET value = 'false' WHERE key = 'maintenance.enabled';
SELECT key, value FROM system_settings WHERE key = 'maintenance.enabled';
SQL

# Пустая выборка выше означает, что режим ни разу не включали - настройки в базе нет.
echo "Флаг режима выключен. Если он был включён, пользователи вернутся в систему в течение 10 секунд."
