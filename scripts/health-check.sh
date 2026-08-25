#!/usr/bin/env bash
# Проверка живости системы: отвечает ли она так, как её видит человек.
#
#   ./scripts/health-check.sh [local|staging|production]
#   HEALTH_CHECK_URL=https://stand.example ./scripts/health-check.sh staging
#
# Ставится на расписание один раз: sudo ./scripts/health-install.sh production
#
# Проверяются три вещи, и первые две - снаружи, тем же путём, что и у человека:
#   сайт  - внешний адрес отдаёт страницу;
#   вход  - учётная запись действительно входит, получает токен и выходит;
#   база  - отвечает на запрос.
#
# Почему не хватает ручки /health, которая уже есть: она отвечает "ok" на любом
# живом процессе, не заглядывая ни в базу, ни во вход. Система с отвалившейся
# базой отвечает ей бодро, не пуская при этом ни одного человека.
#
# Почему вход проверяется настоящим входом, а не отказом на заведомо неверный
# пароль: отказ доказывает лишь то, что система отвечает, а не то, что она пускает.
# Успешный вход подделать нечем - он требует прочитать строку пользователя, сверить
# пароль и выдать пару маркеров, то есть живую базу и работающую выдачу.
#
# Исторически довод был сильнее: до #2006 ЛЮБАЯ ошибка выборки пользователя,
# включая недоступную базу, сворачивалась в тот же отказ, что и несуществующий
# логин, и проба "ждём отказ" оставалась зелёной ровно во время аварии. Теперь
# сбой базы отвечает отдельно, но проверять вход по-прежнему надо настоящим
# входом: между "отвечает" и "пускает" разница остаётся.
#
# Письмо об отказе уходит напрямую по SMTP, параметры читаются из .env. Отправлять
# через почтовый слой системы нельзя: он часть того, что отказало.
#
# Коды возврата: 0 - в порядке (или объявленные техработы), 1 - отказ системы,
# 2 - сама проверка не смогла выполниться.
set -euo pipefail
umask 077

# Ранний перехват - до того, как появились каталог состояния и журнал. Всё, что
# ломается здесь (нечитаемый .env, каталог без права записи, кончившийся диск),
# писать некуда, но провалиться молча оно не должно: код 2 красит юнит в
# systemctl, а строка уходит в journalctl. Ниже, когда журнал доступен, перехват
# заменяется на полный - с записью состояния и письмом.
trap 'echo "СБОЙ ПРОВЕРКИ: прервано на строке ${LINENO}, код $? (состояние ещё не открыто, писать некуда)" >&2; exit 2' ERR

ENVIRONMENT="${1:-production}"

# Адрес и каталог состояния запоминаются ДО чтения .env и имеют приоритет над ним:
# `set -a; . ./.env` перекрывает окружение, и разовый запуск вида
# HEALTH_CHECK_URL=... ./scripts/health-check.sh молча проверял бы адрес из файла
# параметров вместо указанного. Тот же приём в backup-status.sh для BACKUP_DIR.
HEALTH_CHECK_URL_ARG="${HEALTH_CHECK_URL:-}"
HEALTH_STATE_DIR_ARG="${HEALTH_STATE_DIR:-}"

cd "$(dirname "$0")/.."

case "$ENVIRONMENT" in
  local)      COMPOSE=(docker compose) ;;
  staging)    COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.staging.yml) ;;
  production) COMPOSE=(docker compose -f docker-compose.base.yml -f docker-compose.prod.yml) ;;
  *)
    echo "Использование: $0 [local|staging|production]" >&2
    exit 2
    ;;
esac

if [ ! -f .env ]; then
  echo "Файл параметров .env не найден в $(pwd)" >&2
  exit 2
fi
set -a; . ./.env; set +a

HEALTH_CHECK_URL="${HEALTH_CHECK_URL_ARG:-${HEALTH_CHECK_URL:-}}"
HEALTH_STATE_DIR="${HEALTH_STATE_DIR_ARG:-${HEALTH_STATE_DIR:-/var/lib/systemburo/health}}"
HEALTH_CHECK_USERNAME="${HEALTH_CHECK_USERNAME:-}"
HEALTH_CHECK_PASSWORD="${HEALTH_CHECK_PASSWORD:-}"
HEALTH_TIMEOUT_SEC="${HEALTH_TIMEOUT_SEC:-20}"
HEALTH_CHECK_DB="${HEALTH_CHECK_DB:-auto}"
HEALTH_ALERT_TO="${HEALTH_ALERT_TO:-}"
HEALTH_ALERT_REPEAT_MIN="${HEALTH_ALERT_REPEAT_MIN:-60}"
DB_NAME="${DB_NAME:-auto_registry}"
DB_USER="${DB_USER:-postgres}"

