#!/usr/bin/env bash
# Синтетическая проверка только локального изолированного стенда.
set -euo pipefail
: "${STAGING_TEST_BASE_URL:?Нужен URL локального стенда}"
: "${STAGING_TEST_USER:?Нужен синтетический логин}"
: "${STAGING_TEST_PASS:?Нужен синтетический пароль}"
: "${STAGING_TEST_COMPOSE:?Нужен Compose изолированной среды}"
if [[ ! "$STAGING_TEST_BASE_URL" =~ ^http://(127\.0\.0\.1|localhost):([0-9]{1,5})$ ]]; then
    echo 'Проверка разрешена только на localhost с числовым портом' >&2; exit 1
fi
port=$((10#${BASH_REMATCH[2]}))
((port > 0 && port <= 65535)) || { echo 'Недопустимый порт' >&2; exit 1; }
for tool in curl docker openssl env; do command -v "$tool" >/dev/null; done
endpoint=${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}
case "$endpoint" in
    unix://*) ;;
    *) echo 'Для проверки нужен локальный Docker socket' >&2; exit 1 ;;
esac
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
headers="$tmp/headers"
request() {
    code=$(curl -q --noproxy '*' --silent --show-error --max-time 10 --dump-header "$headers" \
        --output /dev/null --write-out '%{http_code}' "$@")
}
expect_code() { [[ "$code" == "$1" ]] || { echo "Ожидался HTTP $1, получен $code (путь $path)" >&2; exit 1; }; }
no_pass() {
    if grep -qi '^Set-Cookie: staging_auth=' "$headers"; then
        echo 'Маршрут не должен выдавать пропуск' >&2; exit 1
    fi
}
pass_value() { sed -n 's/^Set-Cookie: staging_auth=\([^;]*\);.*/\1/ip' "$headers" | tr -d '\r'; }
security_headers() {
    for name in Strict-Transport-Security X-Frame-Options X-Content-Type-Options Referrer-Policy Permissions-Policy Content-Security-Policy; do
        grep -qi "^$name:" "$headers" || { echo "Отсутствует $name" >&2; exit 1; }
    done
}
for path in / /swagger/ /pgadmin/; do
    request "$STAGING_TEST_BASE_URL$path"; expect_code 401; no_pass; security_headers
    request -u "$STAGING_TEST_USER:incorrect" "$STAGING_TEST_BASE_URL$path"; expect_code 401; no_pass; security_headers
done
for path in /swagger /pgadmin; do
    request "$STAGING_TEST_BASE_URL$path"; expect_code 301; no_pass; security_headers
    request -u 'unverified:unverified' "$STAGING_TEST_BASE_URL$path"; expect_code 301; no_pass
done
for path in /health /api/check /api/events; do
    request "$STAGING_TEST_BASE_URL$path"; expect_code 200; no_pass
    request -u 'unverified:unverified' "$STAGING_TEST_BASE_URL$path"; expect_code 200; no_pass
done
legacy=$(printf '%s:%s' "$STAGING_TEST_USER" "$STAGING_TEST_PASS" | openssl dgst -sha256 -hex | awk '{print $NF}')
for rejected in ok unknown "$legacy"; do
    request -H "Cookie: staging_auth=$rejected" "$STAGING_TEST_BASE_URL/"; expect_code 401; no_pass
done
for path in / /swagger/ /pgadmin/; do
    request -u "$STAGING_TEST_USER:$STAGING_TEST_PASS" "$STAGING_TEST_BASE_URL$path"
    expect_code 200; security_headers
    token=$(pass_value)
    [[ "$token" =~ ^[0-9a-f]{64}$ ]] || { echo 'Не выдан новый пропуск' >&2; exit 1; }
    grep -qi 'Path=/; Max-Age=604800; HttpOnly; Secure; SameSite=Strict' "$headers"
    request -H "Cookie: staging_auth=$token" "$STAGING_TEST_BASE_URL$path"; expect_code 200
    [[ "$(pass_value)" == "$token" ]] || { echo 'Пропуск не продлён' >&2; exit 1; }
done
for path in /health /api/check /api/events; do
    request -H "Cookie: staging_auth=$token" "$STAGING_TEST_BASE_URL$path"; expect_code 200; no_pass
done
for path in /status/302 /status/404 /status/500; do
    request -u "$STAGING_TEST_USER:$STAGING_TEST_PASS" "$STAGING_TEST_BASE_URL$path"
    expect_code "${path##*/}"; no_pass; security_headers
done
old="$token"
# Закрепляем проверенный socket: DOCKER_CONTEXT не должен переопределить его.
env -u DOCKER_CONTEXT -u DOCKER_HOST docker --host "$endpoint" compose \
    -f "$STAGING_TEST_COMPOSE" -p systemburo-s7-check restart nginx >/dev/null
ready=false
for ((i=0; i<30; i++)); do
    if curl -q --noproxy '*' -fsS --max-time 2 "$STAGING_TEST_BASE_URL/health" >/dev/null 2>&1; then ready=true; break; fi
    sleep 1
done
[[ "$ready" == true ]] || { echo 'nginx не готов после рестарта' >&2; exit 1; }
request -H "Cookie: staging_auth=$old" "$STAGING_TEST_BASE_URL/"; expect_code 401; no_pass
request -u "$STAGING_TEST_USER:$STAGING_TEST_PASS" "$STAGING_TEST_BASE_URL/"; expect_code 200
new=$(pass_value)
[[ "$new" =~ ^[0-9a-f]{64}$ && "$new" != "$old" ]] || { echo 'Токен не сменился' >&2; exit 1; }
request -H "Cookie: staging_auth=$new" "$STAGING_TEST_BASE_URL/"; expect_code 200
echo 'PASS: Basic Auth, открытые маршруты, заголовки, legacy и ротация cookie'
