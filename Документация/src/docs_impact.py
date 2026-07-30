#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Что в документации задето изменениями в коде.

Держать в голове соответствие «файл кода - раздел документа» невозможно, и
именно на этом документация отстаёт от системы: правку сделали, документ
забыли. Скрипт сопоставляет изменённые файлы с картой и называет разделы,
которые нужно открыть и сверить.

Карта намеренно грубая: лучше лишний раз открыть раздел, чем пропустить
изменение. Пустой ответ означает «в этих файлах документируемого поведения
нет», а не «проверять не нужно».

Запуск:
    python3 Документация/src/docs_impact.py                 # незакоммиченное
    python3 Документация/src/docs_impact.py origin/dev..HEAD
    python3 Документация/src/docs_impact.py --debt          # долг против dev
"""
import os
import re
import subprocess
import sys

SRC_DIR = os.path.dirname(os.path.abspath(__file__))
DOCS_DIR = os.path.dirname(SRC_DIR)
REPO = os.path.dirname(DOCS_DIR)

# Долг бывает двух видов, и смешивать их нельзя. Правка тестов двигает только
# счётчики в таблице показателей: это закрывается подстановкой чисел и терпит
# до следующей сборки. Всё остальное может менять описанное поведение, и это
# требует прочитать код. Пока оба вида кричали одинаково, редкий важный сигнал
# тонул в частом неважном - и документация отставала именно так.
TESTS_WHERE = "14.1 количественные показатели тестов"

# (шаблон пути, документ, что смотреть)
MAP = [
    (r"^internal/config/config\.go$", "Руководство",
     "6. Параметры конфигурации и приложение Б"),
    (r"^docker-compose\.prod\.yml$", "Руководство",
     "6.4 параметры базы, 5.9 состав служб, приложение А"),
    (r"^docker-compose\.(base|staging)\.yml$", "Руководство",
     "4.3 наложение файлов, 8 стенд, приложение В"),
    (r"^nginx/", "Руководство",
     "7 сертификат, 12 мониторинг, 13.3 и 13.10 диагностика, приложение А"),
    (r"^Dockerfile|^frontend/Dockerfile$", "Руководство",
     "4 состав поставки, 12.1 проверка состояния"),
    (r"^Makefile$", "Руководство",
     "5 установка, 8 стенд, 9.1 команды управления"),
    (r"^scripts/", "Руководство",
     "5.4 файл параметров, 7 сертификат, приложение В"),
    (r"^cmd/seed/", "Руководство", "5.8 создание администратора"),
    (r"^internal/database/migrate\.go$", "Оба",
     "10 организация данных, 10.6 изменения схемы; число таблиц и индексов"),
    (r"^internal/database/partitions\.go$", "Техописание",
     "10.1 разделение журналов, 13.6 сроки хранения"),
    (r"^internal/router/router\.go$", "Техописание",
     "11 программный интерфейс: число методов и публичные роуты"),
    (r"^internal/services/permission_(catalog|keys|service|resolver)",
     "Техописание", "4.2 разграничение доступа, приложение А категорий прав"),
    (r"^internal/middleware/(jwt|permission|ratelimit)", "Оба",
     "13 защита информации; 6.3 ограничения"),
    (r"^internal/auth/|^internal/services/auth_service\.go$", "Техописание",
     "13.1 проверка подлинности"),
    (r"^internal/crypto/", "Техописание",
     "13.5 защита персональных данных при хранении"),
    (r"^internal/models/pd|^internal/middleware/pd_audit\.go$", "Оба",
     "13.6 и 13.7 персональные данные; критерии раздела 5"),
    (r"^cmd/server/main\.go$", "Техописание",
     "9.3 периодические задачи"),
    (r"^internal/services/(application_helpers|.*blacklist.*)\.go$",
     "Техописание", "5.2 чёрные списки, 7 поиск"),
    (r"^internal/normalize/|^frontend/src/utils/searchVariants\.js$",
     "Техописание", "7 поиск и приведение сведений к единому виду"),
    (r"^internal/services/report_|^internal/services/processing_",
     "Критерии", "4 показатели: метрики и разрезы"),
    (r"^internal/realtime/|^internal/handlers/events\.go$", "Техописание",
     "12 обновление без перезагрузки"),
    (r"^internal/upload/", "Техописание", "13.4 загрузка файлов"),
    (r"^internal/services/attachment_blank|^internal/services/attachment_template",
     "Техописание", "6 вложения и бланки: формирование печатных форм"),
    (r"^internal/(services|handlers)/maintenance|"
     r"^frontend/src/views/(Maintenance\.vue|admin/SystemControl\.vue)$",
     "Руководство", "9.5 режим технических работ"),
    (r"^internal/services/(analytics|statistics)", "Критерии",
     "4.1 скорость обработки заявок"),
    (r"^\.github/workflows/", "Техописание",
     "13.9 контроль защищённости, 14.3 конвейер"),
    (r"_test\.go$|\.spec\.(js|ts)$", "Техописание", TESTS_WHERE),
]


def sh(cmd):
    return subprocess.run(cmd, shell=True, cwd=REPO, capture_output=True,
                          text=True).stdout.strip()


def changed(rev_range):
    if rev_range:
        out = sh("git diff --name-only %s" % rev_range)
    else:
        out = sh("git status --porcelain | awk '{print $NF}'")
    return [line for line in out.splitlines() if line]


def impact(files):
    """Разделы документации, задетые файлами; порядок карты сохраняется."""
    hits = []
    for pattern, doc, where in MAP:
        matched = [f for f in files if re.search(pattern, f)]
        if matched:
            hits.append((doc, where, matched))
    return hits


# Пути, документируемого поведения заведомо не несущие: оформление и вёрстка.
# Всё остальное, чего карта не знает, показывается как пробел - карта конечна,
# и её незнание однажды уже прошло за «изменений нет».
COSMETIC = (
    r"\.(css|scss|svg|png|jpg|ico|woff2?)$",
    r"^frontend/src/assets/",
    r"^\.gitignore$|^\.dockerignore$|^README",
)


def uncovered(files):
    """Файлы вне карты, которые могут нести документируемое поведение."""
    known = set()
    for pattern, _doc, _where in MAP:
        known |= {f for f in files if re.search(pattern, f)}
    rest = [f for f in files if f not in known]
    return [f for f in rest if not any(re.search(p, f) for p in COSMETIC)]


MARK = os.path.join(SRC_DIR, ".synced-dev")


def debt_range():
    """Диапазон коммитов dev, не рассмотренных при правке документации.

    Точка отсчёта хранится в файле отметки: по последнему коммиту каталога
    документации её не вычислить, потому что ветка документации содержит весь
    код dev и разница между ними состоит из одних документов.
    """
    if not os.path.exists(MARK):
        return None
    with open(MARK, encoding="utf-8") as fh:
        base = fh.read().strip().split()[0]
    return "%s..origin/dev" % base if base else None


def mark_synced():
    """Запомнить, на каком состоянии dev документацию сверяли."""
    head = sh("git rev-parse origin/dev")
    if not head:
        print("Не удалось определить origin/dev, отметка не поставлена.")
        return 1
    date = sh("git log -1 --format=%cs origin/dev")
    with open(MARK, "w", encoding="utf-8") as fh:
        fh.write("%s %s\n" % (head, date))
    print("Отметка синхронизации: dev %s от %s" % (head[:8], date))
    return 0


def main():
    args = [a for a in sys.argv[1:] if a != "--quiet"]
    quiet = "--quiet" in sys.argv

    if args and args[0] == "--mark":
        return mark_synced()

    if args and args[0] == "--debt":
        rev = debt_range()
        if not rev:
            if not quiet:
                print("Отметки синхронизации нет. Поставить после сверки: "
                      "python3 Документация/src/docs_impact.py --mark")
            return 0
        title = "Пришло в dev после последней сверки документации"
    else:
        rev = args[0] if args else ""
        title = "Изменения" if rev else "Незакоммиченные изменения"

    # Сами документы из разбора исключаются: интересует изменившийся код.
    files = [f for f in changed(rev) if not f.startswith("Документация/")]
    if not files:
        if not quiet:
            print("Изменений нет.")
        return 0

    hits = impact(files)
    blind = uncovered(files)

    if not hits and not blind:
        if not quiet:
            print("%s: файлов %d, документируемого поведения не затронуто."
                  % (title, len(files)))
        return 0

    if not hits and blind:
        print("%s: файлов %d. Карта разделов их не знает, проверить глазами "
              "(%d):" % (title, len(files), len(blind)))
        for name in blind[:8]:
            print("  " + name)
        if len(blind) > 8:
            print("  и ещё %d" % (len(blind) - 8))
        print("Описанное поведение задето - дополнить карту в docs_impact.py.")
        return 0

    sense = [h for h in hits if h[1] != TESTS_WHERE]
    numeric = [h for h in hits if h[1] == TESTS_WHERE]

    if sense:
        print("%s: файлов %d. Прочитать код и сверить разделы:"
              % (title, len(files)))
        for doc, where, matched in sense:
            example = matched[0]
            more = " и ещё %d" % (len(matched) - 1) if len(matched) > 1 else ""
            print("  [%s] %s" % (doc, where))
            print("      из-за %s%s" % (example, more))

    if numeric:
        files_touched = len(numeric[0][2])
        if sense:
            print("Плюс сдвинулись показатели тестов (файлов %d)."
                  % files_touched)
        else:
            print("%s: файлов %d. Смысловых изменений нет, сдвинулись только "
                  "показатели тестов (файлов %d)." % (title, len(files),
                                                      files_touched))
        print("Закрывается без чтения кода: doc_facts.py --fix и пересборка.")
    elif sense:
        print("Сверка чисел: python3 Документация/src/doc_facts.py")

    if blind:
        print("Вне карты, посмотреть глазами (%d): %s%s"
              % (len(blind), ", ".join(blind[:4]),
                 " и ещё %d" % (len(blind) - 4) if len(blind) > 4 else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
