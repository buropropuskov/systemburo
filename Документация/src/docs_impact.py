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
    (r"^internal/router/router\.go$|^docs/(docs\.go|swagger\.(json|yaml))$",
     "Техописание", "11 программный интерфейс: число методов и публичные роуты"),
    # Журнал запросов - обещание из раздела о защите информации, а пишет его
    # одно middleware. Пока файл был вне карты, затирание билетов в журнале
    # прошло мимо сверки и в документ не попало.
    (r"^internal/middleware/request_logger|^internal/(handlers|services)/"
     r"request_log", "Техописание",
     "13.8 журналы и прослеживаемость: состав записи и затираемые параметры"),
    (r"^internal/services/permission_(catalog|keys|service|resolver)|"
     r"^frontend/src/components/admin/(EffectivePermissionsTree|UserAccess"
     r"|GroupPermissionsModal|RolePermissionsModal)",
     "Техописание", "4.2 разграничение доступа, приложение А категорий прав"),
    (r"^internal/middleware/(jwt|permission|ratelimit)", "Оба",
     "13 защита информации; 6.3 ограничения"),
    (r"^internal/auth/|^internal/services/auth_service\.go$", "Техописание",
     "13.1 проверка подлинности"),
    # Сверка ключа при запуске и перевод данных на новый ключ обещаны в двух местах
    # сразу: техописание отвечает за то, что подмена ключа останавливает запуск,
    # руководство - за порядок хранения и саму смену. Без этой строки новые файлы
    # шли в «вне карты» и требовали разбираться заново на каждой сверке.
    (r"^internal/crypto/|^internal/database/(encryption_guard|reencrypt)|"
     r"^cmd/server/reencrypt\.go$", "Оба",
     "техописание 13.6 защита персональных данных при хранении; "
     "руководство 9.12 ключ шифрования и 13.5 сообщения при запуске"),
    (r"^internal/models/pd|^internal/middleware/pd_|^internal/services/pd_|"
     r"^internal/handlers/(consent|settings_pd)|_mask|^internal/services/"
     r"(approver_mask|audit_reader)|^frontend/src/utils/formatName\.js$|"
     r"^frontend/src/components/admin/DataProcessingSettings\.vue$|"
     r"^frontend/src/views/DataProcessingView\.vue$|"
     r"^internal/services/consent_service|^frontend/src/components/"
     r"PDConsentOverlay\.vue$|^frontend/src/(stores|api)/pdConsent\.js$", "Оба",
     "13.6 и 13.7 персональные данные; критерии раздела 5"),
    # Жизненный цикл заявки описан словами интерфейса: состав формы подачи и
    # действия в списке заявок и есть то, что читает заказчик в разделе 5.1.
    (r"^frontend/src/components/CreateApplication/|"
     r"^frontend/src/views/ApplicationsCenter\.vue$|"
     r"^internal/services/application_service\.go$", "Техописание",
     "5.1 порядок обработки заявки, 5.3 прозрачность обработки"),
    (r"^cmd/server/main\.go$", "Техописание",
     "9.3 периодические задачи"),
    (r"^internal/services/(application_helpers|.*blacklist.*)\.go$",
     "Техописание", "5.2 чёрные списки, 7 поиск"),
    (r"^internal/normalize/|^frontend/src/utils/searchVariants\.js$|"
     r"^internal/services/directory_moderation", "Техописание",
     "7.2 приведение сведений к единому виду, модерация справочников"),
    # Сквозной поиск: состав разделов выдачи и то, какие совпадения он распознаёт,
    # описаны в 7.1. Провайдер добавляют одним файлом, и без этой строки новый раздел
    # выдачи не двигал бы ни одного числа и прошёл бы мимо сверки.
    (r"^internal/services/search_|^internal/handlers/search\.go$|"
     r"^frontend/src/components/GlobalSearch(Palette|Panel)\.vue$|"
     r"^frontend/src/composables/useGlobalSearch\.js$|"
     r"^frontend/src/constants/(searchTargets|navSections|searchActions)\.js$",
     "Техописание", "7.1 сквозной поиск: состав разделов и распознаваемые совпадения"),
    (r"^internal/services/report_|^internal/services/processing_",
     "Критерии", "4 показатели: метрики и разрезы"),
    # Уведомления - обещание из 6.2: какие события приходят, как они доставляются
    # наружу и кому не отправляются вовсе. Гейт по состоянию учётной записи
    # (заблокированным и архивным не шлём) живёт в сервисе, а карта его не знала.
    (r"^internal/services/(notification|push)_|"
     r"^internal/handlers/notification|"
     r"^frontend/src/views/NotificationSettingsView\.vue$", "Техописание",
     "6.2 уведомления: состав событий, доставка наружу, кому не отправляются"),
    # Обучающие туры: их состав, деление на главы и продолжение с прерванного
    # места описаны в 8.1 и повторены словами в руководствах ролей.
    (r"^frontend/src/components/onboarding/|"
     r"^frontend/src/stores/onboarding\.js$|"
     r"^frontend/src/composables/useOnboarding\.js$", "Техописание",
     "8.1 обучающие туры: состав, главы, продолжение с прерванного места"),
    # Экран сбоя читает пользователь, а не разработчик: что на нём написано и куда
    # уводят его кнопки - часть 8.3.
    (r"^frontend/src/views/Error500\.vue$|"
     r"^frontend/src/composables/useBugReport\.js$|"
     r"^internal/handlers/bug_report", "Техописание",
     "8.3 сообщение о сбое: экран ошибки и отправка сообщения"),
    # По разделу мониторинга запросов заказчик снимает показатели пилота, поэтому
    # состав шапки и журнала на экране входит в обещание.
    (r"^frontend/src/components/(monitoring|statistics)/|"
     r"^frontend/src/views/RequestsView\.vue$|"
     r"^frontend/src/utils/requestLogs", "Критерии",
     "4 показатели: как они снимаются в разделах «Мониторинг запросов» "
     "и «Аналитика»"),
    # --- ролевые руководства -------------------------------------------------
    # Разделы здесь названы словами, а не номерами: руководства пишутся
    # срезами, и номера разделов до конца работы над документом плавают.
    # Приведём к номерам, когда комплект будет собран целиком.
    (r"^frontend/src/components/CreateApplication/",
     "Руководство пользователя", "оформление и подача заявки: поля форм вложений"),
    (r"^frontend/src/components/(AccountComponent|UserApplications)\.vue$|"
     r"^frontend/src/views/(CarsView|EmployeeView|NewsAndReview|"
     r"NotificationSettingsView)\.vue$",
     "Руководство пользователя",
     "личный кабинет, свои справочники, обзор и новости, настройка уведомлений"),
    (r"^frontend/src/components/ApplicationDetail/",
     "Согласующий и принимающий", "карточка заявки: состав, решения, вопросы"),
    (r"^frontend/src/views/AccessibleAttachmentsView\.vue$|"
     r"^frontend/src/components/(TablesComponent|FactTable|PeopleTable|CarsTable|"
     r"PassReportModal|FactPassModal)\.vue$",
     "Руководство охранника",
     "доступные мне, таблицы поста, отметка прохода, отчёт по смене"),
    (r"^frontend/src/views/admin/|^frontend/src/views/AdminSettings\.vue$|"
     r"^frontend/src/views/(FeedbackPage|TableVersionsView)\.vue$|"
     r"^frontend/src/components/(UserControl|TableConstructor|NewsManagement|"
     r"DocumentsManagement|UserTypes|ApplicationApprovers)\.vue$|"
     r"^frontend/src/components/(Citizenship|Companies|Marks|Organizations)"
     r"Management\.vue$|^frontend/src/components/NumberFormat\.vue$|"
     r"^frontend/src/components/UnloadPlaces/|"
     r"^frontend/src/components/admin/blacklist/",
     "Руководство администратора",
     "разделы администрирования: учётные записи, права, справочники, чёрные "
     "списки, обратная связь, настройки"),
    (r"^internal/realtime/|^internal/handlers/events\.go$", "Техописание",
     "12 обновление без перезагрузки"),
    (r"^internal/upload/", "Техописание", "13.4 загрузка файлов"),
    (r"^internal/services/attachment_blank|^internal/services/attachment_template"
     r"|^internal/handlers/attachment_blank|^internal/download/|"
     r"^frontend/src/components/applications/DownloadBlanksModal\.vue$|"
     r"^frontend/src/api/attachment-templates\.js$", "Техописание",
     "6 вложения и бланки: формирование и выдача печатных форм"),
    (r"^frontend/src/components/(GuideManagement|news/UserGuideModal|"
     r"DocumentsManagement)", "Техописание",
     "8.2 встроенное руководство, 6 разделы администрирования"),
    # Подписи состояний общие для заявок и для выгрузки бланков: словарь один,
    # а описаны они в двух разных разделах.
    (r"^frontend/src/components/ui/StatusBadge\.vue$", "Техописание",
     "5.1 состояния заявки, 6.1 состояния выгрузки бланка"),
    # Файловый архив описан с двух сторон: что он делает - в техописании,
    # как его готовят и обслуживают на сервере - в руководстве.
    (r"^internal/services/(blank_export|archive_path|settings_archive)|"
     r"^internal/diskspace/|^internal/handlers/(file_archive|blank_archive)|"
     r"^internal/models/blank_export|^frontend/src/api/fileArchive\.js$|"
     r"^frontend/src/components/admin/TemplatePatternField\.vue$|"
     r"^frontend/src/(components/admin/FileArchive/|views/admin/FileArchive)|"
     r"^cmd/server/archive\.go$|^internal/services/application_attachment_service"
     r"|^frontend/src/components/AttachmentsManagement\.vue$",
     "Оба", "6.1 файловый архив бланков; руководство 9.6 и приложение Б"),
    # Уборка данных и сроки хранения: обещание о том, что и сколько живёт в базе,
    # плюс две консольные команды из приложения В.
    (r"^cmd/server/(cleanup|storage)\.go$|^internal/database/retention\.go$|"
     r"^internal/services/(retention|trash_service)", "Оба",
     "10.2 сохранность сведений; руководство 9 уборка, сроки хранения, "
     "приложение В"),
    (r"^internal/services/login_guard", "Оба",
     "13.1 проверка подлинности: пороги и сроки блокировки; приложение Б"),
    # Наполнение стенда вымышленными данными: раздел 8.4 описывает и команды, и вид
    # их вывода, и объём заготовок. Правило появилось задним числом - правки наливки
    # не поднимали тревогу вовсе, и вывод команды разошёлся с описанным.
    (r"^cmd/server/fake\.go$|^internal/fakedata/", "Руководство",
     "8.4 наполнение стенда вымышленными данными"),
    (r"^internal/blankpath/", "Оба",
     "6.1 раскладка файлов архива; руководство 9.6 шаблоны и плейсхолдеры"),
    (r"^internal/(services|handlers)/maintenance|"
     r"^frontend/src/views/(Maintenance\.vue|admin/SystemControl\.vue)$",
     "Руководство", "9.5 режим технических работ"),
    (r"^internal/services/settings_service|^frontend/src/views/AdminSettings",
     "Руководство", "9 настройки системы, задаваемые из интерфейса"),
    (r"^frontend/src/components/UserControl\.vue$|"
     r"^frontend/src/views/admin/UserControlView\.vue$", "Техописание",
     "4.1 типы учётных записей, 6 раздел администрирования «Учётные записи»"),
    (r"^internal/services/(analytics|statistics)|"
     r"^frontend/src/utils/presence\.js$", "Критерии",
     "4.1 скорость обработки заявок, показатели посещаемости"),
    (r"^\.github/workflows/", "Техописание",
     "13.9 контроль защищённости, 14.3 конвейер"),
    # Нагрузочные сценарии руководство упоминает ровно один раз - в перечне того,
    # что в архив поставки НЕ входит. Своего описания у них нет, поэтому правка
    # внутри каталога ничего в документе не двигает и в смысловой сигнал не идёт.
    (r"_test\.go$|\.spec\.(js|ts|cjs|mjs)$|^internal/testutil/|^frontend/e2e/|"
     r"^load-tests/",
     "Техописание",
     TESTS_WHERE),
]