STATE_FILE="${HEALTH_STATE_DIR}/state.json"
LOG_FILE="${HEALTH_STATE_DIR}/health.log"
NOW_ISO="$(date -Is)"

mkdir -p "$HEALTH_STATE_DIR"
# 700 на каталог и 600 на файлы (umask выше): в журнале адрес системы и имя
# проверяющей учётной записи - половина пары для входа.
chmod 700 "$HEALTH_STATE_DIR"
touch "$LOG_FILE"

log() {
  local line
  line="$(date '+%Y-%m-%d %H:%M:%S') $*"
  echo "$line"
  echo "$line" >> "$LOG_FILE"
}

WORK_DIR="$(mktemp -d)"
cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

# Итоги проб. Три состояния у каждой: ok, fail, skip. Пропуск - это "не
# проверялось, и вот почему", а не "в порядке": молчание вместо ответа и есть та
# самая немота инструмента диагностики, ради которой всё затевалось. Поэтому у
# пропуска всегда есть причина словами, и она доезжает и в журнал, и в письмо.
SITE_RESULT="skip";  SITE_NOTE="не проверялся"
LOGIN_RESULT="skip"; LOGIN_NOTE="не проверялся"
DB_RESULT="skip";    DB_NOTE="не проверялась"

# on_error ловит то, чего сценарий не предусмотрел: пропавший curl, каталог без
# права записи, синтаксическую ошибку после правки. Такой запуск не имеет права
# выглядеть как успешный - он записывается отдельным состоянием broken и уходит
# письмом, потому что ослепшая проверка ничем не лучше отказавшей системы.
BROKEN_REASON=""
on_error() {
  local code=$? line=$1
  trap - ERR
  BROKEN_REASON="${BROKEN_REASON:-прервано на строке ${line}, код ${code}}"
  log "СБОЙ ПРОВЕРКИ: ${BROKEN_REASON}"
  finish_state "broken" "${BROKEN_REASON}" || true
  exit 2
}

# --- Пробы -------------------------------------------------------------------

# Если адрес закрыт базовой авторизацией nginx (так стоит стенд), без неё проверка
# получала бы 401 на каждом запросе и звала человека к исправной системе.
BASIC_AUTH_ARGS=()
if [ -n "${HEALTH_CHECK_BASIC_AUTH:-}" ]; then
  BASIC_AUTH_ARGS=(--user "$HEALTH_CHECK_BASIC_AUTH")
fi

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

# http_request выполняет запрос и кладёт результат в HTTP_CODE и CURL_ERROR.
# Ответ отдаётся переменными, а не выводом: при `code="$(...)"` присваивание идёт
# в подоболочке, и объяснение сетевой неудачи туда же и девалось - письмо об
# отказе приходило с пустой причиной, ровно в тех случаях (соединение отвергнуто,
# имя не разрешается, истекло ожидание), которые и составляют настоящие аварии.
#
# Сетевая неудача переводится в слово: "не удалось подключиться" объясняет
# причину, а голый код 7 заставляет лезть в справочник посреди аварии.
HTTP_CODE=""
CURL_ERROR=""
http_request() {
  local out="$1"; shift
  local rc=0
  HTTP_CODE="$(curl -sS -o "$out" -w '%{http_code}' \
    --max-time "$HEALTH_TIMEOUT_SEC" "$@" 2>"${WORK_DIR}/curl.err")" || rc=$?
  if [ "$rc" -ne 0 ]; then
    case "$rc" in
      6)     CURL_ERROR="имя узла не разрешается" ;;
      7)     CURL_ERROR="не удалось подключиться" ;;
      28)    CURL_ERROR="превышено время ожидания (${HEALTH_TIMEOUT_SEC} с)" ;;
      35|60) CURL_ERROR="ошибка TLS: $(tr -d '\n' < "${WORK_DIR}/curl.err")" ;;
      *)     CURL_ERROR="$(tr '\n' ' ' < "${WORK_DIR}/curl.err" | cut -c1-200)" ;;
    esac
    [ -n "$CURL_ERROR" ] || CURL_ERROR="curl завершился с кодом ${rc}"
    HTTP_CODE="000"
    return 0
  fi
  CURL_ERROR=""
  return 0
}

