"""Генератор презентации к защите ВКР Суднишникова Н.М.

Стиль: TNR, заголовки 36pt в верхнем левом углу, подзаголовки 20pt, текст 16pt,
выравнивание по левому краю, ключевые слова жирным. Палитра строгая:
navy (#1F3A5F) + серые оттенки.
"""

from __future__ import annotations

import copy
import pathlib
import re
import shutil

from PIL import Image

from pptx import Presentation
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN
from pptx.util import Emu, Pt, Inches

ROOT = pathlib.Path(__file__).resolve().parent.parent
TEMPLATE = pathlib.Path(
    "/home/washka/.var/app/org.telegram.desktop/data/TelegramDesktop/tdata/temp_data/Шаблон презентации_new1 (1).pptx"
)
IMAGES = ROOT / "images"
OUTPUT = ROOT / "output" / "vkr_nikita_presentation.pptx"
WORK = ROOT / "output" / "_presentation_work.pptx"

STUDENT_NAME = "Суднишников Никита Михайлович"
STUDENT_GROUP = "ЭФБО-06-22"
SUPERVISOR = "Бочаров Михаил Иванович"
SUPERVISOR_TITLE = "к.т.н., доцент кафедры индустриального программирования"
VKR_TITLE = (
    "«Разработка веб-приложения для управления техническим "
    "обслуживанием и ремонтом оборудования предприятия»"
)

FONT_NAME = "Times New Roman"
COLOR_NAVY = RGBColor(0x1F, 0x3A, 0x5F)
COLOR_TEXT = RGBColor(0x22, 0x22, 0x22)
COLOR_GRAY = RGBColor(0x55, 0x55, 0x55)


# ---------- Утилиты ----------

def duplicate_slide(prs, src_index: int):
    src = prs.slides[src_index]
    new_slide = prs.slides.add_slide(src.slide_layout)
    for shape in list(new_slide.shapes):
        sp = shape._element
        sp.getparent().remove(sp)
    for shape in src.shapes:
        el = shape._element
        new_slide.shapes._spTree.insert_element_before(copy.deepcopy(el), "p:extLst")
    for rel in src.part.rels.values():
        if "notesSlide" in rel.reltype:
            continue
        if rel.is_external:
            new_slide.part.rels.get_or_add_ext_rel(rel.reltype, rel.target_ref)
        else:
            new_slide.part.rels.get_or_add(rel.reltype, rel.target_part)
    return new_slide


def reorder_slides(prs, new_order: list[int]) -> None:
    slides = prs.slides._sldIdLst
    slide_list = list(slides)
    if sorted(new_order) != list(range(len(slide_list))):
        raise ValueError(f"bad order: {new_order}")
    for sld in slide_list:
        slides.remove(sld)
    for idx in new_order:
        slides.append(slide_list[idx])


def remove_slide(prs, index: int) -> None:
    sldIdLst = prs.slides._sldIdLst
    slide_elems = list(sldIdLst)
    sld = slide_elems[index]
    rId = sld.rId
    sldIdLst.remove(sld)
    try:
        prs.part.rels.pop(rId)
    except KeyError:
        pass


def remove_shapes(slide, names: set[str]) -> None:
    for shape in list(slide.shapes):
        if shape.name in names:
            sp = shape._element
            sp.getparent().remove(sp)


def find_shape(slide, name: str):
    for shape in slide.shapes:
        if shape.name == name:
            return shape
    return None


def set_shape_rect(shape, left: float, top: float, width: float, height: float) -> None:
    shape.left = Inches(left)
    shape.top = Inches(top)
    shape.width = Inches(width)
    shape.height = Inches(height)


def add_notes(slide, text: str) -> None:
    slide.notes_slide.notes_text_frame.text = text


# ---------- Набор текста (TNR + жирные маркеры + выравнивание) ----------

_BOLD_RX = re.compile(r"(\*\*[^*]+\*\*)")


