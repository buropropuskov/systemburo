#!/usr/bin/env bash
# Состояние резервного копирования: когда снята последняя копия, что лежит в
# хранилище, не устарела ли копия.
#
#   ./scripts/backup-status.sh [local|staging|production]
#   BACKUP_DIR=/mnt/disk/systemburo ./scripts/backup-status.sh   # чужое хранилище
#
# Только читает. Нужен для ежедневного контроля: молчаливо сломавшееся копирование
# ничем не отличается от работающего, пока не понадобится восстановление.
set -euo pipefail

cd "$(dirname "$0")/.."

# Каталог из окружения запоминается до чтения .env и имеет приоритет над ним:
# `set -a; . ./.env` перекрывает окружение, и запуск с BACKUP_DIR=... молча
# показывал состояние рабочего каталога вместо указанного. Скрипт только читает,
# и возможность посмотреть чужое хранилище (копии, принесённые с другого сервера,
# примонтированный внешний диск) не должна требовать правки параметров рабочего.
BACKUP_DIR_ARG="${BACKUP_DIR:-}"

if [ ! -f .env ]; then
  echo "Файл параметров .env не найден в $(pwd)" >&2
  exit 1
fi
set -a; . ./.env; set +a

BACKUP_DIR="${BACKUP_DIR_ARG:-${BACKUP_DIR:-/var/backups/systemburo}}"
STATUS_FILE="${BACKUP_DIR}/status.json"

if [ ! -d "$BACKUP_DIR" ]; then
  echo "Каталог копий ${BACKUP_DIR} не существует: копирование ни разу не выполнялось."
  echo "Поставьте его на расписание: sudo ./scripts/backup-install.sh production"
  exit 1
fi

# Проверка выше проходит и на закрытом каталоге: сведения о нём читаются по правам
# родителя, а не его собственным. Без этой строки закрытый каталог выглядел бы как
# «копирование ни разу не завершалось» - то есть скрипт врал бы ровно в том случае,
# ради которого его и открывают.
if [ ! -x "$BACKUP_DIR" ]; then
  echo "Каталог копий ${BACKUP_DIR} закрыт для учётной записи $(id -un)."
  echo "Состояние копирования читает суперпользователь:  sudo ./scripts/backup-status.sh ${1:-production}"
  echo "Оператору доступ выдаётся группой BACKUP_OPERATOR_GROUP, её выставляет scripts/backup-install.sh."
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
elif [ "${BACKUP_ALLOW_UNENCRYPTED:-}" = "yes" ]; then
  echo "Шифрование: ВЫКЛЮЧЕНО по явному разрешению, копии содержат персональные данные в открытом виде"
else
  echo "Шифрование: ключ не задан - копирование останавливается с ошибкой, пока не задан BACKUP_AGE_RECIPIENT"
fi
echo

