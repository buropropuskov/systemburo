#!/bin/sh
# Генерация htpasswd через openssl (встроен в nginx:alpine, не требует apk).
# Раньше использовался apache2-utils, но apk update перестал работать на
# staging-сервере (Permission denied к alpine mirror), поэтому переехали на openssl.
# Формат: user:apr1-hash (Apache MD5 - совместим с nginx auth_basic_user_file).
HASH=$(openssl passwd -apr1 "$BASIC_AUTH_PASS")
echo "$BASIC_AUTH_USER:$HASH" > /etc/nginx/.htpasswd
chmod 644 /etc/nginx/.htpasswd
