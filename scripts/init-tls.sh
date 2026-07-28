#!/bin/bash
set -euo pipefail

# Подготовка TLS-сертификата для production-контура.
#
#   ./scripts/init-tls.sh self   <domain> [доп-имя|IP ...]   локальный УЦ (внутренняя сеть)
#   ./scripts/init-tls.sh acme   <domain> <email>            Let's Encrypt (публичный домен)
#   ./scripts/init-tls.sh import <fullchain.pem> <privkey.pem>  готовый сертификат
#   ./scripts/init-tls.sh renew                              продление ACME + reload nginx
#
# Во всех режимах итог один: nginx/certs/fullchain.pem и nginx/certs/privkey.pem.
# Конфигурация nginx от способа выпуска не зависит.
#
# Порядок при первом запуске: сертификат готовится ДО подъёма контейнеров - nginx
# с директивой ssl_certificate не стартует, если файла нет.

MODE="${1:-}"
CERT_DIR="nginx/certs"
CA_DIR="${CERT_DIR}/ca"
WEBROOT="nginx/acme-webroot"
COMPOSE="docker compose -f docker-compose.base.yml -f docker-compose.prod.yml"

usage() {
    cat <<'EOF'
Использование:
  ./scripts/init-tls.sh self   <domain> [доп-имя|IP ...]
  ./scripts/init-tls.sh acme   <domain> <email>
  ./scripts/init-tls.sh import <fullchain.pem> <privkey.pem>
  ./scripts/init-tls.sh renew

Режимы:
  self    Создаёт локальный удостоверяющий центр и подписывает им серверный
          сертификат. Для контура внутри сети компании, где публичный ACME
          недоступен. Корневой сертификат УЦ нужно раскатать на рабочие станции.

  acme    Выпускает сертификат Let's Encrypt через HTTP-01. Требует публично
          разрешимого домена и доступного извне порта 80.

  import  Ставит выданный сторонним удостоверяющим центром сертификат.

  renew   Продлевает ACME-сертификат и перезагружает nginx. Для расписания.
EOF
}

require_cmd() {
    if ! command -v "$1" >/dev/null; then
        echo "Ошибка: не найдена команда '$1'. $2" >&2
        exit 1
    fi
}

# Проверяем, что запущены из корня репозитория: пути относительные.
if [[ ! -f "docker-compose.prod.yml" ]]; then
    echo "Ошибка: запускать из корня репозитория (рядом с docker-compose.prod.yml)." >&2
    exit 1
fi

install_cert() {
    local src_chain="$1" src_key="$2"

    mkdir -p "$CERT_DIR"
    cp "$src_chain" "${CERT_DIR}/fullchain.pem"
    cp "$src_key" "${CERT_DIR}/privkey.pem"
    chmod 644 "${CERT_DIR}/fullchain.pem"
    chmod 600 "${CERT_DIR}/privkey.pem"

    # Ключ и сертификат обязаны быть парой, иначе nginx упадёт уже после рестарта,
    # когда разбираться будет некогда.
    local c_mod k_mod
    c_mod=$(openssl x509 -noout -modulus -in "${CERT_DIR}/fullchain.pem" | openssl md5)
    k_mod=$(openssl rsa -noout -modulus -in "${CERT_DIR}/privkey.pem" | openssl md5)
    if [[ "$c_mod" != "$k_mod" ]]; then
        echo "Ошибка: приватный ключ не соответствует сертификату." >&2
        exit 1
    fi
}

show_cert_info() {
    echo ""
    echo "Установлен сертификат:"
    openssl x509 -in "${CERT_DIR}/fullchain.pem" -noout -subject -issuer -dates \
        | sed 's/^/  /'
}

reload_nginx() {
    if $COMPOSE ps --status running nginx 2>/dev/null | grep -q nginx; then
        echo "Перезагружаю конфигурацию nginx..."
        $COMPOSE exec -T nginx nginx -s reload
        echo "Готово."
    else
        echo "Контейнер nginx не запущен - перезагрузка не требуется."
    fi
}