check_site() {
  local body="${WORK_DIR}/site.html"
  http_request "$body" -L "${BASIC_AUTH_ARGS[@]}" "$HEALTH_CHECK_URL/"
  if [ "$HTTP_CODE" = "000" ]; then
    SITE_RESULT="fail"; SITE_NOTE="$CURL_ERROR"
    return
  fi
  if [ "$HTTP_CODE" != "200" ]; then
    SITE_RESULT="fail"; SITE_NOTE="HTTP ${HTTP_CODE}"
    return
  fi
  # Пустой ответ с кодом 200 отдаёт nginx, у которого не собрался фронт: код
  # верный, страницы нет. Для человека это тот же отказ.
  if [ ! -s "$body" ]; then
    SITE_RESULT="fail"; SITE_NOTE="HTTP 200, но страница пустая"
    return
  fi
  SITE_RESULT="ok"; SITE_NOTE="HTTP 200"
}

# maintenance_enabled отвечает "yes", только если система прямо сказала, что идут
# объявленные работы. Ошибку и молчание считаем "нет": режим техработ снимает
# тревогу, и принимать за него неразобранный ответ значило бы глушить сигнал.
maintenance_enabled() {
  local body="${WORK_DIR}/maint.json"
  http_request "$body" "${BASIC_AUTH_ARGS[@]}" "$HEALTH_CHECK_URL/api/settings/maintenance"
  [ "$HTTP_CODE" = "200" ] || { echo "no"; return; }
  if grep -q '"enabled":[[:space:]]*true' "$body"; then echo "yes"; else echo "no"; fi
}