def _render_runs(paragraph, text: str, *, font_size: int, bold: bool, color: RGBColor) -> None:
    """Разбирает **...** как bold-фрагменты и добавляет соответствующие runs."""
    for part in _BOLD_RX.split(text):
        if not part:
            continue
        if part.startswith("**") and part.endswith("**"):
            inner = part[2:-2]
            run = paragraph.add_run()
            run.text = inner
            run.font.name = FONT_NAME
            run.font.size = Pt(font_size)
            run.font.bold = True
            run.font.color.rgb = color
        else:
            run = paragraph.add_run()
            run.text = part
            run.font.name = FONT_NAME
            run.font.size = Pt(font_size)
            run.font.bold = bold
            run.font.color.rgb = color


def write_text(shape, lines, *, font_size: int = 16, bold: bool = False,
               color: RGBColor = COLOR_TEXT, align: str = "left",
               first_line_bold: bool = False) -> None:
    """Записывает набор строк в text_frame.

    - Каждая строка → отдельный параграф.
    - В строке `**слово**` даёт полужирный run.
    - Первый абзац становится жирным, если `first_line_bold`.
    """
    tf = shape.text_frame
    tf.word_wrap = True
    tf.clear()
    if isinstance(lines, str):
        lines = [lines]

    align_map = {"left": PP_ALIGN.LEFT, "center": PP_ALIGN.CENTER, "right": PP_ALIGN.RIGHT}
    pp_align = align_map.get(align, PP_ALIGN.LEFT)

    for i, line in enumerate(lines):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = pp_align
        is_bold = bold or (first_line_bold and i == 0)
        _render_runs(p, line, font_size=font_size, bold=is_bold, color=color)


def insert_fitted_image(slide, path: pathlib.Path, *,
                        box: tuple[float, float, float, float]) -> None:
    """Вставляет картинку в прямоугольник (x, y, w, h) в дюймах, сохраняя пропорции."""
    bx, by, bw, bh = box
    with Image.open(path) as im:
        iw, ih = im.size
    ratio = iw / ih
    box_ratio = bw / bh
    if ratio > box_ratio:
        w = bw
        h = bw / ratio
    else:
        h = bh
        w = bh * ratio
    x = bx + (bw - w) / 2
    y = by + (bh - h) / 2
    slide.shapes.add_picture(str(path), Inches(x), Inches(y),
                             width=Inches(w), height=Inches(h))


# ---------- Заполнение слайдов ----------

def fill_slide_1(slide) -> None:
    """Титульный слайд шаблона МИРЭА с нашим контентом."""
    title_shape = find_shape(slide, "Заголовок 1") or find_shape(slide, "TextBox 19")
    # TextBox 19 — тема работы (крупно).
    tb19 = find_shape(slide, "TextBox 19")
    if tb19:
        write_text(tb19, [VKR_TITLE], font_size=24, bold=True,
                   color=COLOR_NAVY, align="center")
    tb20 = find_shape(slide, "TextBox 20")
    if tb20:
        write_text(
            tb20,
            [
                "**Подготовил:**",
                f"студент группы {STUDENT_GROUP}",
                STUDENT_NAME,
                "",
                "**Руководитель:**",
                SUPERVISOR_TITLE,
                SUPERVISOR,
            ],
            font_size=14, color=COLOR_TEXT, align="left",
        )
    add_notes(
        slide,
        "Добрый день, уважаемые члены государственной экзаменационной комиссии! "
        f"Меня зовут Никита Суднишников, группа {STUDENT_GROUP}. "
        "Тема моей бакалаврской работы — разработка веб-приложения для управления "
        "техническим обслуживанием и ремонтом оборудования предприятия. "
        f"Научный руководитель — {SUPERVISOR}.",
    )


