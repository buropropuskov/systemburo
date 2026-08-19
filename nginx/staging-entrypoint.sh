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

# Значение cookie, освобождающей от повторного ввода пароля (#2134). Выводится
# из тех же логина и пароля, что и .htpasswd: в репозитории остаётся механизм
# без значения, а .env на стенде править не требуется - его там намеренно не
# перегенерируют, чтобы не потерять ключ шифрования персональных данных.
# Смена пароля стенда обесценивает и выданные cookie, что и требуется.
TOKEN=$(printf '%s:%s' "$BASIC_AUTH_USER" "$BASIC_AUTH_PASS" | openssl dgst -sha256 -hex | awk '{print $NF}')

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
    "401"   "";
    default "staging_auth=$TOKEN; Path=/; Max-Age=604800; HttpOnly; Secure; SameSite=Strict";
}
CONF
chmod 600 /etc/nginx/staging-auth.conf