case "$MODE" in
    self)
        DOMAIN="${2:-}"
        if [[ -z "$DOMAIN" ]]; then usage; exit 1; fi
        shift 2
        require_cmd openssl "Установите пакет openssl."

        mkdir -p "$CA_DIR"

        # Корневой сертификат переиспользуем: если его перевыпустить, все машины,
        # куда он уже раскатан, перестанут доверять серверу.
        if [[ -f "${CA_DIR}/ca.crt" && -f "${CA_DIR}/ca.key" ]]; then
            echo "Использую существующий локальный УЦ: ${CA_DIR}/ca.crt"
        else
            echo "Создаю локальный удостоверяющий центр..."
            openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
                -keyout "${CA_DIR}/ca.key" -out "${CA_DIR}/ca.crt" \
                -subj "/C=RU/O=Bureau of Passes/CN=Bureau of Passes Internal CA" \
                -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
                -addext "keyUsage=critical,keyCertSign,cRLSign"
            chmod 600 "${CA_DIR}/ca.key"
        fi

        # SAN обязателен: браузеры давно игнорируют CN. Различаем адрес и имя -
        # во внутренней сети к системе часто обращаются напрямую по IP.
        SAN="DNS:${DOMAIN}"
        for extra in "$@"; do
            if [[ "$extra" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                SAN="${SAN},IP:${extra}"
            else
                SAN="${SAN},DNS:${extra}"
            fi
        done
        echo "Альтернативные имена: ${SAN}"

        TMP_EXT=$(mktemp)
        trap 'rm -f "$TMP_EXT"' EXIT
        cat > "$TMP_EXT" <<EOF
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=${SAN}
EOF

        openssl req -newkey rsa:2048 -sha256 -nodes \
            -keyout "${CA_DIR}/server.key" -out "${CA_DIR}/server.csr" \
            -subj "/C=RU/O=Bureau of Passes/CN=${DOMAIN}"

        # 825 дней - потолок, который принимают браузеры для серверных сертификатов.
        openssl x509 -req -in "${CA_DIR}/server.csr" -sha256 -days 825 \
            -CA "${CA_DIR}/ca.crt" -CAkey "${CA_DIR}/ca.key" -CAcreateserial \
            -extfile "$TMP_EXT" -out "${CA_DIR}/server.crt"

        cat "${CA_DIR}/server.crt" "${CA_DIR}/ca.crt" > "${CA_DIR}/fullchain.pem"
        install_cert "${CA_DIR}/fullchain.pem" "${CA_DIR}/server.key"
        show_cert_info

        cat <<EOF

ОБЯЗАТЕЛЬНЫЙ следующий шаг: раскатать корневой сертификат на рабочие станции.

  Файл: ${CA_DIR}/ca.crt

Система отдаёт заголовок HSTS, а он запрещает браузеру кнопку "всё равно перейти"
на недоверенном сертификате. Пока корневой сертификат не установлен в доверенные,
пользователи получат ошибку без возможности её обойти.

  Windows в домене: групповая политика Computer Configuration -> Policies ->
    Windows Settings -> Security Settings -> Public Key Policies ->
    Trusted Root Certification Authorities -> импорт ca.crt
  Windows разово:   certutil -addstore -f Root ca.crt (от администратора)
  Linux:            копировать в /usr/local/share/ca-certificates/, update-ca-certificates

Приватный ключ УЦ ${CA_DIR}/ca.key даёт возможность выпустить сертификат на любое
имя. Хранить как секрет, за пределами рабочих станций.
EOF
        ;;

    acme)
        DOMAIN="${2:-}"
        EMAIL="${3:-}"
        if [[ -z "$DOMAIN" || -z "$EMAIL" ]]; then usage; exit 1; fi
        require_cmd certbot "Установите certbot: apt install certbot (Debian/Ubuntu) или dnf install certbot."

        mkdir -p "$WEBROOT"

        # HTTP-01 требует работающего ответчика на 80. Если nginx ещё не поднят,
        # поднять его нечем: без сертификата он не стартует. Поэтому сначала
        # заглушка через режим self, затем acme поверх.
        if [[ ! -f "${CERT_DIR}/fullchain.pem" ]]; then
            echo "Сертификата ещё нет, а nginx без него не стартует."
            echo "Сначала выполните: ./scripts/init-tls.sh self ${DOMAIN}"
            echo "затем поднимите контур и повторите этот режим."
            exit 1
        fi

        certbot certonly --webroot -w "$WEBROOT" -d "$DOMAIN" \
            --email "$EMAIL" --agree-tos --non-interactive

        install_cert "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" \
                     "/etc/letsencrypt/live/${DOMAIN}/privkey.pem"
        show_cert_info
        reload_nginx

        cat <<EOF

Настройте автоматическое продление - сертификат действует 90 дней.
Задание в cron (например, в 03:30 ежедневно, certbot сам решит, пора ли):

  30 3 * * * cd $(pwd) && ./scripts/init-tls.sh renew >> /var/log/systemburo-tls.log 2>&1
EOF
        ;;

    import)
        SRC_CHAIN="${2:-}"
        SRC_KEY="${3:-}"
        if [[ -z "$SRC_CHAIN" || -z "$SRC_KEY" ]]; then usage; exit 1; fi
        require_cmd openssl "Установите пакет openssl."

        for f in "$SRC_CHAIN" "$SRC_KEY"; do
            if [[ ! -f "$f" ]]; then
                echo "Ошибка: файл не найден: $f" >&2
                exit 1
            fi
        done

        install_cert "$SRC_CHAIN" "$SRC_KEY"
        show_cert_info
        reload_nginx
        ;;

    renew)
        require_cmd certbot "Установите certbot."

        certbot renew --webroot -w "$WEBROOT" --quiet

        # certbot при отсутствии обновлений завершается успешно и ничего не меняет,
        # поэтому копируем и перезагружаем только когда файл действительно новее.
        LIVE_DIR=$(find /etc/letsencrypt/live -mindepth 1 -maxdepth 1 -type d | head -1)
        if [[ -z "$LIVE_DIR" ]]; then
            echo "Ошибка: не найден каталог /etc/letsencrypt/live/<domain>." >&2
            exit 1
        fi

        if [[ "${LIVE_DIR}/fullchain.pem" -nt "${CERT_DIR}/fullchain.pem" ]]; then
            echo "Сертификат обновлён, устанавливаю."
            install_cert "${LIVE_DIR}/fullchain.pem" "${LIVE_DIR}/privkey.pem"
            show_cert_info
            reload_nginx
        else
            echo "Продление не требуется, сертификат актуален."
        fi
        ;;

    *)
        usage
        exit 1
        ;;
esac
