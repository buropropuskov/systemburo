#!/usr/bin/env bash
# Снятие резервной копии системы: база данных и загруженные файлы.
#
#   ./scripts/backup.sh [local|staging|production]
#
# Копия состоит из выгрузки базы (pg_dump в формате custom) и архива тома с
# загруженными файлами. Порядок именно такой: сначала база, потом файлы. При
# обратном порядке файл, загруженный между шагами, попал бы в базу, но не в архив,
# и в восстановленной системе получилась бы ссылка на отсутствующий файл. При
# выбранном порядке возможен только файл-сирота, который никому не мешает.
#
# Файл параметров .env в копию НЕ входит намеренно: в нём ключ шифрования
# персональных данных, и хранить его вместе с копией базы означает, что кража
# архива равносильна краже персональных данных в открытом виде. Он копируется
# отдельно, в хранилище секретов организации.
#
# Параметры берутся из .env рядом со скриптом (см. BACKUP_* в .env.example).
# В контейнеры они не передаются - скрипт работает снаружи.
set -euo pipefail

# Права на каталог копий ставились и раньше, а режим самих файлов оставался за
# umask оператора, обычно 644. Внутри каталога с правами 700 это незаметно, но
# режим уезжает вместе с файлом: во внешнее хранилище, на съёмный носитель и на
# другой сервер при переносе. Ставится до создания чего бы то ни было, заодно
# закрывая журнал и файл состояния.
umask 077

ENVIRONMENT="${1:-local}"
# Метка попадает в имя файла: «buro-db-2026-08-01-1612-pered-obnovleniem.dump».
# Без неё копии различаются только временем, и найти среди них ту, что снята
# перед конкретной работой, можно лишь по памяти.
LABEL="${2:-}"
cd "$(dirname "$0")/.."

case "$ENVIRONMENT" in
  local)      COMPOSE=(docker compose) ;;
  staging)    COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.staging.yml) ;;
  production) COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.prod.yml) ;;
  *)
    echo "Использование: $0 [local|staging|production]" >&2
    exit 1
    ;;
esac

if [ ! -f .env ]; then
  echo "Файл параметров .env не найден в $(pwd)" >&2
  exit 1
fi
# Разрешение снимать копию без шифрования запоминается ДО чтения .env: строка
# ниже перекрывает окружение, и разовый запуск с BACKUP_ALLOW_UNENCRYPTED=yes
# молча терялся бы о пустую строку в файле параметров. Тот же приём применён в
# backup-status.sh для BACKUP_DIR.
ALLOW_UNENCRYPTED_ARG="${BACKUP_ALLOW_UNENCRYPTED:-}"
set -a; . ./.env; set +a

BACKUP_DIR="${BACKUP_DIR:-/var/backups/systemburo}"
BACKUP_KEEP_DAILY="${BACKUP_KEEP_DAILY:-7}"
BACKUP_KEEP_WEEKLY="${BACKUP_KEEP_WEEKLY:-4}"
BACKUP_KEEP_MONTHLY="${BACKUP_KEEP_MONTHLY:-6}"
BACKUP_UPLOADS_MODE="${BACKUP_UPLOADS_MODE:-weekly}"
# Группа учётной записи оператора. Оператор отвечает за то, чтобы копирование шло,
# но доступа к самим копиям у него нет и быть не должно: в них выгрузка базы с
# персональными данными. Ему открываются ровно два файла - отчёт о последнем
# запуске и журнал. Пустое значение отключает это целиком, и тогда состояние
# копирования видит только суперпользователь.
#
# Умолчание подставляется через "-", а не ":-", намеренно: строка вида
# BACKUP_OPERATOR_GROUP= в файле параметров - это осознанный отказ открывать
# состояние копирования, и подставлять поверх него buro значило бы открыть каталог
# вопреки написанному. То же различие сделано в backup-install.sh.
BACKUP_OPERATOR_GROUP="${BACKUP_OPERATOR_GROUP-buro}"
BACKUP_AGE_RECIPIENT="${BACKUP_AGE_RECIPIENT:-}"
# Единственный способ получить незашифрованную копию - выставить это в yes.
# Окружение имеет приоритет над файлом параметров: разовый запуск без правки
# .env тоже должен быть возможен.
BACKUP_ALLOW_UNENCRYPTED="${ALLOW_UNENCRYPTED_ARG:-${BACKUP_ALLOW_UNENCRYPTED:-}}"
if [ -n "$BACKUP_AGE_RECIPIENT" ]; then ENCRYPTED=true; else ENCRYPTED=false; fi
BACKUP_S3_REMOTE="${BACKUP_S3_REMOTE:-}"
BACKUP_S3_BUCKET="${BACKUP_S3_BUCKET:-}"
DB_NAME="${DB_NAME:-auto_registry}"
DB_USER="${DB_USER:-postgres}"

