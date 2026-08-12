#!/bin/bash
set -euo pipefail

# Генерация .env для staging/production
# Использование: ./scripts/init-env.sh <staging|production> <domain>
# Пример:       ./scripts/init-env.sh staging stagingburo.washka17.ru
#
# Замысел: заказчик получает готовый файл и вписывает руками только то, что
# нельзя вычислить на месте - домен и ключи внешних служб. Значения берутся из
# internal/config/config.go: расхождение с ним означает, что система работает не
# так, как обещает файл параметров.

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

# В файле пароль базы, оба ключа подписи токенов и ключ шифрования персональных
# данных. umask ставится до создания, чтобы файл не успел побыть общедоступным даже
# на время записи; chmod ниже закрепляет права, если umask перекрыт извне.
umask 077

# --- Генерация секретов ---
DB_PASSWORD=$(openssl rand -base64 24 | tr -d '/+=')
JWT_SECRET=$(openssl rand -base64 48 | tr -d '/+=')
JWT_REFRESH_SECRET=$(openssl rand -base64 48 | tr -d '/+=')
DATA_ENCRYPTION_KEY=$(openssl rand -hex 32)

# Пара ключей шифрования файлового архива бланков. Формат age X25519: открытая
# часть вида age1..., закрытая AGE-SECRET-KEY-1... . На сервере age обычно не
# установлен, поэтому вторым заходом берём его одноразовым контейнером - тем же
# способом, что и scripts/backup.sh.
# Глушится только вывод самой age-keygen: она печатает открытый ключ отдельной
# строкой в поток ошибок, а он и так уходит в файл. Сбои docker и apk через этот
# редирект не проходят и остаются видны; пустой результат разбирается ниже.
generate_age_keypair() {
    if command -v age-keygen >/dev/null 2>&1; then
        age-keygen 2>/dev/null
        return
    fi
    if command -v docker >/dev/null 2>&1; then
        docker run --rm alpine:3.20 sh -c 'apk add --no-cache -q age && age-keygen 2>/dev/null'
        return
    fi
    return 1
}

# Готовые ключи можно подложить через окружение: у внешней стороны, которая читает
# бланки, свой ключ, и он приходит извне, а не создаётся здесь.
ARCHIVE_AGE_RECIPIENT="${ARCHIVE_AGE_RECIPIENT:-}"
ARCHIVE_AGE_IDENTITY="${ARCHIVE_AGE_IDENTITY:-}"
if [[ -z "$ARCHIVE_AGE_IDENTITY" ]]; then
    if ! AGE_KEYPAIR="$(generate_age_keypair)"; then
        AGE_KEYPAIR=""
    fi
    # Разбор только через sed: он отдаёт ноль и когда ничего не нашёл. grep на
    # пустом вводе возвращает единицу, а под set -e с pipefail это обрывало
    # скрипт прямо здесь - ровно там, где ключи создать не удалось и ниже ждёт
    # разбор причины. Оператор не видел ни строчки объяснения. Команда q в конце
    # избавляет от head и от закрытой трубы под тем же pipefail.
    ARCHIVE_AGE_IDENTITY="$(printf '%s\n' "$AGE_KEYPAIR" | sed -n '/^AGE-SECRET-KEY-/{p;q;}')"
    if [[ -z "$ARCHIVE_AGE_RECIPIENT" ]]; then
        ARCHIVE_AGE_RECIPIENT="$(printf '%s\n' "$AGE_KEYPAIR" | sed -n '/^# public key: /{s/^# public key: //;p;q;}')"
    fi
fi

if [[ -z "$ARCHIVE_AGE_RECIPIENT" || -z "$ARCHIVE_AGE_IDENTITY" ]]; then
    if [[ "$ENV" == "production" ]]; then
        cat >&2 <<EOM

Не удалось создать пару ключей шифрования файлового архива. Файл параметров не создан.

Прод-профиль включает REQUIRE_ENCRYPTION=true, и с ним система не стартует, пока
ключи не заданы: в бланках заявок ФИО, паспортные данные и номера патентов, и
ложиться на диск открытым текстом они не должны.

Исправить одним из трёх способов.

1. Поставить age и повторить запуск:
     apt install age
2. Дать docker доступ в сеть - без age на сервере ключи создаются одноразовым
   контейнером alpine.
3. Подложить готовые ключи и повторить запуск:
     ARCHIVE_AGE_RECIPIENT=age1... ARCHIVE_AGE_IDENTITY=AGE-SECRET-KEY-1... $0 $ENV $DOMAIN

