#!/usr/bin/env bash
# Восстановление системы из резервной копии.
#
#   ./scripts/restore.sh [local|staging|production] <файл выгрузки базы> [архив файлов]
#
# Операция необратима: текущее содержимое базы заменяется содержимым копии.
# Поэтому она требует явного подтверждения и выполняется при остановленном
# прикладном сервере, под режимом технических работ - чтобы пользователи видели
# объяснение, а не ошибки.
#
# Порядок обратный снятию: сначала база, потом файлы. Проверка счётчиков идёт до
# запуска приложения: пустая база обнаруживается до того, как в неё начнут писать.
#
# Зашифрованные копии (.age) расшифровываются закрытым ключом, путь к которому
# передаётся в AGE_IDENTITY. На сервере этот ключ не хранится - его приносит
# оператор на время восстановления.
set -euo pipefail

ENVIRONMENT="${1:-}"
DB_ARCHIVE="${2:-}"
UPLOADS_ARCHIVE="${3:-}"
cd "$(dirname "$0")/.."

usage() {
  cat >&2 <<'EOF'
Использование:
  ./scripts/restore.sh [local|staging|production] <файл выгрузки базы> [архив файлов]

Переменные окружения:
  CONFIRM=yes       пропустить запрос подтверждения (для неинтерактивного запуска)
  AGE_IDENTITY=путь закрытый ключ age, если копия зашифрована
EOF
}

case "$ENVIRONMENT" in
  local)      COMPOSE=(docker compose) ;;
  staging)    COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.staging.yml) ;;
  production) COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.prod.yml) ;;
  *) usage; exit 1 ;;
esac

if [ -z "$DB_ARCHIVE" ] || [ ! -f "$DB_ARCHIVE" ]; then
  echo "Файл выгрузки базы не найден: ${DB_ARCHIVE:-не указан}" >&2
  usage
  exit 1
fi

if [ ! -f .env ]; then
  echo "Файл параметров .env не найден в $(pwd)" >&2
  exit 1
fi
set -a; . ./.env; set +a
DB_NAME="${DB_NAME:-auto_registry}"
DB_USER="${DB_USER:-postgres}"
AGE_IDENTITY="${AGE_IDENTITY:-}"

echo "Контур:        $ENVIRONMENT"
echo "База данных:   $DB_NAME"
echo "Выгрузка базы: $DB_ARCHIVE"
echo "Архив файлов:  ${UPLOADS_ARCHIVE:-не указан, файлы останутся текущими}"
echo
echo "Текущее содержимое базы будет заменено. Отменить это нельзя."

if [ "${CONFIRM:-}" != "yes" ]; then
  printf 'Введите ВОССТАНОВИТЬ для продолжения: '
  read -r answer
  if [ "$answer" != "ВОССТАНОВИТЬ" ]; then
    echo "Отменено." >&2
    exit 1
  fi
fi

# Восстановление идёт минутами, и без отсчёта непонятно, работает оно или зависло.
# Шаги нумеруются, у каждого печатается время выполнения.
STEP_NO=0
# Шагов семь, а с восстановлением загруженных файлов - восемь: этот шаг
# выполняется, только если архив передан вторым аргументом.
STEP_TOTAL=7
[ -n "$UPLOADS_ARCHIVE" ] && STEP_TOTAL=8
STEP_STARTED=0
step() {
  if [ "$STEP_STARTED" -ne 0 ]; then
    echo "      готово за $(( $(date +%s) - STEP_STARTED )) с"
  fi
  STEP_NO=$(( STEP_NO + 1 ))
  STEP_STARTED=$(date +%s)
  echo
  echo "[$STEP_NO/$STEP_TOTAL] $1"
}

WORK_DIR="$(mktemp -d)"
cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

# Прерваться посреди восстановления - худший исход: службы остановлены, режим
# технических работ включён, и без подсказки человек остаётся с выключенной
# системой. Поднимать её автоматически нельзя (база может быть неполной), поэтому
# печатаем точные команды.
on_failure() {
  echo >&2
  echo "ВОССТАНОВЛЕНИЕ ПРЕРВАНО. Система сейчас остановлена." >&2
  echo "Проверьте состояние базы, затем поднимите систему:" >&2
  echo "  ${COMPOSE[*]} start backend frontend nginx" >&2
  echo "  bash scripts/maintenance-off.sh ${ENVIRONMENT}" >&2
}
trap on_failure ERR