LOG_FILE="${BACKUP_DIR}/backup.log"
STATUS_FILE="${BACKUP_DIR}/status.json"
STAMP="$(date +%Y-%m-%d-%H%M)"
if [ -n "$LABEL" ]; then
  # Латиница, цифры, дефис и подчёркивание: имя файла уезжает в архив, во внешнее
  # хранилище и в команды восстановления, где пробелы и кириллица только мешают.
  case "$LABEL" in
    *[!A-Za-z0-9_-]*)
      echo "Метка «$LABEL» содержит недопустимые знаки. Разрешены латиница, цифры, дефис и подчёркивание." >&2
      exit 1
      ;;
  esac
  STAMP="${STAMP}-${LABEL}"
fi
TODAY_DOW="$(date +%u)"   # 7 = воскресенье, недельная копия
TODAY_DOM="$(date +%d)"   # 01 = первое число, месячная копия

mkdir -p "${BACKUP_DIR}"/{daily,weekly,monthly}
chmod 700 "${BACKUP_DIR}"

log() {
  local line
  line="$(date '+%Y-%m-%d %H:%M:%S') $*"
  echo "$line"
  echo "$line" >> "$LOG_FILE"
}

# Каталогу копий даётся 710 с группой оператора: это только право пройти внутрь,
# без права посмотреть, что там лежит. Прочитать по точному имени отчёт и журнал
# оператор сможет, перечислить копии - нет. Каталоги сроков хранения остаются 700,
# файлы копий - 600 у суперпользователя, так что и открыть копию, зная имя, тоже
# нельзя. Открываются ему ровно два файла и по 640: status.json и backup.log.
#
# Сменить группу может только суперпользователь. На сервере копирование и идёт от
# него, поэтому неудача там означает настоящую поломку - о ней пишем в журнал.
# На своей машине скрипт запускают обычным пользователем, там оператора нет вовсе,
# и предупреждение в каждом запуске только приучало бы пропускать слово ВНИМАНИЕ.
# Свой вывод chgrp заглушен намеренно: «invalid group» не объясняет ничего,
# строка ниже объясняет всё.
OPERATOR_ACCESS=false
if [ -n "$BACKUP_OPERATOR_GROUP" ]; then
  if chgrp "$BACKUP_OPERATOR_GROUP" "$BACKUP_DIR" 2>/dev/null; then
    chmod 710 "$BACKUP_DIR"
    OPERATOR_ACCESS=true
  elif [ "$(id -u)" -eq 0 ]; then
    log "ВНИМАНИЕ: каталог копий не открыт группе ${BACKUP_OPERATOR_GROUP} (проверьте, что такая группа заведена), состояние копирования видит только суперпользователь"
  fi
fi

# Открывает файл на чтение группе оператора. Вызывается для журнала и отчёта; на
# файлы копий не вызывается никогда - им остаётся режим от umask 077 выше.
share_with_operator() {
  [ "$OPERATOR_ACCESS" = true ] || return 0
  if ! chgrp "$BACKUP_OPERATOR_GROUP" "$1" 2>/dev/null || ! chmod 640 "$1"; then
    log "ВНИМАНИЕ: не удалось открыть $1 на чтение группе ${BACKUP_OPERATOR_GROUP}"
  fi
}

touch "$LOG_FILE"
share_with_operator "$LOG_FILE"