EOM
        exit 1
    fi
    echo "ВНИМАНИЕ: ключи шифрования файлового архива не созданы (нет age и docker)." >&2
    echo "Бланки на стенде будут записываться открытым текстом." >&2
fi

# Параметры по окружению
if [[ "$ENV" == "staging" ]]; then
    LOG_LEVEL="debug"
    # Стенд разворачивается в каталоге установки: отдельный раздел под архив там
    # не выделяют, а данные на нём демонстрационные.
    ARCHIVE_HOST_PATH="./archive"
    ENTITY_EXPORT_HOST_PATH="./entity-export"
    REQUIRE_ENCRYPTION="false"
    PGADMIN_PASSWORD=$(openssl rand -base64 12 | tr -d '/+=')
    BASIC_AUTH_USER="admin"
    BASIC_AUTH_PASS=$(openssl rand -base64 12 | tr -d '/+=')
else
    LOG_LEVEL="info"
    # Отдельный каталог вне установки: раздел под ним можно зашифровать, отдать
    # только на чтение и включить в резервное копирование. Подробности в
    # docs/DEPLOYMENT.md, подраздел про файловый архив бланков.
    ARCHIVE_HOST_PATH="/srv/systemburo/archive"
    ENTITY_EXPORT_HOST_PATH="/srv/systemburo/entity-export"
    REQUIRE_ENCRYPTION="true"
fi
CORS_ORIGINS="https://${DOMAIN}"
API_URL="https://${DOMAIN}"

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

# Имя набора контейнеров и томов. Задано явно, чтобы имена не зависели от того,
# в какой каталог склонирован проект: по ним ищут том загрузок при копировании.
# На уже работающей установке менять нельзя - сменится и имя тома с базой.
COMPOSE_PROJECT_NAME=systemburo

# === Go Backend ===
BIND_HOST=0.0.0.0
BIND_PORT=8080
LOG_LEVEL=${LOG_LEVEL}
# Схема API на /swagger/index.html. Наружу не отдаётся: рабочей системе она не
# нужна, а описание всех методов облегчает подбор точек входа.
SWAGGER_ENABLED=false

# === JWT ===
JWT_SECRET=${JWT_SECRET}
JWT_REFRESH_SECRET=${JWT_REFRESH_SECRET}
# Короткий срок у рабочего токена, длинный у обновляющего: клиент продлевает сам.
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h

# === CORS ===
CORS_ALLOWED_ORIGINS=${CORS_ORIGINS}

# === Загрузка файлов ===
UPLOAD_MAX_FILE_SIZE=10485760
UPLOAD_ALLOWED_IMAGE_TYPES=image/jpeg,image/png,image/webp
UPLOAD_ALLOWED_DOC_TYPES=application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
UPLOAD_PATH=/app/uploads

# === Файловый архив бланков ===
# Каталог на хосте, куда складываются заполненные бланки. Обязан лежать ВНЕ
# UPLOAD_PATH: загрузки раздаются без проверки авторизации, а в бланках
# персональные данные. Приложение проверяет это при старте и отказывается
# запускаться при вложенных каталогах.
# Каталог создать до первого запуска, владелец uid и gid 1001 - иначе docker
# создаст его от суперпользователя и служба не сможет туда писать.
ARCHIVE_HOST_PATH=${ARCHIVE_HOST_PATH}
# Каталог пакетов выгрузки по идентификатору сущности. Условия те же, что у
# архива: вне каталога загрузок и с владельцем uid и gid 1001 до первого запуска.
ENTITY_EXPORT_HOST_PATH=${ENTITY_EXPORT_HOST_PATH}
# Открытый ключ той стороны, которая читает бланки. Пока он не известен, здесь
# стоит открытая часть ключа самой системы: файлы лежат зашифрованными и система
# их читает. Когда ключ внешней стороны появится, строку заменить - записанные
# ранее файлы останутся читаемыми.
ARCHIVE_AGE_RECIPIENT=${ARCHIVE_AGE_RECIPIENT}
# Закрытая часть ключа системы: ею отдаются бланки по кнопке в карточке заявки.
# Скопировать в хранилище секретов организации - при её потере зашифрованные
# бланки не прочитает никто.
ARCHIVE_AGE_IDENTITY=${ARCHIVE_AGE_IDENTITY}

# === Шифрование ПД (152-ФЗ) ===
# Ключ шифрования полей базы: 32 байта в hex.
DATA_ENCRYPTION_KEY=${DATA_ENCRYPTION_KEY}
# Требовать шифрования: при true система не стартует с пустым ключом базы или
# пустыми ключами архива. Так проверяется, что данные не легли открытым текстом
# из-за незаполненной строки.
REQUIRE_ENCRYPTION=${REQUIRE_ENCRYPTION}