def fill_slide_2(slide) -> None:
    """Актуальность."""
    # Заголовок в верхний левый.
    tb9 = find_shape(slide, "TextBox 9")
    if tb9:
        set_shape_rect(tb9, 0.3, 0.3, 12.0, 0.8)
        write_text(tb9, ["Актуальность исследования"],
                   font_size=36, bold=True, color=COLOR_NAVY, align="left")
    # Основной текст — развёрнутый, с жирными ключевыми словами.
    tb14 = find_shape(slide, "TextBox 14")
    if tb14:
        set_shape_rect(tb14, 0.3, 1.3, 12.6, 1.5)
        write_text(
            tb14,
            [
                "На промышленных предприятиях на техническое обслуживание и "
                "ремонт оборудования уходит **от 15 до 40 процентов** операционного "
                "бюджета. Бумажный документооборот замедляет обработку заявок в "
                "несколько раз и приводит к потере **до 20 процентов обращений**. "
                "Готовые CMMS-системы либо стоят **миллионы рублей**, либо хранят "
                "данные **за рубежом**. Для заводов среднего масштаба (200–800 "
                "единиц оборудования) доступной отечественной альтернативы нет.",
            ],
            font_size=16, color=COLOR_TEXT, align="left",
        )
    # 4 преимущества.
    rect_texts = [
        ("Прямоугольник: скругленные углы 9",
         "Сокращение времени обработки заявки **с часов до минут**"),
        ("Прямоугольник: скругленные углы 12",
         "**Централизованный учёт** 200–800 единиц оборудования"),
        ("Прямоугольник: скругленные углы 13",
         "**Автоматическое планирование** регламентного ТО"),
        ("Прямоугольник: скругленные углы 14",
         "**Импортонезависимость** — данные на своих серверах"),
    ]
    for name, text in rect_texts:
        sh = find_shape(slide, name)
        if sh:
            write_text(sh, [text], font_size=13, color=COLOR_TEXT, align="left")

    add_notes(
        slide,
        "Начну с актуальности. Каждый промышленный завод тратит от пятнадцати до "
        "сорока процентов операционного бюджета на поддержание оборудования в "
        "рабочем состоянии. При бумажном документообороте до двадцати процентов "
        "заявок теряется, время реакции достигает восьми часов. Крупные CMMS, "
        "такие как SAP или IBM Maximo, стоят до пятнадцати миллионов рублей. "
        "Облачные сервисы хранят данные за рубежом. Для заводов среднего масштаба "
        "нужна доступная альтернатива.",
    )