# Итог пишется в файл состояния при любом исходе: молчаливо упавший бэкап хуже
# отсутствующего, о нём узнают в момент восстановления. Отсюда же берёт данные
# команда backup-status.
FAIL_REASON=""
write_status() {
  local result="$1" size="${2:-0}"
  cat > "$STATUS_FILE" <<EOF
{
  "finished_at": "$(date -Is)",
  "environment": "$ENVIRONMENT",
  "result": "$result",
  "stamp": "$STAMP",
  "size_bytes": $size,
  "encrypted": $ENCRYPTED,
  "reason": "$FAIL_REASON"
}
EOF
  share_with_operator "$STATUS_FILE"
}

on_error() {
  local code=$?
  [ -n "$FAIL_REASON" ] || FAIL_REASON="прервано на строке $BASH_LINENO, код $code"
  log "ОШИБКА: $FAIL_REASON"
  write_status "failed"
  rm -rf "${WORK_DIR:-}"
  exit "$code"
}
trap on_error ERR

# Проверка ключа идёт до первого обращения к базе: смысл в том, чтобы не создать
# незашифрованную выгрузку вовсе, а не удалять её потом. Раньше здесь была строка
# в журнале - но журнал открывают уже при разборе аварии, когда копии с ФИО,
# паспортными данными и номерами патентов год как лежат в открытом виде.
if [ -z "$BACKUP_AGE_RECIPIENT" ] && [ "$BACKUP_ALLOW_UNENCRYPTED" != "yes" ]; then
  FAIL_REASON="не задан BACKUP_AGE_RECIPIENT: копия содержала бы персональные данные в открытом виде"
  log "ОТКАЗ: $FAIL_REASON"
  write_status "failed"
  cat >&2 <<'EOF'

Копирование не выполнено: не задан ключ шифрования копий.

Выгрузка базы и архив бланков содержат персональные данные - ФИО, паспортные
данные, номера патентов. Без ключа они лягут на диск и уедут во внешнее
хранилище в открытом виде: кража архива будет равносильна краже базы.

Исправить одним из двух способов.

1. Завести ключ (так правильно):
     age-keygen -o buro-backup.key
   Закрытую часть унести с сервера в хранилище секретов организации, открытую
   (строка вида age1...) вписать в .env рядом со скриптом:
     BACKUP_AGE_RECIPIENT=age1...

2. Осознанно согласиться на незашифрованные копии - тогда за их сохранность
   отвечает то место, где они лежат:
     BACKUP_ALLOW_UNENCRYPTED=yes

EOF
  exit 1
fi

WORK_DIR="$(mktemp -d "${BACKUP_DIR}/.work-XXXXXX")"

# Шифрование открытым ключом age: закрытый ключ на сервере не нужен и там не
# хранится, поэтому кража сервера не даёт доступа к архивам. Если age не
# установлен на хосте, берём его одноразовым контейнером - ставить пакеты на
# рабочий сервер ради одной команды не требуется.
encrypt_file() {
  local src="$1" dst="$2"
  if [ -z "$BACKUP_AGE_RECIPIENT" ]; then
    # Без шифрования имена совпадают (суффикса .age нет) - переносить нечего.
    if [ "$src" != "$dst" ]; then mv "$src" "$dst"; fi
    return
  fi
  if command -v age >/dev/null 2>&1; then
    age -r "$BACKUP_AGE_RECIPIENT" -o "$dst" "$src"
  else
    docker run --rm -i alpine:3.20 sh -c \
      "apk add --no-cache -q age && age -r '$BACKUP_AGE_RECIPIENT'" < "$src" > "$dst"
  fi
  rm -f "$src"
}

suffix() {
  if [ -n "$BACKUP_AGE_RECIPIENT" ]; then echo ".age"; fi
}

log "начало копирования, контур $ENVIRONMENT"
if [ "$ENCRYPTED" = false ]; then
  log "ВНИМАНИЕ: копия НЕ шифруется по явному разрешению BACKUP_ALLOW_UNENCRYPTED=yes и содержит персональные данные в открытом виде"
fi

# --- база данных ---
DB_RAW="${WORK_DIR}/buro-db-${STAMP}.dump"
FAIL_REASON="не удалось выгрузить базу $DB_NAME"
"${COMPOSE[@]}" exec -T db pg_dump -U "$DB_USER" -Fc "$DB_NAME" > "$DB_RAW"

