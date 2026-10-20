#!/bin/sh
set -eu
set -f

# Пустой список: прямое подключение, внешним заголовкам не доверяем.
# Адреса берутся только из подтверждённой конфигурации инфраструктуры.
generated=$(mktemp /etc/nginx/proxy-trust.XXXXXX)
trap 'rm -f "$generated"' EXIT HUP INT TERM
printf '%s\n' 'real_ip_header X-Forwarded-For;' 'real_ip_recursive on;' > "$generated"

trusted=$(printf '%s' "${EDGE_TRUSTED_PROXY_CIDRS:-}" | tr ',' ' ')
for cidr in $trusted; do
    case "$cidr" in
        *[!0-9a-fA-F:./]*|*/*/*) echo 'Некорректный EDGE_TRUSTED_PROXY_CIDRS' >&2; exit 1 ;;
    esac
    case "$cidr" in
        */*) ;;
        *) echo 'EDGE_TRUSTED_PROXY_CIDRS требует адреса с длиной префикса' >&2; exit 1 ;;
    esac
    address=${cidr%/*}
    prefix=${cidr##*/}
    case "$prefix" in
        ''|*[!0-9]*) echo 'Некорректная длина префикса внешнего прокси' >&2; exit 1 ;;
    esac
    case "$address" in
        *:*) maximum=128 ;;
        *.*) maximum=32 ;;
        *) echo 'Некорректный адрес внешнего прокси' >&2; exit 1 ;;
    esac
    if [ "$prefix" -gt "$maximum" ]; then
        echo 'Слишком большая длина префикса внешнего прокси' >&2
        exit 1
    fi
    # Полную корректность IPv4/IPv6 проверяет nginx при запуске.
    printf 'set_real_ip_from %s;\n' "$cidr" >> "$generated"
done
chmod 644 "$generated"
mv "$generated" /etc/nginx/proxy-trust.conf
