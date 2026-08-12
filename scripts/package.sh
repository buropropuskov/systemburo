#!/usr/bin/env bash
#
# Сборка архива поставки systemburo-<версия>.tar.gz для площадки без доступа к
# репозиторию. Разворачивается так, как описано в руководстве по развёртыванию:
#
#   tar -xzf systemburo-<версия>.tar.gz -C /opt/systemburo --strip-components=1
#
# Отсюда единственный верхний каталог внутри архива: --strip-components=1 снимает
# ровно его, и содержимое ложится в каталог системы.
#
# Собирается из зафиксированного состояния (HEAD), а не из рабочего каталога:
# git archive берёт дерево коммита, поэтому незакоммиченные правки в архив не
# попадают, а один и тот же коммит даёт один и тот же архив побайтово. Отсюда же
# бесплатно берётся главное свойство: в архив физически не может попасть то, чего
# нет в репозитории, - файл параметров .env, node_modules, собранные образы,
# загруженные файлы, каталоги рабочих копий. Всё это не отслеживается.
#
# Версия: make package VERSION=1.2.0. Без неё берётся метка (git tag), а если
# меток нет - дата коммита и его короткий определитель.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Каталоги, которые отслеживаются, но к установке отношения не имеют.
EXCLUDE=(
	':(exclude).github'    # конвейер сборки на стороне разработчика
	':(exclude)load-tests' # нагрузочные сценарии, инструмент разработки
)

OUT_DIR="${OUT_DIR:-dist}"

if [ -z "${VERSION:-}" ]; then
	VERSION="$(git describe --tags --abbrev=0 2>/dev/null || true)"
fi
if [ -z "$VERSION" ]; then
	VERSION="$(git log -1 --format=%cd --date=format:%Y.%m.%d)-$(git log -1 --format=%h)"
fi

NAME="systemburo-${VERSION}"
ARCHIVE="${OUT_DIR}/${NAME}.tar.gz"

if ! git diff-index --quiet HEAD -- 2>/dev/null; then
	echo "ВНИМАНИЕ: в рабочем каталоге есть незакоммиченные правки." >&2
	echo "          Архив собирается из коммита $(git log -1 --format=%h), их в нём не будет." >&2
fi

mkdir -p "$OUT_DIR"
# gzip -n не пишет в заголовок имя и время: без этого два прогона на одном коммите
# давали бы разные файлы, и сверить поставку с исходником было бы нечем.
git archive --format=tar --prefix="${NAME}/" HEAD -- . "${EXCLUDE[@]}" | gzip -9n >"$ARCHIVE"

# Проверка того, что собралось, а не того, что задумывалось. Файл параметров несёт
# пароль базы, ключи подписи маркеров доступа и ключ шифрования персональных
# данных: попади он в поставку - утекут все три сразу. Поэтому проверка идёт по
# готовому архиву и роняет сборку, а не печатает предупреждение.
CONTENTS="$(mktemp)"
trap 'rm -f "$CONTENTS"' EXIT
tar -tzf "$ARCHIVE" >"$CONTENTS"

FORBIDDEN="$(sed "s|^${NAME}/||" "$CONTENTS" |
	grep -E '(^|/)(\.env($|\.)|\.git/|node_modules/|dist/)' |
	grep -Ev '(^|/)\.env\.example$' || true)"
if [ -n "$FORBIDDEN" ]; then
	echo "ОШИБКА: в архив попало то, чего в поставке быть не должно:" >&2
	echo "$FORBIDDEN" >&2
	rm -f "$ARCHIVE"
	exit 1
fi

FILES="$(grep -v '/$' "$CONTENTS" | wc -l)"
SIZE="$(du -h "$ARCHIVE" | cut -f1)"

echo "Архив поставки собран"
echo "  путь:    ${ARCHIVE}"
echo "  версия:  ${VERSION} (коммит $(git log -1 --format=%h) от $(git log -1 --format=%cd --date=format:%d.%m.%Y))"
echo "  размер:  ${SIZE}"
echo "  файлов:  ${FILES}"
echo "  не вошли: .env и всё неотслеживаемое, $(printf '%s ' "${EXCLUDE[@]}" | sed 's/:(exclude)//g')"