# Повреждённую выгрузку ловим здесь, а не в момент восстановления, когда она
# оказывается единственной надеждой.
#
# Имя файла не указываем намеренно: pg_restore читает выгрузку со стандартного
# ввода. Вариант с /dev/stdin, который выглядит очевиднее, внутри контейнера не
# работает - pg_restore не находит сигнатуру и объявляет исправную выгрузку битой.
FAIL_REASON="выгрузка базы не читается: повреждена или пуста"
"${COMPOSE[@]}" exec -T db pg_restore --list < "$DB_RAW" > /dev/null
DB_SIZE="$(stat -c %s "$DB_RAW")"
log "база выгружена: $(numfmt --to=iec "$DB_SIZE" 2>/dev/null || echo "$DB_SIZE Б")"

FAIL_REASON="не удалось зашифровать выгрузку базы"
DB_FILE="${WORK_DIR}/buro-db-${STAMP}.dump$(suffix)"
encrypt_file "$DB_RAW" "$DB_FILE"

# --- загруженные файлы ---
# Файлы живут в двух местах: том uploads (фотографии и вложения заявок) и каталог
# файлового архива бланков на хосте. Второй намеренно сделан каталогом, а не томом,
# чтобы его можно было вынести на отдельный раздел - в копию он входит наравне.
#
# Полный архив каждый день умножается на число хранимых копий, поэтому по умолчанию
# файлы архивируются раз в неделю. Если их объём невелик, режим переключается на
# daily одной строкой в параметрах.
UPLOADS_FILE=""
ARCHIVE_FILE=""

# pack_files собирает tar.gz из тома или каталога хоста. Локально uploads смонтирован
# каталогом проекта, на сервере - именованным томом; поддержаны оба варианта, иначе
# скрипт нельзя проверить там, где его пишут.
pack_files() {
  local out="$1" volume="$2" host_dir="$3"
  if [ -n "$volume" ]; then
    # umask задаётся внутри контейнера: маска процесса-хозяина на процесс в
    # контейнере не распространяется, и без этой строки архив тома ложился бы
    # с правами 644 при закрытых остальных файлах копии.
    docker run --rm -v "$volume":/data:ro -v "$WORK_DIR":/out alpine:3.20 \
      sh -c "umask 077 && tar czf '/out/$(basename "$out")' -C /data ."
  else
    tar czf "$out" -C "$host_dir" .
  fi
}

if [ "$BACKUP_UPLOADS_MODE" = "daily" ] || [ "$TODAY_DOW" = "7" ] || [ "$TODAY_DOM" = "01" ]; then
  UPLOADS_VOL="$(docker volume ls --format '{{.Name}}' | grep '_uploads$' | head -1 || true)"
  UPLOADS_HOST="${UPLOAD_PATH:-./uploads}"
  if [ -n "$UPLOADS_VOL" ] || [ -d "$UPLOADS_HOST" ]; then
    FAIL_REASON="не удалось заархивировать загруженные файлы"
    UPLOADS_RAW="${WORK_DIR}/buro-uploads-${STAMP}.tar.gz"
    pack_files "$UPLOADS_RAW" "$UPLOADS_VOL" "$UPLOADS_HOST"
    log "загруженные файлы заархивированы: $(numfmt --to=iec "$(stat -c %s "$UPLOADS_RAW")" 2>/dev/null || stat -c %s "$UPLOADS_RAW")"

    FAIL_REASON="не удалось зашифровать архив файлов"
    UPLOADS_FILE="${UPLOADS_RAW}$(suffix)"
    encrypt_file "$UPLOADS_RAW" "$UPLOADS_FILE"
  else
    log "ВНИМАНИЕ: ни тома, ни каталога загруженных файлов не найдено, файлы в копию не вошли"
  fi

  ARCHIVE_HOST="${ARCHIVE_HOST_PATH:-./archive}"
  if [ -d "$ARCHIVE_HOST" ]; then
    FAIL_REASON="не удалось заархивировать файловый архив бланков"
    ARCHIVE_RAW="${WORK_DIR}/buro-archive-${STAMP}.tar.gz"
    tar czf "$ARCHIVE_RAW" -C "$ARCHIVE_HOST" .
    log "файловый архив заархивирован: $(numfmt --to=iec "$(stat -c %s "$ARCHIVE_RAW")" 2>/dev/null || stat -c %s "$ARCHIVE_RAW")"

    FAIL_REASON="не удалось зашифровать файловый архив"
    ARCHIVE_FILE="${ARCHIVE_RAW}$(suffix)"
    encrypt_file "$ARCHIVE_RAW" "$ARCHIVE_FILE"
  fi
