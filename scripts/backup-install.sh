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

BACKUP_DIR_VALUE="$(grep -E '^BACKUP_DIR=' "${PROJECT_DIR}/.env" 2>/dev/null | cut -d= -f2- || true)"
BACKUP_DIR_VALUE="${BACKUP_DIR_VALUE:-/var/backups/systemburo}"
mkdir -p "${BACKUP_DIR_VALUE}"
chmod 700 "${BACKUP_DIR_VALUE}"

systemctl daemon-reload
systemctl enable --now systemburo-backup.timer systemburo-backup-verify.timer

echo
echo "Расписание установлено."
echo "  копирование:  ежедневно в ${AT_TIME}, каталог ${BACKUP_DIR_VALUE}"
echo "  проверка:     первого числа каждого месяца в 05:00"
echo
echo "Ближайшие запуски:  systemctl list-timers 'systemburo-*'"
echo "Журнал последнего:  journalctl -u systemburo-backup -n 50"
echo "Снять с расписания: systemctl disable --now systemburo-backup.timer systemburo-backup-verify.timer"