def fill_slide_3(slide) -> None:
    """Объект, предмет, цель, задачи — с дословными формулировками задания."""
    # Верхний заголовок.
    tb58 = find_shape(slide, "TextBox 58")
    if tb58:
        set_shape_rect(tb58, 0.3, 0.3, 12.0, 0.8)
        write_text(tb58, ["Объект, предмет, цель и задачи"],
                   font_size=36, bold=True, color=COLOR_NAVY, align="left")

    # Убираем лишние стрелки и картинки иконок из шаблона (они мешают компоновке).
    remove_shapes(slide, {
        "Стрелка: вниз 22", "Стрелка: вниз 25", "Стрелка: вниз 26",
        "Рисунок 51", "Рисунок 52", "Рисунок 53",
    })

    tb42 = find_shape(slide, "TextBox 42")
    if tb42:
        set_shape_rect(tb42, 0.3, 1.3, 6.0, 0.9)
        write_text(
            tb42,
            [
                "**Объект исследования:**",
                "процессы технического обслуживания и ремонта "
                "оборудования предприятия",
            ],
            font_size=14, color=COLOR_TEXT, align="left",
        )
    tb43 = find_shape(slide, "TextBox 43")
    if tb43:
        set_shape_rect(tb43, 6.5, 1.3, 6.4, 0.9)
        write_text(
            tb43,
            [
                "**Предмет исследования:**",
                "веб-приложение для управления процессами ТОиР",
            ],
            font_size=14, color=COLOR_TEXT, align="left",
        )
    tb44 = find_shape(slide, "TextBox 44")
    if tb44:
        set_shape_rect(tb44, 0.3, 2.3, 12.6, 1.9)
        write_text(
            tb44,
            [
                "**Цель работы:**",
                "повышение эффективности процессов технического обслуживания и "
                "ремонта оборудования предприятия за счёт разработки и внедрения "
                "веб-приложения, обеспечивающего **централизованный учёт** "
                "оборудования и его технического состояния, **планирование и "
                "контроль** выполнения регламентных и внеплановых работ, "
                "**автоматизацию формирования заявок** на ремонт, а также повышение "
                "прозрачности и управляемости взаимодействия между техническими "
                "службами и эксплуатирующими подразделениями.",
            ],
            font_size=14, color=COLOR_TEXT, align="left",
        )
    rect50 = find_shape(slide, "Прямоугольник 50")
    if rect50:
        set_shape_rect(rect50, 0.3, 4.3, 12.6, 3.0)
        write_text(
            rect50,
            [
                "**Задачи работы:**",
                "1. Провести анализ предметной области ТОиР и выявить ключевые "
                "проблемы учёта, планирования и координации ремонтных работ.",
                "2. Исследовать существующие цифровые решения и подходы (CMMS/EAM), "
                "оценить их достоинства и ограничения.",
                "3. Сформировать функциональные и нефункциональные требования.",
                "4. Разработать архитектуру программного решения и структуру "
                "основных компонентов.",
                "5. Спроектировать базу данных для учёта оборудования, "
                "регламентных работ, заявок и истории обслуживания.",
                "6. Реализовать веб-приложение для учёта, планирования, обработки "
                "заявок и формирования отчётности.",
                "7. Провести тестирование разработанного решения и оценить его "
                "практическую применимость.",
            ],
            font_size=12, color=COLOR_TEXT, align="left",
        )
    add_notes(
        slide,
        "Объект исследования — процессы технического обслуживания и ремонта "
        "оборудования предприятия. Предмет — веб-приложение для управления этими "
        "процессами. Цель работы: повышение эффективности ТОиР через "
        "централизованный учёт оборудования, планирование регламентных и "
        "внеплановых работ, автоматизацию формирования заявок и прозрачное "
        "взаимодействие между службами. Для её достижения сформулировано семь "
        "задач — от анализа предметной области до тестирования готового решения.",
    )


def fill_content_slide(slide, *, title: str, subtitle: str | None, bullets: list[str],
                       context: str, image_path: pathlib.Path | None, notes: str) -> None:
    """Базовый шаблон информационного слайда.

    - Убираем декорации шаблона (диаграммы, лишние картинки).
    - Заголовок 36pt слева сверху, подзаголовок 20pt, далее левая колонка — диаграмма,
      правая — текст 16pt с жирными ключевыми словами.
    """
    # Нижний ярлык шаблона «Результаты исследования» — прячем.
    tb26 = find_shape(slide, "TextBox 26")
    if tb26:
        set_shape_rect(tb26, 0.3, 0.3, 12.6, 0.85)
        write_text(tb26, [title], font_size=36, bold=True,
                   color=COLOR_NAVY, align="left")

    # TextBox 15 — подзаголовок.
    tb15 = find_shape(slide, "TextBox 15")
    if tb15:
        set_shape_rect(tb15, 0.3, 1.2, 12.6, 0.55)
        write_text(tb15, [subtitle or ""], font_size=20, bold=True,
                   color=COLOR_TEXT, align="left")

    # TextBox 16 — основной текст (маркеры + дополнительный контекст).
    tb16 = find_shape(slide, "TextBox 16")
    if tb16:
        set_shape_rect(tb16, 8.3, 1.95, 4.9, 4.2)
        write_text(tb16, bullets, font_size=16, color=COLOR_TEXT, align="left")

    tb17 = find_shape(slide, "TextBox 17")
    if tb17:
        set_shape_rect(tb17, 8.3, 6.25, 4.9, 1.05)
        write_text(tb17, [context], font_size=14, color=COLOR_GRAY, align="left")

    # Удаляем декоративные чарты шаблона.
    remove_shapes(slide, {"Диаграмма 37", "Диаграмма 18", "Рисунок 19"})

    # Вставляем диаграмму слева.
    if image_path and image_path.exists():
        insert_fitted_image(slide, image_path, box=(0.3, 1.95, 7.85, 5.35))

    add_notes(slide, notes)