# check_login проходит вход целиком: получает токен и тут же выходит. Выход не
# вежливость, а уборка - каждый вход кладёт строку в refresh_tokens, и проверка
# раз в пять минут оставляла бы почти три сотни строк в сутки. Заодно выход
# доказывает, что выданный токен принимается: /api/logout стоит за проверкой
# подписи. Он же выбран намеренно - это один из немногих методов, пропускаемых
# и гейтом согласия, и требованием сменить пароль (см. PDConsentWhitelist и
# MustChangePasswordWhitelist), поэтому состояние проверяющей учётной записи не
# превратит уборку в ложную тревогу.
check_login() {
  if [ -z "$HEALTH_CHECK_USERNAME" ] || [ -z "$HEALTH_CHECK_PASSWORD" ]; then
    LOGIN_RESULT="skip"
    LOGIN_NOTE="не настроена: пусты HEALTH_CHECK_USERNAME/HEALTH_CHECK_PASSWORD"
    return
  fi

  local body="${WORK_DIR}/login.json" jar="${WORK_DIR}/cookies" token
  # Кавычка и обратная косая в пароле обязаны быть экранированы: иначе тело
  # запроса перестаёт быть разбираемым JSON, система отвечает 400, и проверка
  # объявляет отказ там, где сломан всего лишь её собственный параметр.
  local user_json pass_json
  user_json="$(json_escape "$HEALTH_CHECK_USERNAME")"
  pass_json="$(json_escape "$HEALTH_CHECK_PASSWORD")"
  # Пароль уходит файлом через --data @, а не аргументом командной строки:
  # аргументы видны всем в выводе ps.
  printf '{"username":"%s","password":"%s"}' "$user_json" "$pass_json" \
    > "${WORK_DIR}/login.req"

  http_request "$body" -c "$jar" \
    -H 'Content-Type: application/json' \
    --data "@${WORK_DIR}/login.req" \
    "${BASIC_AUTH_ARGS[@]}" \
    "$HEALTH_CHECK_URL/api/login"

  if [ "$HTTP_CODE" = "000" ]; then
    LOGIN_RESULT="fail"; LOGIN_NOTE="$CURL_ERROR"
    return
  fi
  if [ "$HTTP_CODE" != "200" ]; then
    # Ответ системы приводим целиком: 401 у проверяющей учётной записи означает
    # смену пароля, а 503 - техработы, и по одному коду их не различить.
    local msg
    msg="$(grep -o '"error":"[^"]*"' "$body" | head -1 | cut -d'"' -f4 || true)"
    LOGIN_RESULT="fail"
    LOGIN_NOTE="HTTP ${HTTP_CODE}${msg:+ - ${msg}}"
    return
  fi

  token="$(grep -o '"token":"[^"]*"' "$body" | head -1 | cut -d'"' -f4 || true)"
  if [ -z "$token" ]; then
    LOGIN_RESULT="fail"; LOGIN_NOTE="HTTP 200, но токен не выдан"
    return
  fi

  LOGIN_RESULT="ok"; LOGIN_NOTE="вход выполнен, токен получен"

  # Базовая авторизация и токен системы делят один заголовок Authorization, и
  # второй затирает первый: за такой дверью выход вернул бы 401 от nginx, а не от
  # системы. Убирать нечего - вход прошёл, отвечает система. Оставляем сессию и
  # говорим об этом: на закрытом стенде строки refresh_tokens будут копиться.
  if [ ${#BASIC_AUTH_ARGS[@]} -gt 0 ]; then
    log "выход не выполняется: адрес закрыт базовой авторизацией, она занимает тот же заголовок, что и токен"
    return
  fi

  # Неудачный выход не делает вход отказом: человек в этот момент работать может.
  # Но и молчать нельзя - строки refresh_tokens начнут копиться.
  http_request "${WORK_DIR}/logout.json" -X POST -b "$jar" \
    -H "Authorization: Bearer ${token}" \
    "$HEALTH_CHECK_URL/api/logout"
  if [ "$HTTP_CODE" != "200" ]; then
    log "ПРЕДУПРЕЖДЕНИЕ: вход прошёл, а выход вернул ${HTTP_CODE}${CURL_ERROR:+ (${CURL_ERROR})} - сессия осталась в refresh_tokens"
  fi
}

# check_db отвечает на вопрос "база вообще жива", отдельно от входа. Разделение
# нужно ради разбора: вход упал и база молчит - причина одна, вход упал при живой
# базе - совсем другая, и в письме это разные советы.
check_db() {
  case "$HEALTH_CHECK_DB" in
    off)
      DB_RESULT="skip"; DB_NOTE="выключена параметром HEALTH_CHECK_DB=off"
      return
      ;;
    auto|on) ;;
    *)
      DB_RESULT="skip"; DB_NOTE="непонятное значение HEALTH_CHECK_DB=${HEALTH_CHECK_DB}"
      return
      ;;
  esac

  # Стек может стоять не на той машине, откуда идёт проверка. Тогда база
  # недоступна отсюда по устройству, а не по аварии, и объявлять это отказом -
  # значит звать человека к исправной системе. Отличаем одно от другого до
  # запроса: если docker не описывает службу db, проверять нечего.
  if [ "$HEALTH_CHECK_DB" = "auto" ]; then
    if ! command -v docker >/dev/null 2>&1; then
      DB_RESULT="skip"; DB_NOTE="docker не установлен на этой машине"
      return
    fi
    # Перечень служб сначала забирается целиком, и только потом ищется db.
    # Через `... | grep -qx db` проверка работала через раз: grep выходит по
    # первому совпадению, docker получает SIGPIPE, а pipefail поднимает его код
    # наверх - и живая база превращалась в «стек не описан на этой машине».
    local services
    services="$("${COMPOSE[@]}" config --services 2>/dev/null || true)"
    case $'\n'"${services}"$'\n' in
      *$'\n'db$'\n'*) ;;
      *)
        DB_RESULT="skip"; DB_NOTE="стек ${ENVIRONMENT} не описан на этой машине"
        return
        ;;
    esac
  fi

  local answer rc=0
  # SELECT 1, а не pg_isready: приём подключений и способность отвечать на запрос -
  # разные вещи. База с кончившимся диском подключение принимает.
  answer="$("${COMPOSE[@]}" exec -T db psql -U "$DB_USER" -d "$DB_NAME" -tAc 'SELECT 1' \
    2>"${WORK_DIR}/db.err")" || rc=$?
  if [ "$rc" -ne 0 ]; then
    DB_RESULT="fail"
    DB_NOTE="$(tr '\n' ' ' < "${WORK_DIR}/db.err" | sed 's/  */ /g; s/^ *//; s/ *$//' | cut -c1-200)"
    [ -n "$DB_NOTE" ] || DB_NOTE="запрос завершился с кодом ${rc}"
    return
  fi
  if [ "$(echo "$answer" | tr -d '[:space:]')" != "1" ]; then
    DB_RESULT="fail"; DB_NOTE="на SELECT 1 пришёл неожиданный ответ"
    return
  fi
  DB_RESULT="ok"; DB_NOTE="отвечает"
}

# --- Разбор причины ----------------------------------------------------------

# probable_cause складывает три ответа в одну фразу. Смысл в том, чтобы человек,
# открывший письмо ночью, начал с нужного места, а не с перебора всего подряд.
probable_cause() {
  if [ "$DB_RESULT" = "fail" ]; then
    if [ "$SITE_RESULT" = "ok" ]; then
      echo "Страница открывается, а база не отвечает - вход упирается именно в неё. Начните с базы."
    else
      echo "Не отвечает ни сайт, ни база. Похоже, лежит весь стек - начните с базы, остальное могло встать следом."
    fi
    return
  fi
  if [ "$SITE_RESULT" = "ok" ] && [ "$LOGIN_RESULT" = "fail" ]; then
    echo "Сайт открывается, но войти нельзя - это тот самый отказ, которого не видит проверка живости. Смотрите приложение и журнал входов."
    return
  fi
  if [ "$SITE_RESULT" = "fail" ] && [ "$LOGIN_RESULT" = "ok" ]; then
    echo "API работает, а внешний адрес страницу не отдаёт - смотрите nginx и сборку фронта."
    return
  fi
  if [ "$SITE_RESULT" = "fail" ] && [ "$DB_RESULT" = "ok" ]; then
    echo "База отвечает, а снаружи система недоступна - смотрите приложение и nginx перед ним."
    return
  fi
  echo "Причина по ответам проб не сужается - смотрите журнал служб целиком."
}