if [ -f "$STATUS_FILE" ]; then
  # Подавление здесь не прячет сбой, а наоборот: без него grep, не нашедший поля,
  # возвращает единицу, pipefail поднимает её наверх, и set -e убивает разбор на
  # полуслове. Файл состояния может быть оборван записью на кончившемся диске -
  # то есть ровно тогда, когда эти строки и читают. Пустое значение ниже честно
  # печатается как «причина не записана».
  RESULT="$(grep -o '"result": *"[^"]*"' "$STATUS_FILE" | cut -d'"' -f4 || true)"
  FINISHED="$(grep -o '"finished_at": *"[^"]*"' "$STATUS_FILE" | cut -d'"' -f4 || true)"
  REASON="$(grep -o '"reason": *"[^"]*"' "$STATUS_FILE" | cut -d'"' -f4 || true)"
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
  # Строка выше говорит о настройке на сейчас, эта - о том, что реально лежит на
  # диске: ключ могли задать уже после снятия последней копии. У копий, снятых
  # прежней версией скрипта, поля нет, и строка не печатается.
  if [ "$(grep -o '"encrypted": *[a-z]*' "$STATUS_FILE" | awk '{print $2}')" = "false" ]; then
    echo "ВНИМАНИЕ: последняя копия снята БЕЗ шифрования"
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

# Каталоги сроков хранения открыты только суперпользователю: в копиях персональные
# данные. Оператор видит отчёт и журнал выше, а перечня копий не увидит - и find
# ниже оборвал бы разбор молча: свою ошибку он не показывает (2>/dev/null), а
# pipefail поднимает её код наверх. Поэтому говорим прямо, чего не видно и почему.
if [ -d "${BACKUP_DIR}/daily" ] && [ ! -r "${BACKUP_DIR}/daily" ]; then
  echo "Перечень копий доступен только суперпользователю: сами копии закрыты, оператору видны состояние и журнал."
  echo "Полный перечень:  sudo ./scripts/backup-status.sh ${1:-production}"
  echo "Журнал:           ${BACKUP_DIR}/backup.log"
  exit 0
fi

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

# Метка времени в имени: «2026-08-06-1612» это 15 знаков, дальше идёт метка
# запуска, если она была задана. Разбор нужен и выгрузке базы, и обоим архивам.
STAMP=""; LABEL=""
split_stamp() {
  STAMP="$1"; LABEL=""
  if [ "${#STAMP}" -gt 15 ]; then
    LABEL="${STAMP:16}"
    STAMP="${STAMP:0:15}"
  fi
}

# Файл ищется по всем каталогам сроков хранения: одна и та же копия лежит в них
# жёсткой ссылкой, и в каком именно каталоге она встретится, значения не имеет.
locate() {
  local name="$1" dir
  for dir in daily weekly monthly; do
    if [ -f "${BACKUP_DIR}/${dir}/${name}" ]; then echo "${BACKUP_DIR}/${dir}/${name}"; return 0; fi
  done
  return 0
}

# Спутники выгрузки базы: backup.sh даёт всем трём файлам одной копии общую метку
# времени, поэтому пара к базе находится по её же имени. Суффикс .age появляется
# у зашифрованных копий, так что проверяются оба варианта имени.
find_companion() {
  local prefix="$1" full="$2" dir cand
  for dir in daily weekly monthly; do
    for cand in "${BACKUP_DIR}/${dir}/${prefix}-${full}.tar.gz" \
                "${BACKUP_DIR}/${dir}/${prefix}-${full}.tar.gz.age"; do
      if [ -f "$cand" ]; then echo "$cand"; return 0; fi
    done
  done
  return 0
}

# Ширины подобраны под самое длинное значение колонки: дата «26 сентября 2026,
# 03:00» это 23 знака, состав «база+файлы+архив» - 16, метке хватает 14 (она почти
# всегда «-», а длинная раздвинет строку только в своей копии). Запас здесь стоит
# дорого: перечень уезжает за ширину окна, а в руководстве строка переносится, и
# столбцы рассыпаются.
row() {
  local when="$1" bytes="$2" what="$3" label="$4" name="$5"
  echo "$(pad_right "$when" 23) $(pad_left "$(human_size "$bytes")" 8)  $(pad_right "$what" 16) $(pad_right "$label" 14) ${name}"
}

# Строка спутника: дата и метка у него те же, что у базы строкой выше, поэтому
# повторно они не печатаются - иначе перечень читается как список разных копий.
companion_row() {
  local file="$1" what="$2" bytes
  bytes="$(stat -c%s "$file" 2>/dev/null || echo 0)"
  row "" "$bytes" "  ${what}" "" "$(basename "$file")"
}

echo "Копии, пригодные для восстановления"
echo "$(pad_right "Снята" 23) $(pad_left "Размер" 8)  $(pad_right "Что входит" 16) $(pad_right "Метка" 14) Файл"

found=0
paired=""
partial=0
newest_what=""
while read -r name; do
  [ -n "$name" ] || continue
  found=1
  full="${name#buro-db-}"; full="${full%%.dump*}"
  split_stamp "$full"
  bytes="$(stat -c%s "$(locate "$name")" 2>/dev/null || echo 0)"

  uploads="$(find_companion buro-uploads "$full")"
  archive="$(find_companion buro-archive "$full")"
  # Состав копии одной строкой: без него копия без файлов выглядит так же
  # благополучно, как полная, а разница видна только в момент восстановления.
  what="база"
  if [ -n "$uploads" ]; then what="${what}+файлы"; fi
  if [ -n "$archive" ]; then what="${what}+архив"; fi
  if [ "$what" = "база" ]; then what="только база"; partial=1; fi
  if [ -z "$newest_what" ]; then newest_what="$what"; fi

  row "$(human_when "$STAMP")" "$bytes" "$what" "${LABEL:--}" "$name"
  if [ -n "$uploads" ]; then
    companion_row "$uploads" "файлы"
    paired="${paired} $(basename "$uploads")"
  fi
  if [ -n "$archive" ]; then
    companion_row "$archive" "архив бланков"
    paired="${paired} $(basename "$archive")"
  fi
done <<< "$(find "$BACKUP_DIR"/{daily,weekly,monthly} -maxdepth 1 -name 'buro-db-*' -type f -printf '%f\n' 2>/dev/null | sort -ru)"

if [ "$found" = "0" ]; then
  echo "копий базы нет"
fi

if [ "$partial" = "1" ] && [ "${BACKUP_UPLOADS_MODE:-weekly}" != "daily" ]; then
  # Без этой строки состав «только база» у большинства суточных копий читается
  # как поломка, хотя это заданный режим: файлы архивируются по воскресеньям.
  echo
  echo "Файлы архивируются раз в неделю (BACKUP_UPLOADS_MODE=${BACKUP_UPLOADS_MODE:-weekly}),"
  echo "поэтому состав «только база» у копий за остальные дни - ожидаемый."
fi

# В ежедневном режиме отсутствие архива у свежей копии - уже сбой архивации, а не
# расписание. backup.sh пишет о нём в журнал, но в журнал заглядывают редко.
if [ "${BACKUP_UPLOADS_MODE:-weekly}" = "daily" ] && [ "$newest_what" = "только база" ]; then
  echo
  echo "ВНИМАНИЕ: режим BACKUP_UPLOADS_MODE=daily, но в последнюю копию файлы не вошли."
  echo "Причина - в журнале ${BACKUP_DIR}/backup.log."
fi

# Архивы файлов переживают свою выгрузку базы: сроки хранения считаются по каждому
# префиксу отдельно, а при недельном режиме архивы снимаются реже. Восстанавливать
# такой архив не с чем, но знать о нём нужно - место он занимает наравне с прочими.
orphans=0
while read -r name; do
  [ -n "$name" ] || continue
  case " $paired " in *" $name "*) continue ;; esac
  case "$name" in
    buro-uploads-*) what="файлы"; full="${name#buro-uploads-}" ;;
    buro-archive-*) what="архив бланков"; full="${name#buro-archive-}" ;;
    *) continue ;;
  esac
  full="${full%%.tar.gz*}"
  split_stamp "$full"
  if [ "$orphans" = "0" ]; then
    orphans=1
    echo
    echo "Архивы файлов без выгрузки базы за тот же срок"
    echo "$(pad_right "Снята" 23) $(pad_left "Размер" 8)  $(pad_right "Что входит" 16) $(pad_right "Метка" 14) Файл"
  fi
  bytes="$(stat -c%s "$(locate "$name")" 2>/dev/null || echo 0)"
  row "$(human_when "$STAMP")" "$bytes" "$what" "${LABEL:--}" "$name"
done <<< "$(find "$BACKUP_DIR"/{daily,weekly,monthly} -maxdepth 1 \( -name 'buro-uploads-*' -o -name 'buro-archive-*' \) -type f -printf '%f\n' 2>/dev/null | sort -ru)"