def sh(cmd, check=False):
    """Вывод команды. С check=True несработавшая команда останавливает сверку.

    Без этого git, отказавшийся разбирать диапазон, отдаёт пустой stdout, и
    вызывающий читает его как «изменений нет»: проверка остаётся зелёной ровно
    там, где сломана.
    """
    p = subprocess.run(cmd, shell=True, cwd=REPO, capture_output=True, text=True)
    if check and p.returncode != 0:
        raise SystemExit("Команда не выполнилась: %s\n%s"
                         % (cmd, p.stderr.strip()))
    return p.stdout.strip()


def changed(rev_range):
    if rev_range:
        out = sh("git diff --name-only %s" % rev_range, check=True)
    else:
        out = sh("git status --porcelain | awk '{print $NF}'")
    return [line for line in out.splitlines() if line]


COMMENT_LINE = re.compile(r"^\s*(//|#|\*|/\*|\*/|<!--)")
STYLE_OPEN = re.compile(r"<style\b", re.I)
STYLE_CLOSE = re.compile(r"</style\s*>", re.I)
HUNK = re.compile(r"^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@")


def _norm(line):
    return re.sub(r"\s+", "", line)


def _revisions(rev_range):
    """Пара ревизий «до» и «после»; None означает рабочее дерево."""
    if not rev_range:
        return "HEAD", None
    if ".." in rev_range:
        old, new = rev_range.split("..", 1)
        return old or "HEAD", new or None
    return rev_range, None