human_duration() {
  local sec="$1"
  if [ "$sec" -lt 60 ]; then
    echo "${sec} с"
  elif [ "$sec" -lt 3600 ]; then
    echo "$(( sec / 60 )) мин"
  else
    echo "$(( sec / 3600 )) ч $(( (sec % 3600) / 60 )) мин"
  fi
}

# --- Состояние и письма ------------------------------------------------------

# Состояние лежит файлом на диске той машины, откуда идёт проверка, и это выбор,
# а не удобство. Держать его в базе нельзя: база - одно из того, что проверяется,
# и в аварию защита от повторов оказалась бы недоступна ровно тогда, когда она
# работает. Файл переживает перезагрузку (в отличие от памяти процесса) и
# читается глазами во время разбора: cat state.json.
read_state() {
  PREV_STATE="unknown"; PREV_SINCE=""; PREV_ALERT_AT=""
  [ -f "$STATE_FILE" ] || return 0
  # Подавление здесь не прячет сбой, а наоборот: grep, не нашедший поля, вернул бы
  # единицу, pipefail поднял бы её наверх и set -e убил бы разбор на полуслове.
  # Файл может быть оборван записью на кончившемся диске - то есть ровно тогда,
  # когда его и читают. Пустое значение ниже честно означает "прежнего нет".
  PREV_STATE="$(grep -o '"state": *"[^"]*"' "$STATE_FILE" | cut -d'"' -f4 || true)"
  PREV_SINCE="$(grep -o '"since": *"[^"]*"' "$STATE_FILE" | cut -d'"' -f4 || true)"
  PREV_ALERT_AT="$(grep -o '"last_alert_at": *"[^"]*"' "$STATE_FILE" | cut -d'"' -f4 || true)"
  [ -n "$PREV_STATE" ] || PREV_STATE="unknown"
}

write_state() {
  local state="$1" since="$2" alert_at="$3" reason="$4"
  cat > "$STATE_FILE" <<EOF
{
  "checked_at": "${NOW_ISO}",
  "environment": "${ENVIRONMENT}",
  "state": "${state}",
  "since": "${since}",
  "last_alert_at": "${alert_at}",
  "reason": "${reason//\"/\'}",
  "site": "${SITE_RESULT}",
  "login": "${LOGIN_RESULT}",
  "db": "${DB_RESULT}"
}
EOF
}

mail_configured() {
  [ -n "${SMTP_HOST:-}" ] && [ -n "$HEALTH_ALERT_TO" ]
}

# mime_header кодирует кириллицу в заголовке. Без этого тема письма доезжает
# набором знаков вопроса: заголовки ходят в ASCII, восьмибитные байты в них
# каждый сервер трактует по-своему.
mime_header() {
  printf '=?UTF-8?B?%s?=' "$(printf '%s' "$1" | base64 -w0)"
}

# send_mail отправляет письмо напрямую по SMTP через curl. Почтовый слой системы
# здесь не используется намеренно: если отказала система, отказала и её почта.
# Возвращает ненулевой код, когда письмо не ушло - вызывающий обязан это заметить
# и не записать в состояние, будто человека уже позвали.
SEND_ERROR=""
send_mail() {
  local subject="$1" body="$2"
  local msg="${WORK_DIR}/message.eml"
  local from_name="${SMTP_FROM_NAME:-Бюро пропусков}"
  local from_addr="${SMTP_FROM:-}"
  local domain="${from_addr#*@}"

  if [ -z "$from_addr" ]; then
    SEND_ERROR="не задан SMTP_FROM"
    return 1
  fi

  local rcpt_header url
  # Пробелы снимаются до расстановки: "a@b, c@d" в файле параметров иначе
  # превратилось бы в двойной пробел в заголовке письма.
  rcpt_header="${HEALTH_ALERT_TO// /}"
  rcpt_header="${rcpt_header//,/, }"
  case "${SMTP_TLS_MODE:-starttls}" in
    tls)      url="smtps://${SMTP_HOST}:${SMTP_PORT:-465}" ;;
    starttls) url="smtp://${SMTP_HOST}:${SMTP_PORT:-587}" ;;
    none)     url="smtp://${SMTP_HOST}:${SMTP_PORT:-25}" ;;
    *)
      SEND_ERROR="непонятный SMTP_TLS_MODE=${SMTP_TLS_MODE}"
      return 1
      ;;
  esac

  {
    printf 'From: %s <%s>\r\n' "$(mime_header "$from_name")" "$from_addr"
    printf 'To: %s\r\n' "$rcpt_header"
    printf 'Subject: %s\r\n' "$(mime_header "$subject")"
    printf 'Date: %s\r\n' "$(date -R)"
    printf 'Message-ID: <health-%s-%s@%s>\r\n' "$(date +%s)" "$$" "${domain:-localhost}"
    printf 'Auto-Submitted: auto-generated\r\n'
    printf 'MIME-Version: 1.0\r\n'
    printf 'Content-Type: text/plain; charset=utf-8\r\n'
    # Тело идёт base64: восьмибитная кириллица требует от сервера расширения
    # 8BITMIME, а длинные строки он вправе переносить сам, ломая текст.
    printf 'Content-Transfer-Encoding: base64\r\n'
    printf '\r\n'
    printf '%s' "$body" | base64 -w76 | sed 's/$/\r/'
  } > "$msg"

  local args=(--silent --show-error --url "$url"
    --mail-from "$from_addr" --upload-file "$msg"
    --max-time "$HEALTH_TIMEOUT_SEC")
  # none - сервер в закрытом контуре, где шифрование обеспечивает сама сеть.
  # Требовать TLS там значит не отправить письмо вовсе.
  if [ "${SMTP_TLS_MODE:-starttls}" != "none" ]; then
    args+=(--ssl-reqd)
  fi
  if [ -n "${SMTP_USERNAME:-}" ]; then
    args+=(--user "${SMTP_USERNAME}:${SMTP_PASSWORD:-}")
  fi
  # Перевод строки в конце обязателен: без него read не отдаёт последнюю строку
  # (возвращает единицу на конце файла, и тело цикла для неё не выполняется).
  # На списке из двух адресов это молча теряло второго дежурного.
  local addr
  while IFS= read -r addr; do
    addr="$(printf '%s' "$addr" | tr -d '[:space:]')"
    if [ -n "$addr" ]; then
      args+=(--mail-rcpt "$addr")
    fi
  done < <(printf '%s\n' "$HEALTH_ALERT_TO" | tr ',' '\n')

  local rc=0
  curl "${args[@]}" 2>"${WORK_DIR}/mail.err" || rc=$?
  if [ "$rc" -ne 0 ]; then
    SEND_ERROR="$(tr '\n' ' ' < "${WORK_DIR}/mail.err" | cut -c1-300)"
    [ -n "$SEND_ERROR" ] || SEND_ERROR="curl завершился с кодом ${rc}"
    return 1
  fi
  SEND_ERROR=""
  return 0
}