else
  log "файлы пропущены: режим $BACKUP_UPLOADS_MODE, сегодня не день архивации"
fi

# --- раскладка по срокам хранения ---
# Одна и та же копия попадает в несколько каталогов жёсткой ссылкой: место
# занимается один раз, а сроки хранения у каталогов разные.
place() {
  local file="$1" dir="$2"
  ln -f "$file" "${BACKUP_DIR}/${dir}/$(basename "$file")"
}

FAIL_REASON="не удалось разложить копию по каталогам"
for f in "$DB_FILE" ${UPLOADS_FILE:+"$UPLOADS_FILE"} ${ARCHIVE_FILE:+"$ARCHIVE_FILE"}; do
  place "$f" daily
  # Именно if, а не «условие && действие»: при ложном условии такая строка
  # возвращает единицу, и set -e обрывает скрипт в обычный будний день.
  if [ "$TODAY_DOW" = "7" ]; then place "$f" weekly; fi
  if [ "$TODAY_DOM" = "01" ]; then place "$f" monthly; fi
done
rm -rf "$WORK_DIR"

# --- ротация ---
# Считаем по меткам времени в именах, а не по времени файла: жёсткие ссылки и
# копирование сохраняют исходную дату не всегда.
# find, а не ls по шаблону: при пустом каталоге ls возвращает ошибку, а с pipefail
# она поднимается наверх и обрывает копирование в первый же день, когда недельных
# копий ещё нет. find на существующем каталоге отдаёт пустой список и код ноль.
rotate() {
  local dir="$1" keep="$2" prefix
  for prefix in buro-db buro-uploads buro-archive; do
    find "${BACKUP_DIR}/${dir}" -maxdepth 1 -name "${prefix}-*" -type f -printf '%f\n' \
      | sort -r | tail -n +"$((keep + 1))" \
      | while read -r name; do
          rm -f "${BACKUP_DIR}/${dir}/${name}"
          log "удалена по сроку хранения: ${name}"
        done
  done
}
FAIL_REASON="не удалось выполнить ротацию"
rotate daily "$BACKUP_KEEP_DAILY"
rotate weekly "$BACKUP_KEEP_WEEKLY"
rotate monthly "$BACKUP_KEEP_MONTHLY"

# --- выгрузка вне сервера ---
# Копия на том же диске не защищает от отказа диска - это требование раздела 11.3
# руководства. Пока хранилище не подключено, честно пишем об этом в журнал, а не
# делаем вид, что всё в порядке.
if [ -n "$BACKUP_S3_REMOTE" ] && [ -n "$BACKUP_S3_BUCKET" ]; then
  FAIL_REASON="не удалось выгрузить копии во внешнее хранилище"
  docker run --rm \
    -v "${HOME}/.config/rclone:/config/rclone:ro" \
    -v "${BACKUP_DIR}:/data:ro" \
    rclone/rclone:latest sync /data "${BACKUP_S3_REMOTE}:${BACKUP_S3_BUCKET}" \
    --exclude '.work-*/**' --exclude 'backup.log'
  log "копии выгружены во внешнее хранилище ${BACKUP_S3_REMOTE}:${BACKUP_S3_BUCKET}"
else
  log "ВНИМАНИЕ: внешнее хранилище не настроено, копии лежат только на этом сервере"
fi

TOTAL="$(du -sb "$BACKUP_DIR" 2>/dev/null | cut -f1 || echo 0)"
FAIL_REASON=""
write_status "success" "$TOTAL"
log "копирование завершено, всего в хранилище $(numfmt --to=iec "$TOTAL" 2>/dev/null || echo "$TOTAL Б")"