def _file_lines(path, rev):
    if rev is None:
        full = os.path.join(REPO, path)
        if not os.path.exists(full):
            return []
        with open(full, encoding="utf-8", errors="replace") as fh:
            return fh.read().splitlines()
    out = subprocess.run("git show %s:%s" % (rev, path), shell=True, cwd=REPO,
                         capture_output=True, text=True)
    return out.stdout.splitlines()


def _style_lines(path, rev, cache):
    """Номера строк файла, попадающие внутрь блока <style>.

    Правка оформления компонента - цвет, тень, отступ - вида системы не
    описывает ни один раздел документа, но файл она трогает так же, как
    переписанное условие в скрипте. Снятая тень у восьми тумблеров поднимала
    ту же тревогу, что новая проверка прав, и обесценивала её.
    """
    key = (path, rev)
    if key in cache:
        return cache[key]
    inside, marks = False, set()
    for num, line in enumerate(_file_lines(path, rev), 1):
        opened = not inside and STYLE_OPEN.search(line)
        if opened or inside:
            marks.add(num)
            inside = not STYLE_CLOSE.search(line)
    cache[key] = marks
    return marks


def weights(rev_range):
    """Сколько в каждом файле изменилось строк, несущих поведение.

    Карта срабатывает по имени файла, и перестановка импортов трогает файл
    ровно так же, как переписанная проверка пароля. Пока оба случая попадали
    в «важное», предупреждение переставали читать - а среди шума терялась
    единственная строка, менявшая обещание заказчику.

    Строка считается значащей, если после снятия пробелов она не нашлась по
    другую сторону правки. Это отсеивает выравнивание gofmt, переупорядочение
    импортов и переносы, оставляя изменение смысла. Комментарии тоже не в счёт:
    поведение системы от них не меняется. Стилевые блоки компонентов - тоже,
    см. _style_lines.
    """
    diff = sh("git diff --unified=0 %s" % rev_range) if rev_range \
        else sh("git diff --unified=0")
    old_rev, new_rev = _revisions(rev_range)
    styles, old_no, new_no = {}, 0, 0
    result, path, added, removed = {}, None, [], []

    def flush():
        if path is None:
            return
        rest = list(added)
        for line in removed:
            if line in rest:
                rest.remove(line)
        back = list(removed)
        for line in added:
            if line in back:
                back.remove(line)
        result[path] = len(rest) + len(back)

    def styled(rev, number):
        """Строка лежит в стилевом блоке компонента."""
        return path.endswith(".vue") and number in _style_lines(path, rev, styles)

    for line in diff.splitlines():
        if line.startswith("+++ b/"):
            flush()
            path, added, removed = line[6:], [], []
        elif line.startswith("diff --git "):
            flush()
            path, added, removed = None, [], []
        elif line.startswith("@@"):
            found = HUNK.match(line)
            if found:
                old_no, new_no = int(found.group(1)), int(found.group(2))
        elif path and line.startswith("+") and not line.startswith("+++"):
            body = line[1:]
            if body.strip() and not COMMENT_LINE.match(body) \
                    and not styled(new_rev, new_no):
                added.append(_norm(body))
            new_no += 1
        elif path and line.startswith("-") and not line.startswith("---"):
            body = line[1:]
            if body.strip() and not COMMENT_LINE.match(body) \
                    and not styled(old_rev, old_no):
                removed.append(_norm(body))
            old_no += 1
    flush()
    return result