compose_command() {
  local c="" part
  for part in "${COMPOSE[@]}"; do c="${c}${part} "; done
  echo "${c% }"
}

failure_letter() {
  local since_human="$1"
  cat <<EOF
Проверка живости не прошла: система не отвечает так, как её видит человек.

Адрес системы:  ${HEALTH_CHECK_URL}
Стенд:          ${ENVIRONMENT}
Сервер:         $(hostname)
Проверено:      $(date '+%d.%m.%Y %H:%M:%S')
Отказ длится с: ${since_human}

Что проверялось:
  сайт по внешнему адресу  - $(verdict_word "$SITE_RESULT"): ${SITE_NOTE}
  вход учётной записью     - $(verdict_word "$LOGIN_RESULT"): ${LOGIN_NOTE}
  база данных              - $(verdict_word "$DB_RESULT"): ${DB_NOTE}

$(probable_cause)

Что посмотреть на сервере:
  systemctl status systemburo-health.timer
  journalctl -u systemburo-health -n 50
  $(compose_command) ps
  tail -n 50 ${LOG_FILE}

Письмо отправлено напрямую по SMTP, минуя почтовый слой системы: он часть того,
что отказало, и через него это письмо не ушло бы.
${REPEAT_NOTE}
EOF
}

recovery_letter() {
  local downtime="$1" since_machine="$2" since_rfc="$3"
  cat <<EOF
Система снова отвечает.

Адрес системы:  ${HEALTH_CHECK_URL}
Стенд:          ${ENVIRONMENT}
Сервер:         $(hostname)
Восстановлено:  $(date '+%d.%m.%Y %H:%M:%S')
Отказ длился:   ${downtime}

Что отвечает сейчас:
  сайт по внешнему адресу  - $(verdict_word "$SITE_RESULT"): ${SITE_NOTE}
  вход учётной записью     - $(verdict_word "$LOGIN_RESULT"): ${LOGIN_NOTE}
  база данных              - $(verdict_word "$DB_RESULT"): ${DB_NOTE}

Проверка не знает, почему система отказала и почему поднялась. Причину смотрите
в журналах за время отказа:
  journalctl --since '${since_machine}'
  $(compose_command) logs --since ${since_rfc}
EOF
}