# Расшифровка. Отдельным шагом, чтобы дальше работать с обычным файлом независимо
# от того, была копия зашифрована или нет.
decrypt_if_needed() {
  local src="$1" out="$2"
  case "$src" in
    *.age)
      if [ -z "$AGE_IDENTITY" ]; then
        echo "Копия зашифрована, но AGE_IDENTITY не задан" >&2
        exit 1
      fi
      if command -v age >/dev/null 2>&1; then
        age -d -i "$AGE_IDENTITY" -o "$out" "$src"
      else
        docker run --rm -i -v "$(realpath "$AGE_IDENTITY")":/key:ro alpine:3.20 sh -c \
          'apk add --no-cache -q age && age -d -i /key' < "$src" > "$out"
      fi
      ;;
    *) cp "$src" "$out" ;;
  esac
}

step "Включаю режим технических работ"
# Пользователи должны видеть объяснение, а не ошибки подключения. Ключи те же,
# что использует служба режима работ; снимается он scripts/maintenance-off.sh.
# Сбой здесь не отменяет восстановление, но и не проглатывается молча: оператор
# должен знать, что люди увидят ошибки вместо объяснения.
if ! "${COMPOSE[@]}" exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
INSERT INTO system_settings (key, value, type) VALUES
  ('maintenance.enabled', 'true', 'bool'),
  ('maintenance.message', 'Идёт восстановление данных из резервной копии', 'string')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
SQL
then
  echo "ПРЕДУПРЕЖДЕНИЕ: режим технических работ включить не удалось, пользователи увидят ошибки" >&2
fi

step "Останавливаю прикладной сервер"
"${COMPOSE[@]}" stop backend frontend nginx

step "Восстанавливаю базу данных"
decrypt_if_needed "$DB_ARCHIVE" "${WORK_DIR}/db.dump"

# База пересоздаётся целиком, а не чистится ключом --clean. Причина: журналы
# запросов и обращений к персональным данным разбиты на разделы по датам, а снять
# ключ с раздела нельзя - он унаследован от родительской таблицы. На такой базе
# --clean выдаёт по ошибке на каждый раздел (их десятки) и возвращает ненулевой код,
# из-за чего восстановление обрывается на полпути, оставив систему выключенной.
"${COMPOSE[@]}" exec -T db psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity
      WHERE datname = '${DB_NAME}' AND pid <> pg_backend_pid();" >/dev/null
"${COMPOSE[@]}" exec -T db psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS ${DB_NAME};" -c "CREATE DATABASE ${DB_NAME};" >/dev/null
# pv рисует полосу по объёму выгрузки. Его отсутствие не повод ставить пакет на
# рабочий сервер - тогда идёт обычный отсчёт времени шага.
if command -v pv >/dev/null 2>&1; then
  pv "${WORK_DIR}/db.dump" | "${COMPOSE[@]}" exec -T db \
    pg_restore -U "$DB_USER" -d "$DB_NAME" --no-owner
else
  echo "      идёт восстановление, обычно занимает минуту-две..."
  "${COMPOSE[@]}" exec -T db pg_restore -U "$DB_USER" -d "$DB_NAME" --no-owner \
    < "${WORK_DIR}/db.dump"
fi

if [ -n "$UPLOADS_ARCHIVE" ]; then
  if [ ! -f "$UPLOADS_ARCHIVE" ]; then
    echo "Архив файлов не найден: $UPLOADS_ARCHIVE" >&2
    exit 1
  fi
  echo "== Восстанавливаю загруженные файлы"
  decrypt_if_needed "$UPLOADS_ARCHIVE" "${WORK_DIR}/uploads.tar.gz"
  UPLOADS_VOL="$(docker volume ls --format '{{.Name}}' | grep '_uploads$' | head -1)"
  if [ -z "$UPLOADS_VOL" ]; then
    echo "Не найден том с загруженными файлами" >&2
    exit 1
  fi
  docker run --rm -v "$UPLOADS_VOL":/data -v "$WORK_DIR":/in alpine:3.20 \
    tar xzf /in/uploads.tar.gz -C /data
fi