def fill_slide_last(slide) -> None:
    h1 = find_shape(slide, "Заголовок 1")
    if h1:
        write_text(h1, ["Спасибо за внимание"],
                   font_size=44, bold=True, color=COLOR_NAVY, align="center")
    tb2 = find_shape(slide, "TextBox 2")
    if tb2:
        write_text(tb2, [VKR_TITLE], font_size=18, bold=False,
                   color=COLOR_TEXT, align="center")
    tb3 = find_shape(slide, "TextBox 3")
    if tb3:
        write_text(
            tb3,
            [
                "**Подготовил:**",
                f"студент группы {STUDENT_GROUP}",
                STUDENT_NAME,
                "",
                "**Руководитель:**",
                SUPERVISOR_TITLE,
                SUPERVISOR,
            ],
            font_size=14, color=COLOR_TEXT, align="left",
        )
    add_notes(slide, "Спасибо за внимание! Готов ответить на ваши вопросы.")


# ---------- Контент информационных слайдов ----------

CONTENT_SLIDES = [
    {
        "title": "Обзор CMMS-систем",
        "subtitle": "Пустая ниша для среднего бизнеса",
        "bullets": [
            "**1С:ТОИР** — от 300 тыс. ₽, требует штатного 1С-программиста и сервера приложений.",
            "**SAP PM** — 10–15 млн ₽ только за лицензию, внедрение от полугода.",
            "**IBM Maximo** — 30 тыс. $/год, интеграции под крупные холдинги.",
            "**UpKeep (SaaS)** — 45 $/мес за пользователя, данные хранятся за рубежом.",
            "Для завода на **200–800 станков** и **5–15 ремонтников** доступного отечественного on-premise-решения на рынке нет.",
        ],
        "context": "Вывод: собственная разработка закрывает нишу, где крупные CMMS избыточны, а облачные сервисы неприменимы.",
        "image": "pres_cmms_positioning.png",
        "notes": "В первой главе я сравнил четыре ведущие CMMS-системы. Один Эс "
                 "ТОиР стоит от трёхсот тысяч рублей. СAP и IBM Maximo ориентированы "
                 "на крупные холдинги и стоят миллионы. UpKeep — доступный SaaS, "
                 "но хранит данные за рубежом. Для заводов среднего масштаба, на "
                 "двести-восемьсот станков, доступного отечественного on-premise "
                 "решения нет. Это обосновало необходимость собственной разработки.",
    },
    {
        "title": "Технологический стек",
        "subtitle": "Пять слоёв: от SPA до контейнеризации",
        "bullets": [
            "**Go + Echo** — один бинарник 18 МБ, горутины держат 500+ соединений при медиане 47 мс.",
            "**Vue 3 + Pinia** (Composition API) — ядро 33 КБ, итоговый бандл 142 КБ gzip.",
            "**PostgreSQL 16** — ACID-транзакции, JSONB для расширяемых полей оборудования.",
            "**Nginx** — reverse proxy: раздача SPA и проксирование /api на бэкенд.",
            "**Docker Compose** — multi-stage сборка, развёртывание одной командой.",
        ],
        "context": "Отвергнуты: Python (GIL тормозит конкурентность), Node.js (single-thread под нагрузкой), React (на 40 % тяжелее Vue).",
        "image": "tech_stack.png",
        "notes": "Стек выбирался под два сценария: десятки операторов одновременно "
                 "создают заявки и бэкенд держит стабильное время ответа. Go даёт "
                 "конкурентность через горутины и компилируется в один бинарник. "
                 "Vue 3 — лёгкий фреймворк с реактивностью через прокси. "
                 "PostgreSQL выбран из-за ACID-гарантий и JSONB-полей. "
                 "Весь стек контейнеризован и разворачивается одной командой.",
    },
    {
        "title": "Роли пользователей и требования",
        "subtitle": "4 роли, 8 ключевых сценариев, RBAC на 16 ключах",
        "bullets": [
            "**Оператор** — создаёт заявки на ремонт, отслеживает их статус.",
            "**Техник** — принимает заявки в работу, фиксирует результаты.",
            "**Инженер** — планирует регламентное ТО, смотрит отчётность и дашборд.",
            "**Администратор** — управляет пользователями, ролями, справочниками.",
            "Всего: **15 функциональных** и **12 нефункциональных** требований.",
        ],
        "context": "Авторизация — JWT с refresh-ротацией. Доступ — модель RBAC на 16 ключах, подсистема привязана к ролям.",
        "image": "pres_usecase.png",
        "notes": "В системе выделено четыре роли. Оператор создаёт заявки. "
                 "Техник их выполняет. Инженер планирует регламентное ТО и строит "
                 "отчётность. Администратор управляет пользователями и "
                 "справочниками. На основе ролей и бизнес-процессов "
                 "сформулировано пятнадцать функциональных и двенадцать "
                 "нефункциональных требований. Контроль доступа — RBAC на "
                 "шестнадцати ключах.",
    },
    {
        "title": "Пользовательские истории",
        "subtitle": "User Story Map: 5 этапов × 3 уровня приоритета",
        "bullets": [
            "**Backbone** (этапы пути): аутентификация → оборудование → заявки → планирование ТО → аналитика.",
            "**MVP** — минимальный функционал каждого этапа (5 историй).",
            "**Расширение** — второй приоритет (назначение, автонаряды, отчёты).",
            "**Развитие** — уведомления, напоминания, экспорт, аудит.",
            "Карта связывает требования, этапы разработки и сценарии тестирования.",
        ],
        "context": "Каждая история прописана в формате «Как %роль%, я хочу %действие%, чтобы %результат%» и покрыта тестами.",
        "image": "pres_user_stories.png",
        "notes": "Чтобы перейти от требований к плану разработки, я построил User "
                 "Story Map — карту пользовательских сценариев. Она охватывает весь "
                 "путь пользователя: от аутентификации до аналитики. В карте пять "
                 "этапов и три уровня приоритета — MVP, расширение, развитие. "
                 "Получилось пятнадцать связанных между собой историй.",
    },
    {
        "title": "Архитектура приложения",
        "subtitle": "Clean Architecture на Go, Vue 3 SPA через Nginx",
        "bullets": [
            "**Слои бэкенда:** handler → service → repository с инверсией зависимостей.",
            "**62 Go-файла** в 14 пакетах обслуживают **32 REST-эндпоинта**.",
            "**98 Vue-компонентов**, Pinia-сторы, ленивая загрузка админ-модулей.",
            "**Nginx** работает обратным прокси: SPA single-origin, проксирование /api.",
            "Инфраструктура устойчивости: **rate limit 120 req/min**, graceful shutdown, структурированные логи (zap).",
        ],
        "context": "Все 32 эндпоинта задокументированы в Swagger; из того же OpenAPI-спека можно генерировать типобезопасных клиентов.",
        "image": "pres_architecture.png",
        "notes": "Серверная часть построена по Clean Architecture. Шестьдесят два "
                 "Go-файла в четырнадцати пакетах обслуживают тридцать два "
                 "REST-эндпоинта. Клиентская часть — девяносто восемь "
                 "Vue-компонентов. Между ними Nginx: раздаёт SPA и проксирует "
                 "API-запросы. Настроены rate limiting, graceful shutdown, "
                 "структурированные логи через zap.",
    },
    {
        "title": "Модель данных",
        "subtitle": "6 центральных сущностей, 8 всего, 12 таблиц",
        "bullets": [
            "**equipment** — паспорт станка, JSONB-поле для расширяемых характеристик.",
            "**work_order** — заявка на ремонт с FK на оборудование и исполнителя.",
            "**maintenance_history** — журнал фактических работ и их длительности.",
            "**maintenance_plan** — регламент ТО: интервал, следующая дата, активность.",
            "**user, role** — авторизация, 4 роли и 16 разрешений.",
        ],
        "context": "Foreign keys с каскадным поведением, индексы на частые запросы (по статусу, исполнителю, сроку).",
        "image": "pres_er.png",
        "notes": "База данных содержит восемь основных сущностей и двенадцать "
                 "таблиц. Центральная связка — оборудование, заявки и история "
                 "обслуживания. Отдельно смоделирован RBAC: пользователь, роль, "
                 "разрешение. У таблицы оборудования есть JSONB-поле для "
                 "расширяемых характеристик, что избавляет от изменения схемы при "
                 "появлении нового типа станка.",
    },
    {
        "title": "Интерфейс: работа с заявками",
        "subtitle": "Форма создания, список с фильтрами, 6 состояний наряда",
        "bullets": [
            "**Форма заявки** — выбор оборудования из справочника, тип проблемы, срочность, фото.",
            "**Список заявок** — фильтры по статусу, исполнителю, типу оборудования; подсветка просроченных.",
            "**Состояния:** новая → принята → в работе → на проверке → закрыта → архив.",
            "**Уведомления** исполнителю при назначении и за сутки до дедлайна.",
            "Kanban-режим для диспетчера (опциональный).",
        ],
        "context": "Валидация на клиенте и сервере. Срок выполнения рассчитывается из приоритета и регламента предприятия.",
        "image": "3.2_forma_sozdaniya_zayavki.png",
        "notes": "Ключевой сценарий системы — обработка заявок. Оператор заполняет "
                 "форму: выбирает оборудование, указывает тип проблемы, срочность, "
                 "прикрепляет фото. Заявка проходит шесть состояний — от новой до "
                 "закрытой. Список фильтруется по статусу, исполнителю, "
                 "оборудованию. Просроченные заявки подсвечиваются, уведомления "
                 "приходят ответственному технику.",
    },
    {
        "title": "Интерфейс: оборудование и ТО",
        "subtitle": "Карточки, списки, автоматические графики ППР",
        "bullets": [
            "**Карточка оборудования** — паспорт, журнал работ, расписание ТО, вложения.",
            "**Список** — сортировка по цеху, состоянию, наработке.",
            "**Графики ТО** — расчёт дат по циклу и наработке; автоматические наряды.",
            "**История обслуживания** доступна одним кликом из карточки.",
            "Регламент настраивается под конкретную модель станка.",
        ],
        "context": "Система напоминает о предстоящем обслуживании и формирует наряды по расписанию без участия инженера.",
        "image": "3.6_kartochka_oborudovaniya.png",
        "notes": "Для управления парком предусмотрены карточки оборудования: "
                 "паспорт, журнал работ, расписание ТО, прикреплённые документы. "
                 "Список сортируется по цеху и состоянию. Графики ТО рассчитываются "
                 "автоматически. Система сама формирует наряды, когда подходит "
                 "срок обслуживания.",
    },
    {
        "title": "Дашборд и REST API",
        "subtitle": "KPI в реальном времени, 32 эндпоинта в Swagger",
        "bullets": [
            "**Дашборд** — активные заявки, загрузка техников, процент выполнения ППР.",
            "**Период и фильтры** настраиваемы, данные обновляются в реальном времени.",
            "**32 REST-эндпоинта** задокументированы в Swagger UI (OpenAPI 3).",
            "**JWT** хранится в httpOnly-cookie, активна CSRF-защита и строгие CSP-заголовки.",
            "API доступен сторонним системам (1С, IoT-шлюзы) через отдельный API-ключ.",
        ],
        "context": "Swagger UI доступен только администратору и инженеру. Из OpenAPI-спека генерируется типобезопасный TS-клиент.",
        "image": "3.9_swagger_ui.png",
        "notes": "Дашборд инженера показывает три ключевые метрики: активные "
                 "заявки, загрузку техников, процент выполнения ППР. Данные "
                 "обновляются в реальном времени. Все тридцать два REST-эндпоинта "
                 "задокументированы в Swagger, из того же OpenAPI-спека можно "
                 "генерировать клиентов. Токены JWT в httpOnly-cookie, активна "
                 "CSRF-защита и строгие CSP-заголовки.",
    },
    {
        "title": "Тестирование и нагрузка",
        "subtitle": "81 тест, покрытие 68 %, медиана 47 мс при 500 подключениях",
        "bullets": [
            "**28 модульных** тестов (Go) — доменная логика и сервисы.",
            "**45 интеграционных** (testcontainers + PostgreSQL) — репозитории и API.",
            "**8 сквозных** (Playwright) — ключевые пользовательские сценарии.",
            "**Покрытие критичных модулей 76–84 %** (аутентификация, обработка заявок).",
            "**Нагрузка:** 500 одновременных подключений, p95 = 180 мс, throughput 2.4k req/s.",
        ],
        "context": "Все 81 тест проходят в CI. Docker-образы: Go API 18 МБ, Nginx+Vue 25 МБ. Развёртывание — одна команда.",
        "image": "test_metrics.png",
        "notes": "Тестирование проведено на трёх уровнях. Двадцать восемь модульных "
                 "тестов, сорок пять интеграционных и восемь сквозных. Все "
                 "восемьдесят один тест пройдены. Общее покрытие — шестьдесят "
                 "восемь процентов, для критичных модулей — до восьмидесяти "
                 "четырёх. В нагрузочном тестировании через k6 при пятистах "
                 "одновременных подключениях медиана составила сорок семь "
                 "миллисекунд, девяносто пятый процентиль — сто восемьдесят. "
                 "Стек собирается в Docker и разворачивается одной командой.",
    },
]


