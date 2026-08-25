#!/usr/bin/env bash
# Постановка проверки живости на расписание.
#
#   sudo ./scripts/health-install.sh [staging|production] [минут между запусками]
#
# Создаёт службу и таймер systemd. Выбран systemd, а не cron, по той же причине,
# что и у резервного копирования: видно время запуска, код возврата и вывод
# последнего прогона через journalctl.
#
# Скрипт идемпотентен: повторный запуск обновляет описания и перечитывает их.
set -euo pipefail

ENVIRONMENT="${1:-production}"
INTERVAL_MIN="${2:-5}"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

case "$ENVIRONMENT" in
  staging|production) ;;
  *)
    echo "Использование: $0 [staging|production] [минут между запусками]" >&2
    exit 1
    ;;
esac

case "$INTERVAL_MIN" in
  ''|*[!0-9]*)
    echo "Промежуток задаётся целым числом минут, получено «${INTERVAL_MIN}»" >&2
    exit 1
    ;;
esac
if [ "$INTERVAL_MIN" -lt 1 ] || [ "$INTERVAL_MIN" -gt 60 ]; then
  echo "Промежуток допустим от 1 до 60 минут, получено ${INTERVAL_MIN}" >&2
  exit 1
fi
if [ $(( 60 % INTERVAL_MIN )) -ne 0 ]; then
  echo "ВНИМАНИЕ: 60 не делится на ${INTERVAL_MIN} нацело - на стыке часов промежуток будет короче остальных." >&2
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "Требуются права суперпользователя: sudo $0 $*" >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemd не найден. Поставьте задание вручную, порядок в руководстве, подраздел про мониторинг." >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl не найден, а проверка ходит по внешнему адресу и отправляет письмо им же." >&2
  exit 1
fi

write_unit() {
  local name="$1"
  cat > "/etc/systemd/system/${name}"
}

# After, но НЕ Requires: проверка обязана запускаться и тогда, когда docker лежит.
# С Requires systemd отказался бы стартовать службу при мёртвом docker, письма бы
# не было, и авария выглядела бы как тишина - ровно то, ради чего эта проверка и
# заводится.
write_unit systemburo-health.service <<EOF
[Unit]
Description=Проверка живости системы электронной подачи заявок
After=docker.service network-online.target

[Service]
Type=oneshot
WorkingDirectory=${PROJECT_DIR}
ExecStart=/usr/bin/env bash ${PROJECT_DIR}/scripts/health-check.sh ${ENVIRONMENT}
EOF

# Persistent здесь намеренно не ставится, в отличие от копирования: догонять
# пропущенную проверку бессмысленно, её ответ был бы про прошлое. После включения
# сервера первый запуск идёт по OnBootSec, дальше по расписанию.
write_unit systemburo-health.timer <<EOF
[Unit]
Description=Проверка живости каждые ${INTERVAL_MIN} мин

[Timer]
OnBootSec=3min
OnCalendar=*:0/${INTERVAL_MIN}
AccuracySec=30s

[Install]
WantedBy=timers.target
EOF

# Читать файл параметров целиком здесь незачем - нужны несколько строк, и все
# для того, чтобы сказать, чего не хватает. Умолчания те же, что в health-check.sh.
read_env() {
  grep -E "^$1=" "${PROJECT_DIR}/.env" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"' || true
}
STATE_DIR="$(read_env HEALTH_STATE_DIR)"
STATE_DIR="${STATE_DIR:-/var/lib/systemburo/health}"
CHECK_URL="$(read_env HEALTH_CHECK_URL)"
CHECK_USER="$(read_env HEALTH_CHECK_USERNAME)"
ALERT_TO="$(read_env HEALTH_ALERT_TO)"
SMTP_HOST_VALUE="$(read_env SMTP_HOST)"

# 700 и суперпользователь: в журнале внешний адрес системы и имя проверяющей
# учётной записи - половина пары для входа. Каталог создаётся здесь, чтобы права
# были верными с самого начала, а не после первого запуска.
mkdir -p "$STATE_DIR"
chmod 700 "$STATE_DIR"

systemctl daemon-reload
systemctl enable --now systemburo-health.timer

echo
echo "Расписание установлено."
echo "  проверка:   каждые ${INTERVAL_MIN} мин, стенд ${ENVIRONMENT}"
echo "  состояние:  ${STATE_DIR} (только суперпользователь)"

if [ -z "$CHECK_URL" ]; then
  echo
  echo "ВНИМАНИЕ: не задан HEALTH_CHECK_URL - проверка будет падать с кодом 2 при каждом запуске."
  echo "Впишите внешний адрес системы в ${PROJECT_DIR}/.env:  HEALTH_CHECK_URL=https://buro.example.ru"
else
  echo "  адрес:      ${CHECK_URL}"
fi

if [ -z "$CHECK_USER" ]; then
  echo
  echo "ВНИМАНИЕ: не заданы HEALTH_CHECK_USERNAME/HEALTH_CHECK_PASSWORD."
  echo "Вход проверяться не будет, и отказ вида «сайт открывается, а войти нельзя»"
  echo "останется незамеченным - то есть ровно тот, ради которого проверка и нужна."
else
  echo "  входит как: ${CHECK_USER}"
fi

if [ -z "$SMTP_HOST_VALUE" ] || [ -z "$ALERT_TO" ]; then
  echo
  echo "ВНИМАНИЕ: письма выключены (нужны SMTP_HOST и HEALTH_ALERT_TO)."
  echo "Проверка будет работать и писать итог в ${STATE_DIR}/health.log, но об отказе"
  echo "никто не узнает, пока туда не заглянут."
else
  echo "  письма:     ${ALERT_TO}"
fi

echo
echo "Проверить сейчас:    sudo ${PROJECT_DIR}/scripts/health-check.sh ${ENVIRONMENT}"
echo "Ближайшие запуски:   systemctl list-timers 'systemburo-*'"
echo "Журнал последнего:   journalctl -u systemburo-health -n 50"
echo "Состояние:           sudo cat ${STATE_DIR}/state.json"
echo "Снять с расписания:  systemctl disable --now systemburo-health.timer"
