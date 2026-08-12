#!/usr/bin/env bash
# Тест выравнивания прав каталога копий при включении доступа оператора
# (scripts/backup-install.sh, issue #2031). Гоняется без настоящего root и
# systemd: id -u и systemctl подменяются в отдельном PATH, юниты systemd
# пишутся во временный каталог через SYSTEMD_DIR, а chgrp настоящий -
# тестовый пользователь меняет владение своих файлов на свою же группу,
# для этого root не нужен.
#
#   ./scripts/backup-install_test.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# Переопределяемо, чтобы прогнать тот же тест против версии до правки
# (см. отчёт: "красным до, зелёным после").
TARGET="${TARGET:-${SCRIPT_DIR}/backup-install.sh}"

SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT

PROJECT_ROOT="${SANDBOX}/project"
mkdir -p "${PROJECT_ROOT}/scripts"
cp "$TARGET" "${PROJECT_ROOT}/scripts/backup-install.sh"

BACKUP_DIR="${SANDBOX}/backups"
OPERATOR_GROUP="$(id -gn)"

# Состояние воспроизводит установку, где каталог копий существовал ещё до
# umask 077 в backup.sh: корень уже закрыт 700, а подкаталоги сроков хранения
# и файлы в них остались с правами, которые оставлял оператор без маски -
# 755 и 644. Ровно то, что нашлось на стенде 12.08.2026.
mkdir -p "${BACKUP_DIR}"/{daily,weekly,monthly}
for retention in daily weekly monthly; do
  echo "тестовая копия" > "${BACKUP_DIR}/${retention}/buro-db-2026-01-01-0000.dump"
  chmod 644 "${BACKUP_DIR}/${retention}/buro-db-2026-01-01-0000.dump"
  chmod 755 "${BACKUP_DIR}/${retention}"
done
chmod 700 "${BACKUP_DIR}"

cat > "${PROJECT_ROOT}/.env" <<EOF
BACKUP_DIR=${BACKUP_DIR}
BACKUP_OPERATOR_GROUP=${OPERATOR_GROUP}
EOF

FAKEBIN="${SANDBOX}/fakebin"
mkdir -p "$FAKEBIN"

cat > "${FAKEBIN}/id" <<'EOF'
#!/usr/bin/env bash
# Скрипту под тестом root нужен только для проверки в начале, дальше он
# работает с обычными файлами тестового пользователя.
if [ "$1" = "-u" ]; then
  echo 0
else
  exec /usr/bin/id "$@"
fi
EOF
chmod +x "${FAKEBIN}/id"

cat > "${FAKEBIN}/systemctl" <<'EOF'
#!/usr/bin/env bash
# Юниты не должны реально ставиться на расписание - тест проверяет только
# права каталога копий.
exit 0
EOF
chmod +x "${FAKEBIN}/systemctl"

SYSTEMD_SANDBOX="${SANDBOX}/etc-systemd-system"
mkdir -p "$SYSTEMD_SANDBOX"

run_install() {
  PATH="${FAKEBIN}:${PATH}" SYSTEMD_DIR="$SYSTEMD_SANDBOX" \
    bash "${PROJECT_ROOT}/scripts/backup-install.sh" production 03:30
}

OUTPUT="$(run_install)"

FAILED=0
check() {
  local what="$1" expected="$2" actual="$3"
  if [ "$expected" != "$actual" ]; then
    echo "НЕ ПРОШЛО: $what - ожидали $expected, получили $actual" >&2
    FAILED=1
  fi
}

for retention in daily weekly monthly; do
  check "режим каталога ${retention}" 700 "$(stat -c %a "${BACKUP_DIR}/${retention}")"
  check "режим файла в ${retention}" 600 "$(stat -c %a "${BACKUP_DIR}/${retention}/buro-db-2026-01-01-0000.dump")"
done
check "режим корня каталога копий" 710 "$(stat -c %a "${BACKUP_DIR}")"

if ! echo "$OUTPUT" | grep -q "выровнено:.*закрыты права 6 записей"; then
  echo "НЕ ПРОШЛО: в выводе установки нет строки о выравнивании 6 записей" >&2
  echo "--- вывод установки ---" >&2
  echo "$OUTPUT" >&2
  FAILED=1
fi

# Повторная установка ничего уже не закрывает - права выставлены верно, и
# строка о выравнивании не должна появляться второй раз молчаливо ничего не
# трогая, но и не крича об уже верном состоянии.
OUTPUT2="$(run_install)"
if echo "$OUTPUT2" | grep -q "выровнено:"; then
  echo "НЕ ПРОШЛО: повторная установка снова сообщает о выравнивании, хотя права уже верны" >&2
  echo "--- вывод повторной установки ---" >&2
  echo "$OUTPUT2" >&2
  FAILED=1
fi

if [ "$FAILED" -eq 0 ]; then
  echo "ОК: права каталога копий выровнены при включении доступа оператора"
else
  exit 1
fi