broken_letter() {
  cat <<EOF
Проверка живости не смогла выполниться.

Это не значит, что система в порядке, - это значит, что о ней сейчас ничего не
известно. Пока проверка сломана, отказ останется незамеченным.

Стенд:    ${ENVIRONMENT}
Сервер:   $(hostname)
Время:    $(date '+%d.%m.%Y %H:%M:%S')
Причина:  ${BROKEN_REASON}

Что посмотреть:
  journalctl -u systemburo-health -n 50
  bash -n $(pwd)/scripts/health-check.sh
  tail -n 50 ${LOG_FILE}
EOF
}

verdict_word() {
  case "$1" in
    ok)   echo "в порядке" ;;
    fail) echo "ОТКАЗ" ;;
    *)    echo "не проверялось" ;;
  esac
}

# finish_state сводит вердикт с прежним состоянием и решает, звать ли человека.
# Правило простое: письмо уходит на смене состояния (упало, поднялось) и потом
# напоминанием не чаще HEALTH_ALERT_REPEAT_MIN. Письмо о восстановлении
# обязательно - без него человек не знает, кончилась авария или он просто перестал
# получать письма.
finish_state() {
  local state="$1" reason="$2"
  read_state

  local since="$PREV_SINCE" alert_at="$PREV_ALERT_AT"
  local send_kind=""

  if [ "$state" = "ok" ] || [ "$state" = "maintenance" ]; then
    if [ "$PREV_STATE" = "fail" ] || [ "$PREV_STATE" = "broken" ]; then
      send_kind="recovery"
    fi
    since="$NOW_ISO"; alert_at=""
  else
    if [ "$PREV_STATE" != "$state" ]; then
      # Смена состояния: и первый отказ, и переход "отказ -> проверка сломалась"
      # (о нём человек должен узнать отдельно - это другая беда).
      send_kind="alert"
      since="$NOW_ISO"
    else
      [ -n "$since" ] || since="$NOW_ISO"
      if [ -z "$alert_at" ]; then
        # Об этом отказе ещё ни одно письмо не дошло: первая попытка сорвалась
        # (почтовый сервер отверг, сеть моргнула). Пробуем снова каждый запуск,
        # пока не выйдет. Без этой ветки одна неудачная отправка в момент аварии
        # означала бы тишину до самого конца отказа - и человек не узнал бы
        # ничего, хотя проверка всё это время честно видела отказ.
        send_kind="alert"
      elif [ "${HEALTH_ALERT_REPEAT_MIN}" -gt 0 ]; then
        local elapsed
        elapsed=$(( $(date +%s) - $(date -d "$alert_at" +%s 2>/dev/null || echo 0) ))
        if [ "$elapsed" -ge $(( HEALTH_ALERT_REPEAT_MIN * 60 )) ]; then
          send_kind="alert"
        fi
      fi
    fi
  fi

  REPEAT_NOTE=""
  if [ "${HEALTH_ALERT_REPEAT_MIN}" -gt 0 ]; then
    REPEAT_NOTE="Пока отказ длится, напоминание придёт раз в ${HEALTH_ALERT_REPEAT_MIN} мин. О восстановлении письмо придёт само."
  else
    REPEAT_NOTE="Напоминания выключены (HEALTH_ALERT_REPEAT_MIN=0). О восстановлении письмо придёт само."
  fi

  if [ -z "$send_kind" ]; then
    write_state "$state" "$since" "$alert_at" "$reason"
    return 0
  fi

  if ! mail_configured; then
    # Молчать здесь нельзя: без этой строки ненастроенная почта выглядит как
    # тишина исправной системы. Ровно тот случай, ради которого правило и есть.
    local why="SMTP_HOST не задан"
    [ -n "${SMTP_HOST:-}" ] && why="не задан HEALTH_ALERT_TO"
    log "ПИСЬМО НЕ ОТПРАВЛЕНО (${why}): ${reason}"
    write_state "$state" "$since" "$alert_at" "$reason"
    return 0
  fi

  local subject body
  case "$send_kind" in
    recovery)
      # Время начала отказа идёт в письмо и словами, и в том виде, в каком его
      # примут journalctl и docker: строка "1 ч 20 мин ago" в команду не
      # подставляется, а команда из письма должна работать копированием.
      local downtime="неизвестно" since_machine="" since_rfc=""
      if [ -n "$PREV_SINCE" ]; then
        downtime="$(human_duration $(( $(date +%s) - $(date -d "$PREV_SINCE" +%s 2>/dev/null || date +%s) )))"
        since_machine="$(date -d "$PREV_SINCE" '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "$PREV_SINCE")"
        since_rfc="$(date -d "$PREV_SINCE" '+%Y-%m-%dT%H:%M:%S' 2>/dev/null || echo "$PREV_SINCE")"
      fi
      subject="Бюро пропусков (${ENVIRONMENT}): система снова отвечает"
      body="$(recovery_letter "$downtime" "$since_machine" "$since_rfc")"
      ;;
    alert)
      if [ "$state" = "broken" ]; then
        subject="Бюро пропусков (${ENVIRONMENT}): проверка живости сломана"
        body="$(broken_letter)"
      else
        local since_human
        since_human="$(date -d "$since" '+%d.%m.%Y %H:%M:%S' 2>/dev/null || echo "$since")"
        subject="Бюро пропусков (${ENVIRONMENT}): система не отвечает"
        body="$(failure_letter "$since_human")"
      fi
      ;;
  esac

  if send_mail "$subject" "$body"; then
    log "письмо отправлено (${send_kind}) на ${HEALTH_ALERT_TO}"
    alert_at="$NOW_ISO"
  else
    # last_alert_at не двигаем намеренно: иначе неотправленное письмо считалось
    # бы доставленным, и следующая попытка отложилась бы на час.
    log "ПИСЬМО НЕ ОТПРАВЛЕНО: ${SEND_ERROR}"
    write_state "$state" "$since" "$alert_at" "$reason"
    return 1
  fi

  write_state "$state" "$since" "$alert_at" "$reason"
  return 0
}

