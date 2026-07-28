#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Сверка утверждений документации с кодом.

Документы передаются заказчику, поэтому число в тексте, разошедшееся с
реализацией, дороже бага: по нему принимают решения и планируют железо.
Держать эти числа в голове невозможно - скрипт считает их по репозиторию и
сравнивает с тем, что написано.

Каждый факт описан якорем: регулярным выражением, которое вытаскивает
значение из текста. Якорь не нашёлся - это тоже сообщение (формулировку
переписали, факт перестал проверяться), а не молчаливый пропуск.

Запуск:
    python3 Документация/src/doc_facts.py           # все проверки
    python3 Документация/src/doc_facts.py --quiet   # только расхождения

Код возврата 1, если найдено расхождение или потерян якорь.
"""
import os
import re
import subprocess
import sys

SRC_DIR = os.path.dirname(os.path.abspath(__file__))
DOCS_DIR = os.path.dirname(SRC_DIR)
REPO = os.path.dirname(DOCS_DIR)

OVERVIEW = "Техническое описание системы"
DEPLOY = "Руководство по развёртыванию и сопровождению"

WORDS = {
    "один": 1, "два": 2, "три": 3, "четыре": 4, "пять": 5, "шесть": 6,
    "семь": 7, "восемь": 8, "девять": 9, "десять": 10, "одиннадцать": 11,
    "двенадцать": 12, "тринадцать": 13, "четырнадцать": 14, "пятнадцать": 15,
    "шестнадцать": 16, "семнадцать": 17, "восемнадцать": 18,
    "девятнадцать": 19, "двадцать": 20,
    "одной": 1, "двух": 2, "трёх": 3, "четырёх": 4, "пяти": 5, "шести": 6,
    "семи": 7, "восьми": 8, "девяти": 9, "десяти": 10,
}


def to_int(raw):
    """Число из текста: '4 104', '4104' или 'десять'."""
    cleaned = raw.strip().lower().replace(" ", "").replace(" ", "")
    if cleaned.isdigit():
        return int(cleaned)
    return WORDS.get(raw.strip().lower())


def sh(cmd):
    """Команда в корне репозитория, вывод строкой."""
    return subprocess.run(cmd, shell=True, cwd=REPO, capture_output=True,
                          text=True).stdout


def count(cmd):
    out = sh(cmd).strip()
    return int(out) if out.isdigit() else 0


def read_doc(name):
    path = os.path.join(DOCS_DIR, name, name + ".md")
    with open(path, encoding="utf-8") as fh:
        return fh.read()


# --------------------------------------------------------------------------
# счётчики по коду
# --------------------------------------------------------------------------
def metrics():
    go_files = "$(find internal cmd -name '*_test.go')"
    vue = "frontend/src"
    spec = ("find %s -name '*.spec.js' -o -name '*.spec.ts'" % vue)
    code = (r"git ls-files | grep -E '\.(go|vue|js|ts|css|scss)$' "
            r"| grep -v '^docs/'")

    m = {}
    m["go_tests"] = count(r"grep -rhE '^func Test' --include='*_test.go' "
                          r"internal cmd | wc -l")
    m["go_test_files"] = count("find internal cmd -name '*_test.go' | wc -l")
    m["go_db_files"] = count(r"grep -rl 'testutil\.' --include='*_test.go' "
                             r"internal | wc -l")
    m["vitest"] = count(r"grep -rhoE '^\s*it(\.each)?\s*\(' "
                        r"--include='*.spec.js' --include='*.spec.ts' "
                        r"%s | wc -l" % vue)
    m["vitest_files"] = count("%s | wc -l" % spec)
    m["e2e"] = count(r"grep -rhoE '^\s*test\s*\(' --include='*.spec.*' "
                     r"frontend/e2e | wc -l")
    m["e2e_files"] = count("find frontend/e2e -name '*.spec.*' | wc -l")
    m["tests_total"] = m["go_tests"] + m["vitest"] + m["e2e"]

    # Таблицы: модели AutoMigrate плюс партиционированный журнал доступа к ПД,
    # который заводится в обход AutoMigrate (см. installLogPartitioning).
    m["db_tables"] = count(
        r"sed -n '15,190p' internal/database/migrate.go "
        r"| grep -o '&models\.[A-Za-z0-9_]*' | sort -u | wc -l") + 1

    m["api_methods"] = count(r"grep -oE '\.(GET|POST|PUT|PATCH|DELETE)\(' "
                             r"internal/router/router.go | wc -l")
    m["api_groups"] = count(r"grep -oE '\.Group\(' internal/router/router.go "
                            r"| wc -l")

    catalog = "internal/services/permission_catalog.go"
    m["perm_keys"] = count(r"sed -n '/func staticCatalog/,/^}/p' %s "
                           r"| grep -c '{Key:'" % catalog)
    m["perm_cats"] = count(r"sed -n '/func staticCatalog/,/^}/p' %s "
                           r"| grep -o 'Category: Cat[A-Za-z]*' | sort -u "
                           r"| wc -l" % catalog)
    m["table_verbs"] = count(
        r"sed -n '/^var tableVerbs/,/^}/p' internal/services/"
        r"permission_service.go | grep -c '^\s*{\"'")

    m["gin"] = count("grep -c 'USING gin' internal/database/migrate.go")
    m["user_types"] = count(
        r"sed -n '/types := \[\]models.UserType{/,/^\t\t}/p' "
        r"internal/database/migrate.go | grep -c 'Code:'")

    m["code_files"] = count("%s | wc -l" % code)
    m["code_lines"] = count("%s | xargs cat 2>/dev/null | wc -l" % code)
    m["be_lines"] = count("find cmd internal -name '*.go' ! -name '*_test.go' "
                          "-exec cat {} + | wc -l")
    m["services"] = count("ls internal/services/*.go | grep -vc _test")
    m["handlers"] = count("ls internal/handlers/*.go | grep -vc _test")
    m["models"] = count("ls internal/models/*.go | grep -vc _test")
    m["vue_lines"] = count("find %s -name '*.vue' -exec cat {} + | wc -l" % vue)
    m["views"] = count("find %s/views -name '*.vue' | wc -l" % vue)
    m["components"] = count("find %s/components -name '*.vue' | wc -l" % vue)
    return m


# --------------------------------------------------------------------------
# якоря: что и где сверяем
# --------------------------------------------------------------------------
def anchors(m):
    """(документ, описание, регулярка, ожидаемые значения по группам)."""
    return [
        (OVERVIEW, "типов учётных записей",
         r"заведено (\S+) типов учётных записей", [m["user_types"]]),
        (OVERVIEW, "ключей и категорий каталога прав",
         r"Каталог прав содержит (\d+) фиксированных ключей в (\S+) категориях",
         [m["perm_keys"], m["perm_cats"]]),
        (OVERVIEW, "прав на таблицу поста",
         r"автоматически создаётся (\S+) прав", [m["table_verbs"]]),
        (OVERVIEW, "поисковых индексов",
         r"создано ([\d  ]+?) специальных индексов", [m["gin"]]),
        (OVERVIEW, "таблиц базы данных",
         r"База содержит ([\d  ]+?) таблиц", [m["db_tables"]]),
        (OVERVIEW, "методов и групп программного интерфейса",
         r"Зарегистрировано ([\d  ]+?) методов в (\d+) группах",
         [m["api_methods"], m["api_groups"]]),
        (OVERVIEW, "бэкенд-тестов",
         r"Штатное средство тестирования Go \| ([\d  ]+?) тест\w* "
         r"в (\d+) файлах", [m["go_tests"], m["go_test_files"]]),
        (OVERVIEW, "фронтенд-тестов",
         r"Vitest \| ([\d  ]+?) тест\w* в (\d+) файлах",
         [m["vitest"], m["vitest_files"]]),
        (OVERVIEW, "сквозных сценариев",
         r"Playwright \| ([\d  ]+?) тест\w* в (\d+) файлах",
         [m["e2e"], m["e2e_files"]]),
        (OVERVIEW, "всего тестов",
         r"\| Всего \| \| ([\d  ]+?) \|", [m["tests_total"]]),
        (OVERVIEW, "тестов на настоящей базе",
         r"(\d+) файлов бэкенд-тестов выполняются", [m["go_db_files"]]),
        (OVERVIEW, "объёма исходного кода",
         r"Объём исходного кода \| ([\d  ]+?) строк "
         r"в ([\d  ]+?) файлах", [m["code_lines"], m["code_files"]]),
        (OVERVIEW, "состава серверной части",
         r"Серверная часть \| ([\d  ]+?) строк\w* без тестов: (\d+) служб\w*, "
         r"(\d+) обработчик\w*, (\d+) модел\w*",
         [m["be_lines"], m["services"], m["handlers"], m["models"]]),
        (OVERVIEW, "состава интерфейса",
         r"интерфейса \| ([\d  ]+?) строк\w*: (\d+) экранов, "
         r"(\d+) элемент\w*", [m["vue_lines"], m["views"], m["components"]]),
    ]


# --------------------------------------------------------------------------
# параметры окружения и базы данных
# --------------------------------------------------------------------------
def env_defaults():
    """Имя переменной -> значение по умолчанию из config.go."""
    path = os.path.join(REPO, "internal/config/config.go")
    with open(path, encoding="utf-8") as fh:
        body = fh.read()
    found = {}
    for name, default in re.findall(
            r'env:"([A-Z_]+)(?:,required)?"(?:\s+envDefault:"([^"]*)")?', body):
        found[name] = default
    return found


def doc_env_table(text):
    """Имя переменной -> значение из приложения Б."""
    rows = {}
    for name, value in re.findall(r"^\| `([A-Z_]+)` \| ([^|]+?) \|", text, re.M):
        rows[name] = value.strip().strip("`")
    return rows


def db_settings():
    """Параметр базы -> значение из описания рабочего сервера."""
    path = os.path.join(REPO, "docker-compose.prod.yml")
    with open(path, encoding="utf-8") as fh:
        body = fh.read()
    return dict(re.findall(r'"([a-z_]+)=([^"]+)"', body))


def check_env(text, problems):
    code = env_defaults()
    doc = doc_env_table(text)
    for name, shown in doc.items():
        if name not in code:
            continue
        actual = code[name]
        if actual == "" and shown in ("пусто", "нет", "нет, обязателен"):
            continue
        if shown.startswith("нет"):
            continue
        # Часть значений в таблице записана словами («и формат документов»),
        # потому что машинное значение нечитаемо. Кириллица - признак такого
        # описания: сверять его построчно бессмысленно.
        if re.search(r"[а-яё]", shown, re.I):
            continue
        # В тексте у части значений идёт пояснение через запятую.
        head = shown.split(",")[0].strip()
        if head != actual and shown != actual:
            problems.append(
                "%s, приложение Б, %s: в тексте «%s», в config.go «%s»"
                % (DEPLOY, name, shown, actual))


def check_db(text, problems):
    code = db_settings()
    for name, shown in re.findall(r"^\| `([a-z_]+)` \| `([^`]+)` \|", text, re.M):
        if name in code and code[name] != shown:
            problems.append(
                "%s, параметры базы, %s: в тексте «%s», "
                "в docker-compose.prod.yml «%s»" % (DEPLOY, name, shown, code[name]))


def check_paths(text, problems):
    """Файлы, названные обязательными, должны существовать."""
    section = text.split("# Приложение В")
    if len(section) < 2:
        problems.append("%s: не найдено приложение с перечнем файлов" % DEPLOY)
        return
    listed = re.findall(r"^\| `([^`]+)` \|", section[1], re.M)
    for item in listed:
        for part in [p.strip() for p in item.split(",")]:
            if not part or part.startswith("."):
                continue
            if not os.path.exists(os.path.join(REPO, part)):
                problems.append("%s, приложение В: нет файла %s" % (DEPLOY, part))


def check_make_targets(problems):
    """Каждая команда make из документов должна быть целью Makefile."""
    with open(os.path.join(REPO, "Makefile"), encoding="utf-8") as fh:
        targets = set(re.findall(r"^([a-z][a-z0-9-]*):", fh.read(), re.M))
    for name in (OVERVIEW, DEPLOY):
        text = read_doc(name)
        for target in sorted(set(re.findall(r"make ([a-z][a-z0-9-]*)", text))):
            if target not in targets:
                problems.append("%s: команда «make %s» в Makefile отсутствует"
                                % (name, target))


# --------------------------------------------------------------------------
def behind_dev():
    """На сколько коммитов рабочее дерево отстало от dev.

    Проверка считает по рабочему дереву, поэтому на отставшей ветке она
    уверенно врёт: сообщает о «несуществующих» файлах, которые в dev уже
    есть, и о числах, отставших вместе с кодом. Ровно так документация
    однажды описала выпуск сертификата, которого в её собственной ветке
    не было.
    """
    out = sh("git rev-list --count HEAD..origin/dev 2>/dev/null").strip()
    return int(out) if out.isdigit() else 0


def main():
    quiet = "--quiet" in sys.argv
    m = metrics()
    problems, lost, checked = [], [], 0

    lag = behind_dev()
    if lag:
        print("ВНИМАНИЕ: ветка отстала от origin/dev, коммитов позади: %d. "
              "Сначала влить dev, иначе сверка идёт против старого кода." % lag)

    texts = {name: read_doc(name) for name in (OVERVIEW, DEPLOY)}

    for doc, label, pattern, expected in anchors(m):
        match = re.search(pattern, texts[doc])
        if not match:
            lost.append("%s: якорь «%s» не найден, факт больше не проверяется"
                        % (doc, label))
            continue
        for shown, actual in zip(match.groups(), expected):
            checked += 1
            value = to_int(shown)
            if value != actual:
                problems.append("%s, %s: в тексте «%s», по коду %d"
                                % (doc, label, shown.strip(), actual))

    # Второе вхождение числа индексов: оно упоминается в двух разделах.
    for shown in re.findall(r"создано ([\d  ]+?) специальных индексов",
                            texts[OVERVIEW]):
        checked += 1
        if to_int(shown) != m["gin"]:
            problems.append("%s, поисковые индексы: в тексте «%s», по коду %d"
                            % (OVERVIEW, shown.strip(), m["gin"]))

    check_env(texts[DEPLOY], problems)
    check_db(texts[DEPLOY], problems)
    check_paths(texts[DEPLOY], problems)
    check_make_targets(problems)

    if problems:
        print("РАСХОЖДЕНИЯ С КОДОМ (%d):" % len(problems))
        for line in problems:
            print("  " + line)
    if lost:
        print("ПОТЕРЯННЫЕ ЯКОРЯ (%d):" % len(lost))
        for line in lost:
            print("  " + line)
    if not problems and not lost and not quiet:
        print("Документация сходится с кодом: сверено утверждений %d, "
              "параметров %d." % (checked, len(env_defaults())))
    return 1 if problems or lost else 0


if __name__ == "__main__":
    sys.exit(main())
