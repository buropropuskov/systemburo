#!/usr/bin/env bash
# Состояние резервного копирования: когда снята последняя копия, что лежит в
# хранилище, не устарела ли копия.
#
#   ./scripts/backup-status.sh [local|staging|production]
#
# Только читает. Нужен для ежедневного контроля: молчаливо сломавшееся копирование
# ничем не отличается от работающего, пока не понадобится восстановление.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
  echo "Файл параметров .env не найден в $(pwd)" >&2
  exit 1
fi
set -a; . ./.env; set +a

BACKUP_DIR="${BACKUP_DIR:-/var/backups/systemburo}"
STATUS_FILE="${BACKUP_DIR}/status.json"

if [ ! -d "$BACKUP_DIR" ]; then
  echo "Каталог копий ${BACKUP_DIR} не существует: копирование ни разу не выполнялось."
  echo "Поставьте его на расписание: sudo ./scripts/backup-install.sh production"
  exit 1
fi

echo "Каталог копий: ${BACKUP_DIR}"
if [ -n "${BACKUP_S3_REMOTE:-}" ] && [ -n "${BACKUP_S3_BUCKET:-}" ]; then
  echo "Внешнее хранилище: ${BACKUP_S3_REMOTE}:${BACKUP_S3_BUCKET}"
else
  echo "Внешнее хранилище: не настроено, копии лежат только на этом сервере"
fi
if [ -n "${BACKUP_AGE_RECIPIENT:-}" ]; then
  echo "Шифрование: включено"
else
  echo "Шифрование: ВЫКЛЮЧЕНО, копии содержат персональные данные в открытом виде"
fi
echo

if [ -f "$STATUS_FILE" ]; then
  RESULT="$(grep -o '"result": *"[^"]*"' "$STATUS_FILE" | cut -d'"' -f4)"
  FINISHED="$(grep -o '"finished_at": *"[^"]*"' "$STATUS_FILE" | cut -d'"' -f4)"
  REASON="$(grep -o '"reason": *"[^"]*"' "$STATUS_FILE" | cut -d'"' -f4)"
  AGE_SEC=$(( $(date +%s) - $(date -d "$FINISHED" +%s 2>/dev/null || echo 0) ))
  AGE_HOURS=$(( AGE_SEC / 3600 ))
  # Часы и минуты, а не одни часы: «0 ч назад» не отличает копию пятиминутной
  # давности от снятой сорок минут назад.
  AGE_TEXT="$(( AGE_SEC / 3600 )) ч $(( (AGE_SEC % 3600) / 60 )) мин назад"

  echo "Последний запуск: $(date -d "$FINISHED" '+%d.%m.%Y %H:%M' 2>/dev/null || echo "$FINISHED") (${AGE_TEXT})"
  if [ "$RESULT" = "success" ]; then
    echo "Итог: успешно"
  else
    echo "Итог: СБОЙ - ${REASON:-причина не записана}"
  fi
  # Сутки с запасом на сдвиг расписания: копия старше двух суток означает, что
  # запуск не состоялся, и это уже повод разбираться, а не ждать дальше.
  if [ "$AGE_HOURS" -gt 48 ]; then
    echo "ВНИМАНИЕ: свежей копии нет более двух суток, проверьте journalctl -u systemburo-backup"
  fi
else
  echo "Файл состояния не найден: копирование ни разу не завершалось."
fi

echo

# Выравнивание по знакам, а не по байтам: printf с %-10s считает байты, и
# кириллический заголовок занимает вдвое больше, из-за чего шапка съезжает
# относительно значений. ${#s} в UTF-8 локали возвращает именно число знаков.
pad_right() { local s="$1" w="$2"; printf '%s%*s' "$s" "$(( w > ${#s} ? w - ${#s} : 0 ))" ""; }
pad_left()  { local s="$1" w="$2"; printf '%*s%s' "$(( w > ${#s} ? w - ${#s} : 0 ))" "" "$s"; }

echo "$(pad_right "Каталог" 10) $(pad_left "Файлов" 8) $(pad_left "Объём" 10)  Последняя копия базы"
for dir in daily weekly monthly; do
  path="${BACKUP_DIR}/${dir}"
  [ -d "$path" ] || continue
  count="$(find "$path" -maxdepth 1 -name 'buro-*' -type f 2>/dev/null | wc -l)"
  size="$(du -sh "$path" 2>/dev/null | cut -f1)"
  last="$(find "$path" -maxdepth 1 -name 'buro-db-*' -type f -printf '%f\n' 2>/dev/null | sort -r | head -1)"
  echo "$(pad_right "$dir" 10) $(pad_left "$count" 8) $(pad_left "${size:-0}" 10)  ${last:-нет}"
done

echo

# Перечень копий: имя файла в машинном виде оператору читать тяжело, а именно из
# него он выбирает копию для восстановления. Дата разворачивается словами, размер
# приводится к мегабайтам, метка выносится отдельной колонкой.
MONTHS=(января февраля марта апреля мая июня июля августа сентября октября ноября декабря)

# Разделитель - точка, как в `server archive show` и в разделе «Файловый архив»:
# оператор видит размеры в трёх местах и не должен гадать, одна ли это величина.
# LC_ALL=C держит вывод одинаковым на любом сервере: gawk под POSIXLY_CORRECT
# берёт разделитель из локали и печатает запятую. Именно LC_ALL, а не LC_NUMERIC:
# заданный в окружении LC_ALL перебил бы его.
human_size() {
  LC_ALL=C awk -v b="$1" 'BEGIN {
    if (b >= 1073741824) printf "%.1f ГБ", b / 1073741824;
    else printf "%.1f МБ", b / 1048576;
  }'
}

# Дата из имени файла, а не из времени файла: жёсткая ссылка и перенос во внешнее
# хранилище время правки не сохраняют, а имя копии несёт момент её снятия.
human_when() {
  local stamp="$1" y m d hh mm
  y="${stamp:0:4}"; m="${stamp:5:2}"; d="${stamp:8:2}"
  hh="${stamp:11:2}"; mm="${stamp:13:2}"
  printf '%s %s %s, %s:%s' "$((10#$d))" "${MONTHS[$((10#$m - 1))]}" "$y" "$hh" "$mm"
}

echo "Копии базы, пригодные для восстановления"
echo "$(pad_right "Снята" 24) $(pad_left "Размер" 8)  $(pad_right "Метка" 20) Файл"

found=0
while read -r name; do
  [ -n "$name" ] || continue
  found=1
  stamp="${name#buro-db-}"; stamp="${stamp%%.dump*}"
  label=""
  if [ "${#stamp}" -gt 15 ]; then
    label="${stamp:16}"
    stamp="${stamp:0:15}"
  fi
  file=""
  for dir in daily weekly monthly; do
    [ -f "${BACKUP_DIR}/${dir}/${name}" ] && { file="${BACKUP_DIR}/${dir}/${name}"; break; }
  done
  bytes="$(stat -c%s "$file" 2>/dev/null || echo 0)"
  echo "$(pad_right "$(human_when "$stamp")" 24) $(pad_left "$(human_size "$bytes")" 8)  $(pad_right "${label:--}" 20) ${name}"
done <<< "$(find "$BACKUP_DIR"/{daily,weekly,monthly} -maxdepth 1 -name 'buro-db-*' -type f -printf '%f\n' 2>/dev/null | sort -ru)"

if [ "$found" = "0" ]; then
  echo "копий базы нет"
fi