step "Проверяю восстановленное до запуска приложения"
# Сырую таблицу psql оператор читает плохо: подписи по-английски, рамки, лишние
# строки. Берём только числа и печатаем их словами.
COUNTS="$("${COMPOSE[@]}" exec -T db psql -U "$DB_USER" -d "$DB_NAME" -At -F' ' -c \
  "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM applications),
          (SELECT count(*) FROM system_tables);" | tr -d '\r')"
echo "   Учётные записи:  $(echo "$COUNTS" | awk '{print $1}')"
echo "   Заявки:          $(echo "$COUNTS" | awk '{print $2}')"
echo "   Таблицы постов:  $(echo "$COUNTS" | awk '{print $3}')"

step "Применяю уничтожения заново по журналу"
# Копия, снятая до уничтожения, возвращает и то, что оператор обязан был уничтожить.
# Закон допускает держать копии ограниченный срок при условии, что после
# восстановления удаления применяются заново, - здесь это условие и исполняется.
#
# Шаг стоит ЗДЕСЬ, до запуска системы, а не отдельной командой, о которой надо
# помнить: восстанавливают в спешке, и такой шаг пропускают первым. Пока приложение
# остановлено, команда идёт разовым контейнером - тем же приёмом, что миграции.
#
# Код возврата снимается явно, а не оставляется set -e: у команды их три (0 - снято
# или снимать было нечего, 1 - сбой, 3 - вернулось то, что автоматически не снять), и
# каждый требует своего. Сбой ниже поднимается обратно общему обработчику, так что
# || здесь ничего не проглатывает.
REPLAY_CODE=0
"${COMPOSE[@]}" run --rm -T backend ./server destruction replay -apply || REPLAY_CODE=$?

if [ "$REPLAY_CODE" -ne 0 ] && [ "$REPLAY_CODE" -ne 3 ]; then
  echo >&2
  echo "Повторное применение уничтожений завершилось с ошибкой (код ${REPLAY_CODE})." >&2
  on_failure
  exit "$REPLAY_CODE"
fi

# Код 3 означает: из копии вернулись данные, которые обязаны были остаться
# уничтоженными, а снять их сам проход не может (снос организации идёт только по
# проверенному пакету выгрузки). Напечатать про это предупреждение и тут же открыть
# систему пользователям нельзя - строку в длинном выводе прочитают не всегда, а
# восстанавливают в спешке. Поэтому остановка: система остаётся выключенной под
# режимом технических работ, пока человек не снимет эти данные.
#
# Второго прогона восстановления после этого не нужно: снятая руками организация
# перестаёт находиться, и проход про неё замолчит сам.
if [ "$REPLAY_CODE" -eq 3 ]; then
  echo >&2
  echo "ВОССТАНОВЛЕНИЕ ОСТАНОВЛЕНО: часть уничтожений автоматически не повторяется." >&2
  echo "Из копии вернулись данные, которые должны были остаться уничтоженными - строки" >&2
  echo "с пометкой ТРЕБУЕТ РУК выше. Система НЕ запущена и остаётся в режиме" >&2
  echo "технических работ: пользователи этих данных не увидят." >&2
  echo >&2
  echo "Снимите их (порядок напечатан рядом с каждой строкой), проверьте:" >&2
  echo "  ${COMPOSE[*]} run --rm backend ./server destruction replay" >&2
  echo "и поднимите систему:" >&2
  echo "  ${COMPOSE[*]} start backend frontend nginx" >&2
  echo "  bash scripts/maintenance-off.sh ${ENVIRONMENT}" >&2
  # Свой текст вместо общего on_failure: тот советует проверить состояние базы, а база
  # здесь как раз в порядке - разбираться надо не с ней.
  trap - ERR
  exit 1
fi

step "Запускаю систему"
# start, а не up: up обращается к реестру образов, где нужна авторизация, и
# восстановление падало бы на последнем шаге, оставив систему выключенной.
"${COMPOSE[@]}" start backend frontend nginx

step "Снимаю режим технических работ"
bash scripts/maintenance-off.sh "$ENVIRONMENT" >/dev/null

echo "      готово за $(( $(date +%s) - STEP_STARTED )) с"
echo
echo "Восстановление завершено. Проверьте вход в систему и открытие заявки."
echo "Если числа выше отличаются от ожидаемых, повторите восстановление из другой копии."
