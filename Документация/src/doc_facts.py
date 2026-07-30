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
    python3 Документация/src/doc_facts.py --fix     # подставить числа в текст

Режим --fix правит только те числа, которые скрипт умеет посчитать сам:
объём кода, число тестов, таблиц, методов, прав. Их правка руками - лишняя
работа и источник ошибок, потому что чужой коммит с одним новым тестом
сдвигает сразу несколько показателей. Всё остальное (смысл разделов,
описание поведения) остаётся на человеке: подставить туда нечего.

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
         r"Зарегистрировано ([\d  ]+?) метод\w* в (\d+) группах",
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
        (OVERVIEW, "файлов тестов на настоящей базе",
         r"\| Файлов бэкенд-тестов на настоящей базе \| (\d+) \|",
         [m["go_db_files"]]),
        # Таблица показателей раздела 15 повторяет часть чисел из текста, и эти
        # повторы якорями не были покрыты: методы и число тестов там разошлись с
        # кодом, пока сверка показывала «сходится».
        (OVERVIEW, "методов интерфейса в таблице показателей",
         r"\| Методы программного интерфейса \| ([\d  ]+?) в (\d+) группах \|",
         [m["api_methods"], m["api_groups"]]),
        (OVERVIEW, "таблиц базы в таблице показателей",
         r"\| Таблицы базы данных \| ([\d  ]+?) \|", [m["db_tables"]]),
        (OVERVIEW, "всего тестов в таблице показателей",
         r"\| Автоматические тесты \| ([\d  ]+?) \|", [m["tests_total"]]),
        (OVERVIEW, "объёма исходного кода",
         r"Объём исходного кода \| ([\d  ]+?) строк\w* "
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
# полнота перечней
# --------------------------------------------------------------------------
# Якоря стерегут числа, карта задетых разделов - изменившиеся файлы. Ни то, ни
# другое не ловит пропуск в ПЕРЕЧНЕ: функция, появившаяся до того, как раздел был
# написан, не попадает в дифф и не меняет ни одного числа. Так планировщик
# напоминаний согласующим (#1315, 21.07) не попал в таблицу периодических задач и
# не был замечен ни одной сверкой - нашёлся только ручной ревизией.
#
# Отсюда третья проверка: состав. Для перечня берётся машинный список из кода и
# реестр соответствий «идентификатор -> фрагмент строки документа». Расходится в
# любую сторону - скрипт называет, в какую именно:
#   в коде есть, в реестре нет   -> появилось новое, документ не знает;
#   в реестре есть, в тексте нет -> строку из документа удалили;
#   в реестре есть, в коде нет   -> документ описывает то, чего уже нет.
def code_list(cmd):
    return {line.strip() for line in sh(cmd).splitlines() if line.strip()}


def inventories():
    return [
        {
            "имя": "периодические задачи",
            "документ": OVERVIEW,
            "код": code_list(
                r"grep -oE 'go start[A-Za-z]+\(' cmd/server/main.go "
                r"| sed 's/^go //; s/($//'"),
            "реестр": {
                "startExpiryScheduler": "Архивация просроченных вложений",
                "startAccessDenialsArchiver": "Архивация журнала отказов",
                "startDailyStatusReset": "Суточный снимок и сброс",
                "startDailyPassReportSaver": "Суточный отчёт по проходам",
                "startOnlinePeakSnapshotter": "Фиксация пика посещаемости",
                "startLogPartitionWorker": "Обслуживание журнальных таблиц",
                "startReminderScheduler": "Напоминания согласующим",
            },
        },
        {
            "имя": "категории прав",
            "документ": OVERVIEW,
            "код": code_list(
                r"grep -oE 'Cat[A-Za-z]+ +=' "
                r"internal/services/permission_catalog.go | sed 's/ *=//'"),
            "реестр": {
                "CatNavigation": "Разделы навигации",
                "CatHeader": "Элементы верхней панели",
                "CatCenter": "Центр заявок",
                "CatDetail": "Карточки объектов",
                "CatRegistry": "Реестры",
                "CatAdmin": "Администрирование",
                "CatOverview": "Обзор и руководство",
                "CatTables": "Таблицы постов",
            },
        },
        {
            "имя": "разделы администрирования",
            "документ": OVERVIEW,
            "код": code_list(
                "ls frontend/src/views/admin/*.vue | xargs -n1 basename"),
            "реестр": {
                "AccessDenialsLog.vue": "Журнал отказов",
                "AdminPageShell.vue": "Администрирование",
                "AdminPermissionGroups.vue": "группы прав",
                "AdminRoles.vue": "Роли",
                "ApproversView.vue": "Принимающие заявки",
                "AttachmentTypesView.vue": "типы вложений",
                "BlacklistView.vue": "Чёрные списки",
                "CitizenshipView.vue": "Гражданства",
                "CompaniesView.vue": "Организации и компании",
                "DataProcessingView.vue": "Обработка данных",
                "DocumentsView.vue": "документы",
                "GuideManagementView.vue": "Обучение",
                "MarksView.vue": "марки транспорта",
                "NewsManagement.vue": "Новости и объявления",
                "NumberFormatsView.vue": "форматы регистрационных знаков",
                "OrganizationsView.vue": "Организации и компании",
                "PdAuditLog.vue": "обращений к персональным данным",
                "SystemControl.vue": "режим работ",
                "UnloadPlacesView.vue": "Места разгрузки",
                "UserControlView.vue": "Учётные записи",
                "UserTypesView.vue": "типы работников",
            },
        },
    ]


def check_inventories(texts, problems):
    for inv in inventories():
        text = texts[inv["документ"]]
        for key in sorted(inv["код"]):
            if key not in inv["реестр"]:
                problems.append(
                    "%s, %s: в коде появилось «%s», реестр сверки о нём не знает "
                    "- проверить, описано ли оно в документе"
                    % (inv["документ"], inv["имя"], key))
                continue
            if inv["реестр"][key] not in text:
                problems.append(
                    "%s, %s: «%s» есть в коде, но строка «%s» в документе не "
                    "найдена" % (inv["документ"], inv["имя"], key,
                                 inv["реестр"][key]))
        for key in sorted(inv["реестр"]):
            if key not in inv["код"]:
                problems.append(
                    "%s, %s: «%s» описан в реестре, но в коде его больше нет"
                    % (inv["документ"], inv["имя"], key))


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


def like(sample, value):
    """Число в том же виде, что стояло в тексте: с тем же разделителем разрядов."""
    sep = next((ch for ch in sample if ch in "  "), None)
    if sep is None:
        return str(value)
    body = str(value)
    parts = []
    while len(body) > 3:
        parts.insert(0, body[-3:])
        body = body[:-3]
    parts.insert(0, body)
    return sep.join(parts)


# Формы существительных после числа: 1 тест, 2 теста, 5 тестов. Подстановка
# числа без согласования оставляет «2 781 теста» - документ уходит заказчику
# с ошибкой в первой же таблице показателей.
FORMS = {
    "тест": ("тест", "теста", "тестов"),
    "файл": ("файл", "файла", "файлов"),
    "строка": ("строка", "строки", "строк"),
    "служба": ("служба", "службы", "служб"),
    "обработчик": ("обработчик", "обработчика", "обработчиков"),
    "модель": ("модель", "модели", "моделей"),
    "экран": ("экран", "экрана", "экранов"),
    "элемент": ("элемент", "элемента", "элементов"),
    "таблица": ("таблица", "таблицы", "таблиц"),
    "индекс": ("индекс", "индекса", "индексов"),
    "метод": ("метод", "метода", "методов"),
    "группа": ("группа", "группы", "групп"),
    "ключ": ("ключ", "ключа", "ключей"),
    "категория": ("категория", "категории", "категорий"),
}
WORD_BY_FORM = {form: stem for stem, forms in FORMS.items() for form in forms}


def agreed_form(stem, value):
    """Форма слова при числе value по правилам согласования."""
    one, few, many = FORMS[stem]
    tail100, tail10 = value % 100, value % 10
    if 11 <= tail100 <= 14:
        return many
    if tail10 == 1:
        return one
    if 2 <= tail10 <= 4:
        return few
    return many


def agree_after(text, pos, value):
    """Согласовать существительное, стоящее сразу после числа в позиции pos.

    Слово после предлога («в 317 файлах») остаётся в своём падеже: там число
    не управляет формой, и правка испортила бы фразу.
    """
    before = text[max(0, pos - 4):pos]
    if re.search(r"\b(в|на|из|по|до|от)\s$", before):
        return text
    match = re.match(r"(\s+)([А-Яа-яЁё]+)", text[pos:])
    if not match:
        return text
    word = match.group(2)
    stem = WORD_BY_FORM.get(word.lower())
    if not stem:
        return text
    form = agreed_form(stem, value)
    if form == word.lower():
        return text
    start = pos + len(match.group(1))
    return text[:start] + form + text[start + len(word):]


def apply_fix(texts, m):
    """Подставить посчитанные числа в текст. Возвращает список правок.

    Заменяются только группы якорей, то есть места, которые скрипт умеет
    посчитать. Замены идут с конца строки, чтобы правка одной группы не
    сдвигала границы следующей.
    """
    edits = []
    for doc, label, pattern, expected in anchors(m):
        while True:
            match = re.search(pattern, texts[doc])
            if not match:
                break
            spans = []
            for idx, (shown, actual) in enumerate(zip(match.groups(), expected), 1):
                if to_int(shown) == actual:
                    continue
                if not shown.strip().replace(" ", "").replace(" ", "").isdigit():
                    # Число записано словом: подстановка цифры испортила бы фразу.
                    continue
                spans.append((match.span(idx), like(shown, actual), shown.strip(),
                              actual))
            if not spans:
                break
            for (start, end), new, old, actual in sorted(spans, reverse=True):
                texts[doc] = texts[doc][:start] + new + texts[doc][end:]
                texts[doc] = agree_after(texts[doc], start + len(new), actual)
                edits.append("%s, %s: «%s» -> «%s»" % (doc, label, old, new))
            break

    # Согласование проверяется у всех показателей, а не только у изменённых:
    # неверная форма могла остаться от прошлой подстановки, когда число уже
    # совпадает и в замену не попадает.
    for doc, label, pattern, expected in anchors(m):
        match = re.search(pattern, texts[doc])
        if not match:
            continue
        for idx, actual in enumerate(expected, 1):
            span = match.span(idx)
            if span[0] < 0:
                continue
            before = texts[doc]
            texts[doc] = agree_after(texts[doc], span[1], actual)
            if texts[doc] != before:
                edits.append("%s, %s: согласовано слово после числа %d"
                             % (doc, label, actual))
                match = re.search(pattern, texts[doc])
                if not match:
                    break

    for match in list(re.finditer(r"создано ([\d  ]+?) специальных индексов",
                                  texts[OVERVIEW]))[::-1]:
        shown = match.group(1)
        if to_int(shown) != m["gin"]:
            start, end = match.span(1)
            new = like(shown, m["gin"])
            texts[OVERVIEW] = texts[OVERVIEW][:start] + new + texts[OVERVIEW][end:]
            edits.append("%s, поисковые индексы: «%s» -> «%s»"
                         % (OVERVIEW, shown.strip(), new))
    return edits


def write_doc(name, text):
    path = os.path.join(DOCS_DIR, name, name + ".md")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def main():
    quiet = "--quiet" in sys.argv
    fix = "--fix" in sys.argv
    m = metrics()
    problems, lost, checked = [], [], 0

    if fix:
        if behind_dev():
            print("Ветка отстала от origin/dev: сначала влить dev, "
                  "иначе в документ уедут числа старого кода.")
            return 1
        texts = {name: read_doc(name) for name in (OVERVIEW, DEPLOY)}
        edits = apply_fix(texts, m)
        for name, text in texts.items():
            write_doc(name, text)
        if edits:
            print("Подставлено значений: %d" % len(edits))
            for line in edits:
                print("  " + line)
            print("Пересобрать документы: python3 Документация/src/build_docs.py")
        else:
            print("Числа уже сходятся, правок не потребовалось.")
        return 0

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

    check_inventories(texts, problems)
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
    if lag:
        # Зелёного при отставании быть не может: числа считались по старому
        # коду, и совпадение с текстом означает лишь то, что документ отстал
        # ровно настолько же. Предупреждение без влияния на итог читается как
        # «всё в порядке» - ровно так проверка однажды и соврала.
        if not quiet:
            print("ИТОГ: недостоверно. Сверка шла против кода отставшей "
                  "ветки; влить dev и прогнать заново.")
        return 1
    if not problems and not lost and not quiet:
        print("Документация сходится с кодом: сверено утверждений %d, "
              "параметров %d." % (checked, len(env_defaults())))
    return 1 if problems or lost else 0


if __name__ == "__main__":
    sys.exit(main())