# --- Ход проверки ------------------------------------------------------------

trap 'on_error $LINENO' ERR

if [ -z "$HEALTH_CHECK_URL" ]; then
  BROKEN_REASON="не задан HEALTH_CHECK_URL - проверять нечего"
  log "СБОЙ ПРОВЕРКИ: ${BROKEN_REASON}"
  trap - ERR
  finish_state "broken" "$BROKEN_REASON" || true
  cat >&2 <<'EOF'

Проверка не выполнена: не задан внешний адрес системы.

Проверка ходит тем же путём, что и человек, - через внешний адрес, а не через
localhost. Иначе она не увидит ни сломанный TLS, ни упавший nginx.

Впишите адрес в .env рядом со сценарием:
  HEALTH_CHECK_URL=https://buro.example.ru

EOF
  exit 2
fi

# Хвост адреса обрезается один раз: иначе из "https://host/" вырастет "https://host//api/login".
HEALTH_CHECK_URL="${HEALTH_CHECK_URL%/}"

check_site

if [ "$SITE_RESULT" = "ok" ] && [ "$(maintenance_enabled)" = "yes" ]; then
  log "техработы объявлены системой - проверка входа пропущена, тревога не поднимается"
  trap - ERR
  finish_state "maintenance" "объявлены технические работы" || exit 2
  exit 0
fi

check_login
check_db

trap - ERR

if [ "$LOGIN_RESULT" = "skip" ]; then
  # Настройчивая строка в каждом запуске - плата за молчаливое согласие обходиться
  # без главной пробы. Без входа проверка возвращается ровно к тому, что уже есть
  # в ручке /health: процесс жив, а пускает ли он людей - неизвестно.
  log "ВНИМАНИЕ: вход не проверяется (${LOGIN_NOTE}); отказ вида «сайт открыт, а войти нельзя» останется незамеченным"
fi

STATE="ok"
REASON=""
if [ "$SITE_RESULT" = "fail" ] || [ "$LOGIN_RESULT" = "fail" ] || [ "$DB_RESULT" = "fail" ]; then
  STATE="fail"
  REASON="сайт: ${SITE_NOTE}; вход: ${LOGIN_NOTE}; база: ${DB_NOTE}"
fi

log "итог: ${STATE} | сайт ${SITE_RESULT} (${SITE_NOTE}) | вход ${LOGIN_RESULT} (${LOGIN_NOTE}) | база ${DB_RESULT} (${DB_NOTE})"

MAIL_FAILED=0
finish_state "$STATE" "$REASON" || MAIL_FAILED=1

if [ "$STATE" = "fail" ]; then
  exit 1
fi
# Неотправленное письмо при исправной системе - тоже повод показать красный юнит:
# о следующем отказе тем же путём не сообщат.
if [ "$MAIL_FAILED" -eq 1 ]; then
  exit 2
fi
exit 0