LEVELS = ['^internal/config/config', '^docker-compose', '^nginx/', '^Dockerfile', '^Makefile', '^scripts/', '^cmd/seed/', '^internal/database/migrate', '^internal/database/partitions', '^internal/router/router', '^internal/services/permission_', '^internal/middleware/(jwt', '^internal/auth/', '^internal/crypto/', '^internal/models/pd', '^\\.github/workflows/']


def level(pattern):
    """Насколько срочно смотреть раздел: важное - обещания заказчику."""
    return "высокая" if any(pattern.startswith(p) for p in LEVELS) else "обычная"


def impact(files):
    """Разделы документации, задетые файлами; порядок карты сохраняется."""
    hits = []
    for pattern, doc, where in MAP:
        matched = [f for f in files if re.search(pattern, f)]
        if matched:
            hits.append((doc, where, matched, level(pattern)))
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


def mark_base():
    """Коммит dev, на котором документацию сверяли в последний раз.

    Точка отсчёта хранится в файле отметки: по последнему коммиту каталога
    документации её не вычислить, потому что ветка документации содержит весь
    код dev и разница между ними состоит из одних документов.
    """
    if not os.path.exists(MARK):
        return None
    with open(MARK, encoding="utf-8") as fh:
        return fh.read().strip().split()[0] or None


