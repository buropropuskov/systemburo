#!/usr/bin/env bash
# Постановка резервного копирования на расписание.
#
#   sudo ./scripts/backup-install.sh [staging|production] [время]
#
# Создаёт два таймера systemd: ежедневное копирование (по умолчанию в 03:30) и
# ежемесячную проверку восстановимости. Выбран systemd, а не cron: видно время
# выполнения, код возврата и вывод последнего запуска через journalctl, а
# пропущенный из-за выключенного сервера запуск догоняется сам (Persistent).
#
# Скрипт идемпотентен: повторный запуск обновляет описания и перечитывает их.
set -euo pipefail

ENVIRONMENT="${1:-production}"
AT_TIME="${2:-03:30}"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

case "$ENVIRONMENT" in
  staging|production) ;;
  *)
    echo "Использование: $0 [staging|production] [время вида 03:30]" >&2
    exit 1
    ;;
esac

if [ "$(id -u)" -ne 0 ]; then
  echo "Требуются права суперпользователя: sudo $0 $*" >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemd не найден. Поставьте задание вручную, порядок в руководстве, подраздел 11.4." >&2
  exit 1
fi

write_unit() {
  local name="$1"
  cat > "/etc/systemd/system/${name}"
}

write_unit systemburo-backup.service <<EOF
[Unit]
Description=Резервное копирование системы электронной подачи заявок
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
WorkingDirectory=${PROJECT_DIR}
ExecStart=/usr/bin/env bash ${PROJECT_DIR}/scripts/backup.sh ${ENVIRONMENT}
EOF

write_unit systemburo-backup.timer <<EOF
[Unit]
Description=Ежедневное резервное копирование

[Timer]
OnCalendar=*-*-* ${AT_TIME}:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

write_unit systemburo-backup-verify.service <<EOF
[Unit]
Description=Проверка восстановимости резервной копии
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
WorkingDirectory=${PROJECT_DIR}
ExecStart=/usr/bin/env bash ${PROJECT_DIR}/scripts/backup-verify.sh ${ENVIRONMENT}
EOF

write_unit systemburo-backup-verify.timer <<EOF
[Unit]
Description=Ежемесячная проверка восстановимости

[Timer]
OnCalendar=*-*-01 05:00:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

# Читать файл параметров целиком здесь незачем - нужны две строки, и обе до
# первого копирования. Значения и их умолчания те же, что в backup.sh.
read_env() {
  grep -E "^$1=" "${PROJECT_DIR}/.env" 2>/dev/null | cut -d= -f2- || true
}
BACKUP_DIR_VALUE="$(read_env BACKUP_DIR)"
BACKUP_DIR_VALUE="${BACKUP_DIR_VALUE:-/var/backups/systemburo}"
# Умолчание подставляется, только если строки нет вовсе: BACKUP_OPERATOR_GROUP=
# без значения - осознанный отказ открывать состояние копирования. Проверка именно
# на наличие строки, потому что grep выше не отличает пустое значение от
# отсутствующего, а backup.sh это различает и каждый запуск переставлял бы права.
if grep -qE '^BACKUP_OPERATOR_GROUP=' "${PROJECT_DIR}/.env" 2>/dev/null; then
  OPERATOR_GROUP="$(read_env BACKUP_OPERATOR_GROUP)"
else
  OPERATOR_GROUP="buro"
fi

mkdir -p "${BACKUP_DIR_VALUE}"
chmod 700 "${BACKUP_DIR_VALUE}"

# Копии остаются закрытыми: в них персональные данные. А состояние копирования
# оператор должен видеть, иначе он не знает даже, снимаются ли копии вообще.
# Права 710 дают его группе только проход внутрь каталога: прочитать по имени
# status.json и backup.log можно, перечислить копии - нет, открыть их - тоже
# (каталоги сроков хранения 700, файлы 600). Те же права выставляет каждый запуск
# backup.sh; здесь они появляются сразу, не дожидаясь первого копирования.
OPERATOR_READY=false
if [ -z "$OPERATOR_GROUP" ]; then
  OPERATOR_NOTE="доступ отключён параметром BACKUP_OPERATOR_GROUP"
elif chgrp "$OPERATOR_GROUP" "${BACKUP_DIR_VALUE}" 2>/dev/null; then
  chmod 710 "${BACKUP_DIR_VALUE}"
  OPERATOR_READY=true
  OPERATOR_NOTE="состояние копирования открыто группе ${OPERATOR_GROUP}, сами копии ей недоступны"
else
  OPERATOR_NOTE="группы ${OPERATOR_GROUP} на сервере нет, состояние копирования увидит только суперпользователь"
fi

systemctl daemon-reload
systemctl enable --now systemburo-backup.timer systemburo-backup-verify.timer

echo
echo "Расписание установлено."
echo "  копирование:  ежедневно в ${AT_TIME}, каталог ${BACKUP_DIR_VALUE}"
echo "  проверка:     первого числа каждого месяца в 05:00"
echo "  оператор:     ${OPERATOR_NOTE}"
echo
echo "Ближайшие запуски:  systemctl list-timers 'systemburo-*'"
echo "Журнал последнего:  journalctl -u systemburo-backup -n 50"
if [ "$OPERATOR_READY" = true ]; then
  echo "Оператору без sudo: cat ${BACKUP_DIR_VALUE}/status.json"
  echo "                    tail ${BACKUP_DIR_VALUE}/backup.log"
fi
echo "Снять с расписания: systemctl disable --now systemburo-backup.timer systemburo-backup-verify.timer"