# === Ограничение частоты запросов ===
RATE_LIMIT_PER_MINUTE=200
RATE_LIMIT_WINDOW_SEC=60

# === Постраничная выдача ===
PAGINATION_MAX_LIMIT=100

# === Уведомления в браузер при закрытом сайте (Web Push) ===
# Пара ключей подписи: сгенерировать командой "make staging-vapid" (или
# "make deploy-vapid") и вставить сюда. Пока обе строки пусты, доставка вне
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
EOF

# pgAdmin и basic-auth есть только в описании стенда: в docker-compose.prod.yml
# ни того, ни другого нет, и в рабочем файле это были бы лишние пароли.
if [[ "$ENV" == "staging" ]]; then
cat >> "$ENV_FILE" <<EOF

# === pgAdmin (только стенд) ===
PGADMIN_EMAIL=admin@${DOMAIN}
PGADMIN_PASSWORD=${PGADMIN_PASSWORD}

# === Basic Auth (только стенд) ===
BASIC_AUTH_USER=${BASIC_AUTH_USER}
BASIC_AUTH_PASS=${BASIC_AUTH_PASS}
EOF
fi

cat >> "$ENV_FILE" <<EOF

# === Frontend ===
VITE_API_BASE_URL=${API_URL}

# === Резервное копирование ===
# Сроки хранения и режим архивации файлов описаны в .env.example.
# Ключ шифрования копий заводится отдельно: age-keygen -o buro-backup.key,
# сюда вписывается ОТКРЫТАЯ часть, закрытая хранится вне сервера.
# Пока строка пуста, копирование останавливается с ошибкой и копию не создаёт:
# в выгрузке базы и архиве бланков персональные данные. Снимать копии без
# шифрования осознанно - BACKUP_ALLOW_UNENCRYPTED=yes.
BACKUP_DIR=/var/backups/systemburo
BACKUP_AGE_RECIPIENT=
BACKUP_ALLOW_UNENCRYPTED=
BACKUP_KEEP_DAILY=7
BACKUP_KEEP_WEEKLY=4
BACKUP_KEEP_MONTHLY=6
BACKUP_UPLOADS_MODE=weekly
BACKUP_S3_REMOTE=
BACKUP_S3_BUCKET=

# =============================================================
# Параметры без выделенного разбора выше
#
# Читаются приложением напрямую и доходят до контейнера через явный список
# docker-compose.base.yml. После разворачивания системы их трогают редко,
# поэтому отдельного раздела с описанием под каждый не заводили. Значения
# ниже совпадают с умолчаниями кода (internal/config/config.go) - строку
# можно менять, правка применяется после перезапуска контейнера backend.
# =============================================================

# Пул соединений с базой (#2027). 50 открытых - треть от max_connections=150
# в docker-compose.prod.yml, 25 простаивающих - половина этого предела.
# Подробный расчёт - в комментарии к DBMaxOpenConns в internal/config/config.go.
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=25
DB_CONN_MAX_LIFETIME=1h
DB_CONN_MAX_IDLE_TIME=10m

# Таймауты HTTP-сервера (#2027). Согласованы с nginx: обычные запросы прокси
# и так обрывает на 60 секундах, три долгих маршрута (SSE, обе выгрузки
# файлового архива) освобождены явно и таймаут им снимает сам обработчик.
HTTP_READ_HEADER_TIMEOUT=10s
HTTP_READ_TIMEOUT=120s
HTTP_WRITE_TIMEOUT=120s
HTTP_IDLE_TIMEOUT=120s

# Число одновременных проверок пароля Argon2id (#2027). 0 - по числу ядер
# (GOMAXPROCS). Каждая проверка держит 19 МБ памяти и ядро целиком, без
# предела утренний вход смены мог бы сам стать поводом положить сервер.
ARGON2_HASH_CONCURRENCY=0

# Срок хранения файлов заявки, размеры и число вложений.
APPLICATION_FILE_MAX_COUNT=30
APPLICATION_FILE_MAX_TOTAL_SIZE=104857600
APPLICATION_FILE_DRAFT_TTL=24h
APPLICATION_FILE_IMAGE_MAX_SIDE=2000
APPLICATION_FILE_JPEG_QUALITY=82

# Ограничение попыток входа с одного адреса.
LOGIN_RATE_LIMIT_MAX=10
LOGIN_RATE_LIMIT_WINDOW_SEC=60

# Признак Secure на обновляющем токене. Снимается только для http на локальной машине.
COOKIE_SECURE=true

# Разбор очереди выгрузки бланков и подметание пропущенных заявок.
ARCHIVE_WORKER_TICK=15s
ARCHIVE_SWEEP_INTERVAL=5m