def debt_range():
    base = mark_base()
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
        base = mark_base()
        if not base:
            if not quiet:
                print("Отметки синхронизации нет. Поставить после сверки: "
                      "python3 Документация/src/docs_impact.py --mark")
            return 0
        # Отметка на коммите, которого в репозитории нет, - не «долга нет».
        # Так уже было: конфликт слияния в файле отметки развели вручную, хеш
        # склеился из двух половин, и четыре дня подряд сверка отвечала
        # «Изменений нет» на 21 накопившийся коммит. Молчать тут нельзя.
        if not sh("git rev-parse --verify --quiet %s^{commit}" % base):
            print("Отметка синхронизации указывает на коммит %s, которого нет "
                  "в репозитории." % base[:12])
            print("Обычно это криво разрешённый конфликт в "
                  "Документация/src/.synced-dev. Взять из истории файла "
                  "последнее рабочее значение (git log -p) либо, сверив "
                  "документацию заново, поставить отметку: "
                  "python3 Документация/src/docs_impact.py --mark")
            return 1
        rev = "%s..origin/dev" % base
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

    # Файл, в котором сменился только порядок импортов или ширина отступа,
    # поведения не менял и в список задетых разделов попадать не должен.
    # Про новые файлы вес неизвестен, они считаются значащими.
    weight = weights(rev)
    files = [f for f in files if weight.get(f, 1) > 0]
    if not files:
        if not quiet:
            print("%s: правки есть, но смысл кода не менялся "
                  "(форматирование, импорты, комментарии)." % title)
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
    важные = [h for h in sense if h[3] == "высокая"]
    прочие = [h for h in sense if h[3] != "высокая"]

    def покажи(группа, заголовок):
        if not группа:
            return
        print("  %s" % заголовок)
        for doc, where, matched, _lvl in группа:
            # Пример берётся самый весомый: по нему сразу видно, о правке
            # какого масштаба речь - строка или переписанный сервис.
            matched = sorted(matched, key=lambda f: -weight.get(f, 1))
            example = matched[0]
            more = " и ещё %d" % (len(matched) - 1) if len(matched) > 1 else ""
            total = sum(weight.get(f, 1) for f in matched)
            print("    [%s] %s" % (doc, where))
            print("        из-за %s%s, изменено по существу строк: %d"
                  % (example, more, total))

    if sense:
        print("%s: файлов %d." % (title, len(files)))
        # Порядок не косметика: важное - это обещания, которые заказчик
        # исполняет или проверяет (вход, права, персональные данные, параметры
        # установки, команды, конвейер). Остальное описывает поведение: сверить
        # нужно, но пропуск не меняет порядок действий на месте.
        покажи(важные, "ВАЖНОЕ - обещания заказчику, описать обязательно:")
        покажи(прочие, "Прочее - сверить описание поведения:")

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
