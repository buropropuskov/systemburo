#!/bin/sh
set -e

# Генерация htpasswd через openssl (встроен в nginx:alpine, не требует apk).
# Раньше использовался apache2-utils, но apk update перестал работать на
# staging-сервере (Permission denied к alpine mirror), поэтому переехали на openssl.
# Формат: user:apr1-hash (Apache MD5 - совместим с nginx auth_basic_user_file).
if [ -z "$BASIC_AUTH_PASS" ]; then
    echo "BASIC_AUTH_PASS пуст: стенд без пароля не поднимаем" >&2
    exit 1
fi

HASH=$(openssl passwd -apr1 "$BASIC_AUTH_PASS")
echo "$BASIC_AUTH_USER:$HASH" > /etc/nginx/.htpasswd
chmod 644 /etc/nginx/.htpasswd

# Пропуск действует только до следующего старта nginx. Не принимаем старые
# значения: рестарт отзывает все ранее выданные cookie без изменения .env/JWT.
TOKEN=$(openssl rand -hex 32)
[ "${#TOKEN}" -eq 64 ] || { echo "Не удалось создать токен стенда" >&2; exit 1; }

# map_hash_bucket_size: значение cookie - 64 знака, в корзину по умолчанию
# (64 байта) ключ такой длины не помещается, и nginx отказывается стартовать
# с "could not build map_hash".
cat > /etc/nginx/staging-auth.conf <<CONF
map_hash_bucket_size 128;

map \$cookie_staging_auth \$staging_realm {
    default "Staging";
    "$TOKEN" off;
}

map \$status \$staging_auth_set_cookie {
    default "";
    ~^2 "staging_auth=$TOKEN; Path=/; Max-Age=604800; HttpOnly; Secure; SameSite=Strict";
}
CONF
chmod 600 /etc/nginx/staging-auth.conf

# add_header в location отменяет наследование всех server add_header.
# Общий набор включается явно там, где выдаётся пропуск или собственный CSP.
cat > /etc/nginx/staging-common-headers.conf <<'CONF'
add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
CONF
cat > /etc/nginx/staging-security-headers.conf <<'CONF'
include /etc/nginx/staging-common-headers.conf;
add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'nonce-$request_id'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; object-src 'self' blob:; frame-src 'self' blob:; frame-ancestors 'self';" always;
CONF