# Ротация файла журнала приложения. Сам путь задаёт docker-compose.prod.yml.
LOG_MAX_SIZE_MB=100
LOG_MAX_AGE_DAYS=30
LOG_MAX_BACKUPS=14
LOG_COMPRESS=true

# Журнал запросов: сколько суток храним подробно и на сколько создаём разделы вперёд.
REQUEST_LOG_DETAIL_DAYS=30
REQUEST_LOG_PARTITION_PRECREATE_DAYS=7

# Срок хранения аудита обращений к персональным данным, месяцев.
PD_AUDIT_RETENTION_MONTHS=36

# Суточная уборка: отозванные токены, прочитанные и непрочитанные уведомления.
REFRESH_TOKEN_RETENTION_DAYS=30
READ_NOTIFICATION_RETENTION_DAYS=30
NOTIFICATION_RETENTION_DAYS=90

# Часовой пояс для суточного сброса территориальных статусов.
RESET_TIMEZONE=Europe/Moscow

# Обновление кэша аналитики, секунд. Ноль отключает кэш.
ANALYTICS_CACHE_REFRESH_SEC=60

# Бот и чат, куда уходят сообщения об ошибке 500. Пусто - только запись в базу.
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=

# Почтовая рассылка (#1906). Система не поднимает свой почтовый сервер, а
# подключается клиентом к чужому: Джино, Яндекс 360 или сервер организации.
# Пустой SMTP_HOST - штатный режим "почта не настроена": письма не ставятся
# в очередь, а плановая смена паролей отказывается запускаться, вместо того
# чтобы менять пароли в пустоту. Заполнить руками после генерации файла.
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=
SMTP_FROM_NAME="Бюро пропусков"
SMTP_TLS_MODE=starttls
SMTP_TIMEOUT_SEC=15
SMTP_RATE_PER_HOUR=400
MAIL_RETRY_ATTEMPTS=5
MAIL_WORKER_TICK=15s
EOF

chmod 600 "$ENV_FILE"

echo ""
echo "=== .env создан для ${ENV} (${DOMAIN}) ==="
echo "  файл:   $(pwd)/${ENV_FILE}"
echo "  права:  $(stat -c %a "$ENV_FILE") - читает только владелец"
echo ""
# Значения не печатаются намеренно: этот скрипт запускается в том числе по ssh из
# конвейера выпуска, и всё, что он выводит, оседает в журнале запуска и в истории
# терминала - то есть там, где права 600 на файл уже ничего не значат.
echo "Сгенерированы (значения смотреть в самом файле):"
echo "  DB_PASSWORD, JWT_SECRET, JWT_REFRESH_SECRET, DATA_ENCRYPTION_KEY"
if [[ -n "$ARCHIVE_AGE_IDENTITY" ]]; then
    echo "  ARCHIVE_AGE_RECIPIENT, ARCHIVE_AGE_IDENTITY"
fi
if [[ "$ENV" == "staging" ]]; then
    echo "  PGADMIN_PASSWORD, BASIC_AUTH_PASS"
    echo ""
    echo "Доступ к pgAdmin: https://${DOMAIN}/pgadmin, вход admin@${DOMAIN}"
    echo "Пароль:           grep '^PGADMIN_PASSWORD=' ${ENV_FILE}"
    echo "Basic Auth:       grep '^BASIC_AUTH_' ${ENV_FILE}"
fi
echo ""
echo "Каталоги файлового архива и пакетов выгрузки создать до первого запуска:"
echo "  mkdir -p ${ARCHIVE_HOST_PATH} ${ENTITY_EXPORT_HOST_PATH}"
echo "  chown -R 1001:1001 ${ARCHIVE_HOST_PATH} ${ENTITY_EXPORT_HOST_PATH}"
echo "  chmod 750 ${ARCHIVE_HOST_PATH} ${ENTITY_EXPORT_HOST_PATH}"
echo ""
echo "Закрытую часть ключа архива (ARCHIVE_AGE_IDENTITY) скопировать в хранилище"
echo "секретов: при её потере зашифрованные бланки не прочитать."
echo ""
echo "Резервное копирование настраивается отдельно:"
echo "  1. age-keygen -o buro-backup.key   (закрытый ключ унести с сервера!)"
echo "  2. вписать открытую часть в BACKUP_AGE_RECIPIENT в .env"
echo "  3. sudo ./scripts/backup-install.sh ${ENV}"
echo ""
if [[ "$ENV" == "staging" ]]; then
    echo "Запуск: make staging-up"
else
    echo "Запуск: make deploy-up"
fi