def main() -> None:
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy(TEMPLATE, WORK)

    prs = Presentation(str(WORK))
    remove_slide(prs, 3)  # «Оглавление».
    prs.save(str(WORK))

    prs = Presentation(str(WORK))
    for _ in range(7):
        duplicate_slide(prs, 3)
    prs.save(str(WORK))

    prs = Presentation(str(WORK))
    reorder_slides(prs, [0, 1, 2, 3, 4, 5, 7, 8, 9, 10, 11, 12, 13, 6])
    prs.save(str(WORK))

    prs = Presentation(str(WORK))
    slides = list(prs.slides)
    assert len(slides) == 14

    fill_slide_1(slides[0])
    fill_slide_2(slides[1])
    fill_slide_3(slides[2])
    for i, content in enumerate(CONTENT_SLIDES):
        slide = slides[3 + i]
        img_path = IMAGES / content["image"]
        fill_content_slide(
            slide,
            title=content["title"],
            subtitle=content["subtitle"],
            bullets=content["bullets"],
            context=content["context"],
            image_path=img_path if img_path.exists() else None,
            notes=content["notes"],
        )
    fill_slide_last(slides[13])

    prs.save(str(OUTPUT))
    WORK.unlink()
    print(f"OK -> {OUTPUT}  ({OUTPUT.stat().st_size // 1024} KB, {len(slides)} slides)")


if __name__ == "__main__":
    main()
