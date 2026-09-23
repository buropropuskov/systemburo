"""Генерация презентации Дениса (pptx, 15 слайдов, 16:9).

Стиль: Times New Roman, заголовок 40pt, подзаголовки 18pt, тело 16pt.
Золотистые карточки F1D669, рамки NAVY 1F3A5F.
Использует готовые PNG из Denis/images/.
"""
from pathlib import Path

from pptx import Presentation
from pptx.util import Cm, Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR

IMG = Path("/home/washka/project/diplom/Denis/images")
OUT = Path("/home/washka/project/diplom/Denis/presentation/presentation_denis.pptx")

GOLD = RGBColor(0xF1, 0xD6, 0x69)
GOLD_LIGHT = RGBColor(0xFA, 0xE9, 0xA8)
GOLD_DARK = RGBColor(0xD4, 0xB3, 0x40)
NAVY = RGBColor(0x1F, 0x3A, 0x5F)
BLACK = RGBColor(0x00, 0x00, 0x00)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
GRAY = RGBColor(0x70, 0x70, 0x70)
CREAM = RGBColor(0xFD, 0xF6, 0xDD)
RED = RGBColor(0xC0, 0x39, 0x2B)
GREEN = RGBColor(0x27, 0xAE, 0x60)

FONT = "Times New Roman"

SLIDE_W = Cm(33.87)
SLIDE_H = Cm(19.05)

# ---------- Helpers ----------

def set_text(shape, text, *, size=16, bold=False, color=BLACK,
             align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP, italic=False):
    tf = shape.text_frame
    tf.clear()
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    for i, line in enumerate(str(text).split("\n")):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = align
        r = p.add_run()
        r.text = line
        r.font.name = FONT
        r.font.size = Pt(size)
        r.font.bold = bold
        r.font.italic = italic
        r.font.color.rgb = color


def add_textbox(slide, x, y, w, h, text, **kwargs):
    tb = slide.shapes.add_textbox(Cm(x), Cm(y), Cm(w), Cm(h))
    set_text(tb, text, **kwargs)
    tb.text_frame.margin_left = Cm(0.1)
    tb.text_frame.margin_right = Cm(0.1)
    tb.text_frame.margin_top = Cm(0.05)
    tb.text_frame.margin_bottom = Cm(0.05)
    return tb


def add_card(slide, x, y, w, h, text, *, fill=GOLD, size=16, title=None,
             title_size=None, align=PP_ALIGN.CENTER, color=BLACK,
             border=None, bold_body=False):
    shp = slide.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE,
                                   Cm(x), Cm(y), Cm(w), Cm(h))
    shp.fill.solid()
    shp.fill.fore_color.rgb = fill
    if border is not None:
        shp.line.color.rgb = border
        shp.line.width = Pt(1.2)
    else:
        shp.line.fill.background()
    tf = shp.text_frame
    tf.clear()
    tf.word_wrap = True
    tf.vertical_anchor = MSO_ANCHOR.MIDDLE
    tf.margin_left = Cm(0.3)
    tf.margin_right = Cm(0.3)
    tf.margin_top = Cm(0.2)
    tf.margin_bottom = Cm(0.2)
    parts = []
    if title:
        parts.append((title, title_size or (size + 4), True))
    parts.append((text, size, bold_body))
    for i, (t, sz, b) in enumerate(parts):
        lines = t.split("\n")
        for j, line in enumerate(lines):
            if i == 0 and j == 0:
                p = tf.paragraphs[0]
            else:
                p = tf.add_paragraph()
            p.alignment = align
            r = p.add_run()
            r.text = line
            r.font.name = FONT
            r.font.size = Pt(sz)
            r.font.bold = b
            r.font.color.rgb = color
    return shp


def add_title(slide, text, *, y=0.9):
    tb = slide.shapes.add_textbox(Cm(0.8), Cm(y), Cm(32), Cm(1.97))
    set_text(tb, text, size=40, bold=True, color=NAVY, anchor=MSO_ANCHOR.MIDDLE)
    return tb


def add_page_number(slide, n):
    tb = slide.shapes.add_textbox(Cm(32.2), Cm(18.2), Cm(1.2), Cm(0.5))
    set_text(tb, str(n), size=11, align=PP_ALIGN.RIGHT, color=GRAY)


def add_table(slide, x, y, w, h, headers, rows, *,
              header_fill=NAVY, header_color=WHITE,
              alt_fill=GOLD_LIGHT, body_size=13, header_size=13):
    n_rows = len(rows) + 1
    n_cols = len(headers)
    tbl = slide.shapes.add_table(n_rows, n_cols, Cm(x), Cm(y), Cm(w), Cm(h)).table
    for i, h_text in enumerate(headers):
        cell = tbl.cell(0, i)
        cell.fill.solid()
        cell.fill.fore_color.rgb = header_fill
        tf = cell.text_frame
        tf.clear()
        p = tf.paragraphs[0]
        p.alignment = PP_ALIGN.CENTER
        r = p.add_run()
        r.text = h_text
        r.font.name = FONT
        r.font.size = Pt(header_size)
        r.font.bold = True
        r.font.color.rgb = header_color
        cell.vertical_anchor = MSO_ANCHOR.MIDDLE
        cell.margin_left = Cm(0.12)
        cell.margin_right = Cm(0.12)
        cell.margin_top = Cm(0.05)
        cell.margin_bottom = Cm(0.05)
    for ri, row in enumerate(rows):
        is_alt = ri % 2 == 1
        for ci, val in enumerate(row):
            cell = tbl.cell(ri + 1, ci)
            cell.fill.solid()
            cell.fill.fore_color.rgb = alt_fill if is_alt else WHITE
            tf = cell.text_frame
            tf.clear()
            for li, line in enumerate(str(val).split("\n")):
                p = tf.paragraphs[0] if li == 0 else tf.add_paragraph()
                p.alignment = PP_ALIGN.LEFT
                r = p.add_run()
                r.text = line
                r.font.name = FONT
                r.font.size = Pt(body_size)
                r.font.color.rgb = BLACK
            cell.vertical_anchor = MSO_ANCHOR.MIDDLE
            cell.margin_left = Cm(0.12)
            cell.margin_right = Cm(0.12)
            cell.margin_top = Cm(0.05)
            cell.margin_bottom = Cm(0.05)
    return tbl


def add_image(slide, path, x, y, *, w=None, h=None):
    kwargs = {}
    if w is not None:
        kwargs["width"] = Cm(w)
    if h is not None:
        kwargs["height"] = Cm(h)
    return slide.shapes.add_picture(str(path), Cm(x), Cm(y), **kwargs)


def add_slide(prs, *, blank=True):
    layout = prs.slide_layouts[6] if blank else prs.slide_layouts[0]
    slide = prs.slides.add_slide(layout)
    return slide


def add_background_bar(slide, color=NAVY, height_cm=0.4):
    """Узкая полоса сверху — фирменная мелочь."""
    shp = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE,
                                   Cm(0), Cm(0), SLIDE_W, Cm(height_cm))
    shp.fill.solid()
    shp.fill.fore_color.rgb = color
    shp.line.fill.background()


# ---------- Slides ----------

def slide_01_title(prs):
    slide = add_slide(prs)
    add_background_bar(slide, NAVY, 0.5)

    add_textbox(slide, 0.8, 1.5, 32.0, 3.0,
                "МИНОБРНАУКИ РОССИИ\n"
                "Федеральное государственное бюджетное образовательное учреждение\n"
                "высшего образования «МИРЭА — Российский технологический университет»",
                size=16, bold=True, color=NAVY, align=PP_ALIGN.CENTER)

    add_textbox(slide, 0.8, 4.5, 32.0, 0.8,
                "Институт перспективных технологий и индустриального программирования",
                size=14, color=BLACK, align=PP_ALIGN.CENTER)
    add_textbox(slide, 0.8, 5.3, 32.0, 0.8,
                "Кафедра индустриального программирования",
                size=14, color=BLACK, align=PP_ALIGN.CENTER)

    # Центральная плашка с темой
    add_card(slide, 2.0, 7.5, 30.0, 4.2,
             "",
             title="Разработка системы оценивания степени самостоятельности решения задач",
             title_size=28, size=16, fill=GOLD, align=PP_ALIGN.CENTER, color=NAVY)
    add_textbox(slide, 2.0, 11.8, 30.0, 0.7,
                "Выпускная квалификационная работа бакалавра",
                size=16, italic=True, color=GRAY, align=PP_ALIGN.CENTER)

    # Подвал
    add_textbox(slide, 2.0, 14.2, 14.0, 4.0,
                "Выполнил:\nстудент группы ЭФБО-06-22\nМихайлов Денис Артемович",
                size=16, color=BLACK, align=PP_ALIGN.LEFT)
    add_textbox(slide, 17.5, 14.2, 14.0, 4.0,
                "Научный руководитель:\nк.т.н., доцент кафедры ИП\nЛукьянов П. В.",
                size=16, color=BLACK, align=PP_ALIGN.LEFT)

    add_textbox(slide, 0.8, 18.3, 32.0, 0.5,
                "Москва · 2026",
                size=14, italic=True, color=GRAY, align=PP_ALIGN.CENTER)


def slide_02_relevance(prs):
    slide = add_slide(prs)
    add_title(slide, "Актуальность исследования")

    add_textbox(slide, 0.8, 3.3, 32.0, 2.2,
                "Moodle — фактический стандарт дистанционного обучения: 200 000+ площадок "
                "в 240 странах. В единичной группе 25–30 студентов ручная попарная проверка "
                "самостоятельности обходится преподавателю в 7,5 часов на одно задание.",
                size=18, color=BLACK)

    # Три карточки — болевые точки
    add_card(slide, 0.8, 6.0, 10.3, 4.5,
             "C(25,2) = 300 пар\nпри группе 25 студентов\nи 1,5 мин на каждую",
             title="7,5 часов ручной проверки",
             title_size=20, size=15, fill=GOLD)
    add_card(slide, 11.7, 6.0, 10.3, 4.5,
             "Преподаватель сравнивает выборочно,\n"
             "решение не документируется\n"
             "и не воспроизводится",
             title="Субъективность",
             title_size=20, size=15, fill=GOLD)
    add_card(slide, 22.6, 6.0, 10.3, 4.5,
             "На рынке нет универсального\n"
             "плагина Moodle, работающего\n"
             "с текстом + кодом + таблицами",
             title="Пробел на рынке",
             title_size=20, size=15, fill=GOLD)

    # Нижняя плашка — вывод
    add_card(slide, 0.8, 11.5, 32.2, 5.5,
             "Актуальность темы обусловлена двумя факторами. Первый — реальный дефицит времени "
             "преподавателя в дистанционном обучении. Второй — отсутствие отечественного "
             "инструмента, интегрированного в Moodle, способного одновременно обрабатывать "
             "тексты (DOCX, PDF), электронные таблицы (XLSX) и исходный код (Python, Java, "
             "C++, PHP).",
             title="Что это означает",
             title_size=20, size=16, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    add_page_number(slide, 2)


def slide_03_goal_tasks(prs):
    slide = add_slide(prs)
    add_title(slide, "Объект, предмет, цель и задачи")

    add_card(slide, 0.8, 3.3, 15.8, 3.4,
             "Процесс оценивания степени самостоятельности решения задач студентами "
             "в системе дистанционного обучения Moodle",
             title="Объект исследования",
             title_size=18, size=15, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)
    add_card(slide, 17.0, 3.3, 15.8, 3.4,
             "Плагин попарного сравнения студенческих работ в рамках одного задания Moodle, "
             "реализующий автоматический анализ заимствований для текстовых документов и кода",
             title="Предмет исследования",
             title_size=18, size=15, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    add_card(slide, 0.8, 7.0, 32.0, 2.6,
             "Повышение эффективности процесса оценивания степени самостоятельности решения "
             "задач студентами за счёт снижения трудозатрат преподавателя на проверку заданий "
             "текущего контроля.",
             title="Цель работы",
             title_size=20, size=16, fill=GOLD, align=PP_ALIGN.LEFT, color=NAVY)

    # Четыре задачи — 4 карточки в ряд
    tasks = [
        ("1", "Анализ инструментов",
         "Анализ существующих инструментов проверки уникальности, "
         "выявление их недостатков при применении в LMS, формулировка требований"),
        ("2", "Моделирование",
         "Моделирование предметной области системы оценивания студентов"),
        ("3", "Проектирование",
         "Проектирование модуля проверки работ на заимствования "
         "и его интеграция в систему Moodle"),
        ("4", "Реализация и оценка",
         "Реализация, тестирование разработанного плагина "
         "и оценка эффективности снижения трудозатрат"),
    ]
    for i, (num, title, desc) in enumerate(tasks):
        x = 0.8 + i * 8.05
        # Номер в кружке
        circ = slide.shapes.add_shape(MSO_SHAPE.OVAL,
                                        Cm(x), Cm(10.1), Cm(1.6), Cm(1.6))
        circ.fill.solid()
        circ.fill.fore_color.rgb = NAVY
        circ.line.fill.background()
        set_text(circ, num, size=24, bold=True, color=WHITE,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

        add_card(slide, x, 12.0, 7.7, 5.8, desc,
                 title=title, title_size=16, size=13, fill=GOLD_LIGHT,
                 align=PP_ALIGN.LEFT)

    add_page_number(slide, 3)


def slide_04_market(prs):
    slide = add_slide(prs)
    add_title(slide, "Анализ рынка инструментов проверки уникальности")

    # Таблица сравнения
    add_textbox(slide, 0.8, 3.2, 32.0, 0.8,
                "Сравнительный анализ четырёх ключевых инструментов по 6 критериям",
                size=16, color=GRAY, align=PP_ALIGN.LEFT)

    headers = ["Критерий", "Turnitin", "Антиплагиат", "MOSS", "JPlag"]
    rows = [
        ("Тип проверки", "С интернет-базой", "С базой источников", "Попарное", "Попарное"),
        ("Поддержка текста", "DOCX/PDF/TXT", "DOCX/PDF/TXT/ODT", "нет", "нет"),
        ("Поддержка кода", "ограниченная", "ограниченная", "C, Java, Python", "Java, C++, Python"),
        ("Интеграция Moodle", "Платный плагин", "Платный плагин", "нет", "нет"),
        ("Попарное сравнение", "ограниченно", "ограниченно", "основная функция", "основная функция"),
        ("Стоимость", "от $3/студент", "платная", "бесплатно", "open-source"),
    ]
    add_table(slide, 0.8, 4.3, 24.0, 10.5, headers, rows, body_size=14, header_size=14)

    # Итоговый блок справа
    add_card(slide, 25.2, 4.3, 8.0, 10.5,
             "Универсального плагина\n"
             "Moodle, который работает\n"
             "одновременно с:\n\n"
             "• текстом\n"
             "• кодом\n"
             "• таблицами\n\n"
             "и делает это внутри LMS,\n"
             "на рынке нет.\n\n"
             "Именно эту нишу закрывает\n"
             "разрабатываемая система.",
             title="Ключевой вывод",
             title_size=16, size=13, fill=GOLD, align=PP_ALIGN.LEFT)

    add_page_number(slide, 4)


def slide_05_requirements(prs):
    slide = add_slide(prs)
    add_title(slide, "Требования к разрабатываемой системе")

    add_textbox(slide, 0.8, 3.2, 32.0, 0.8,
                "10 функциональных и 8 нефункциональных требований выведены из анализа аналогов",
                size=16, color=GRAY, align=PP_ALIGN.LEFT)

    # Функциональные
    add_textbox(slide, 0.8, 4.1, 15.8, 0.7,
                "Функциональные требования (10)",
                size=18, bold=True, color=NAVY)
    fr_headers = ["ID", "Требование"]
    fr_rows = [
        ("FR-01", "Установка как plagiarism-плагин Moodle"),
        ("FR-02", "Автозапуск после срока сдачи"),
        ("FR-03", "Поддержка DOCX, PDF, TXT, ODT"),
        ("FR-04", "Поддержка Python, Java, C++, PHP"),
        ("FR-05", "Поддержка XLSX"),
        ("FR-06", "Попарное сравнение всех работ"),
        ("FR-07", "Отчёт с процентом по каждой паре"),
        ("FR-08", "Подсветка совпадающих фрагментов"),
        ("FR-09", "Фоновая очередь задач"),
        ("FR-10", "Настройки порогов"),
    ]
    add_table(slide, 0.8, 4.9, 15.8, 12.0, fr_headers, fr_rows,
              body_size=13, header_size=13)

    # Нефункциональные
    add_textbox(slide, 17.0, 4.1, 15.8, 0.7,
                "Нефункциональные требования (8)",
                size=18, bold=True, color=NAVY)
    nfr_headers = ["ID", "Требование", "Метрика"]
    nfr_rows = [
        ("NFR-01", "Время проверки пары DOCX", "≤ 3 с"),
        ("NFR-02", "Обработка группы 30", "≤ 10 мин"),
        ("NFR-03", "Отказоустойчивость", "fail-safe"),
        ("NFR-04", "Совместимость Moodle", "4.0 LTS+"),
        ("NFR-05", "Стек", "PHP 8.1"),
        ("NFR-06", "Coding Style Moodle", "полное"),
        ("NFR-07", "Локализация", "ru / en"),
        ("NFR-08", "Покрытие тестами", "≥ 50 %"),
    ]
    add_table(slide, 17.0, 4.9, 15.8, 12.0, nfr_headers, nfr_rows,
              body_size=13, header_size=13)

    add_page_number(slide, 5)


def slide_06_actors_processes(prs):
    slide = add_slide(prs)
    add_title(slide, "Акторы и бизнес-процесс «как есть»")

    # Слева — три актора
    add_textbox(slide, 0.8, 3.3, 15.8, 0.8,
                "Три роли в процессе",
                size=20, bold=True, color=NAVY)

    add_card(slide, 0.8, 4.3, 15.8, 3.9,
             "Настраивает задание, получает отчёт,\n"
             "раскрывает подозрительные пары,\n"
             "принимает решение об оценке",
             title="Преподаватель",
             title_size=18, size=14, fill=GOLD, align=PP_ALIGN.LEFT)
    add_card(slide, 0.8, 8.5, 15.8, 3.9,
             "Загружает файл через стандартный\n"
             "элемент «Задание» Moodle — никаких\n"
             "дополнительных действий не требуется",
             title="Студент",
             title_size=18, size=14, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)
    add_card(slide, 0.8, 12.7, 15.8, 3.9,
             "Устанавливает плагин, настраивает\n"
             "пороги, просматривает агрегированную\n"
             "статистику по кафедре или факультету",
             title="Администратор Moodle",
             title_size=18, size=14, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    # Справа — BPMN as-is картинка
    add_textbox(slide, 17.0, 3.3, 15.8, 0.8,
                "Процесс «как есть» — ручная проверка",
                size=20, bold=True, color=NAVY)
    add_image(slide, IMG / "pic_2_1_bpmn_as_is.png", 17.0, 4.3, w=15.8)

    add_page_number(slide, 6)


def slide_07_to_be(prs):
    slide = add_slide(prs)
    add_title(slide, "Предлагаемый процесс «как будет»")

    add_image(slide, IMG / "pic_2_2_bpmn_to_be.png", 0.8, 3.3, w=32.2)

    add_card(slide, 0.8, 15.5, 32.2, 2.8,
             "После срока Moodle Cron триггерит плагин. Плагин создаёт задачи "
             "в очереди, извлекает текст, считает шинглы и Winnowing, сохраняет результат. "
             "Преподаватель получает уведомление и сразу видит цветовую матрицу пар.",
             title="Вместо 7,5 часов ручной работы — 15 минут фонового времени сервера",
             title_size=18, size=14, fill=GOLD, align=PP_ALIGN.LEFT)

    add_page_number(slide, 7)


def slide_08_usecase(prs):
    slide = add_slide(prs)
    add_title(slide, "Диаграмма вариантов использования")

    add_image(slide, IMG / "pic_2_4_use_case.png", 0.8, 3.3, w=22.0)

    add_card(slide, 23.4, 3.3, 9.8, 14.0,
             "4 актора:\n"
             "• Преподаватель\n"
             "• Студент\n"
             "• Администратор\n"
             "• Moodle Cron\n\n"
             "12 вариантов использования\n"
             "организованы в три колонки:\n\n"
             "• пользовательские сценарии\n"
             "• расширения и включения\n"
             "• фоновая обработка\n\n"
             "<<include>> — обязательные\n"
             "этапы обработки\n\n"
             "<<extend>> — опциональное\n"
             "поведение (например,\n"
             "экспорт в CSV)",
             title="Что на диаграмме",
             title_size=16, size=13, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    add_page_number(slide, 8)


def slide_09_architecture(prs):
    slide = add_slide(prs)
    add_title(slide, "Архитектура плагина")

    add_image(slide, IMG / "pic_3_1_architecture.png", 0.8, 3.3, w=22.0)

    add_card(slide, 23.4, 3.3, 9.8, 14.0,
             "Слоистая модель по Мартину:\n"
             "UI → обработка → анализ\n"
             "→ хранилище.\n\n"
             "Тип расширения — нативный\n"
             "plagiarism-плагин Moodle.\n\n"
             "Данные не покидают контур\n"
             "LMS — соответствие ФЗ-152\n"
             "и импортозамещение.\n\n"
             "Паттерны:\n"
             "• Strategy — выбор алгоритма\n"
             "• Observer — реакция на\n"
             "загрузку файлов\n\n"
             "Стек: PHP 8.1, Mustache,\n"
             "Bootstrap 5, AJAX",
             title="Почему так",
             title_size=16, size=13, fill=GOLD, align=PP_ALIGN.LEFT)

    add_page_number(slide, 9)


def slide_10_algorithms(prs):
    slide = add_slide(prs)
    add_title(slide, "Алгоритмы сравнения")

    # Слева — шинглы
    add_card(slide, 0.8, 3.3, 15.8, 14.0,
             "",
             title="Шинглы + коэффициент Жаккара",
             title_size=20, size=13, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)
    # Формула и описание поверх карточки
    add_textbox(slide, 1.2, 5.4, 15.0, 1.0,
                "J(A, B) = |A ∩ B| / |A ∪ B|",
                size=20, bold=True, color=NAVY, align=PP_ALIGN.CENTER)
    add_textbox(slide, 1.2, 6.7, 15.0, 10.0,
                "1. Нормализация текста: нижний регистр, удаление знаков препинания.\n"
                "2. Разбиение на k-шинглы (k=5–7 слов).\n"
                "3. Хеширование каждого шингла (CRC32).\n"
                "4. Вычисление J(A, B) ∈ [0, 1].\n\n"
                "Применяется к:\n"
                "• DOCX, PDF, TXT, ODT\n"
                "• XLSX (строковое представление)\n\n"
                "Работа Бродера [8]: k=5–7 — баланс между\n"
                "чувствительностью к переформулировкам\n"
                "и устойчивостью к случайным совпадениям.",
                size=14, color=BLACK, align=PP_ALIGN.LEFT)

    # Справа — Winnowing
    add_card(slide, 17.0, 3.3, 15.8, 14.0,
             "",
             title="Winnowing для исходного кода",
             title_size=20, size=13, fill=GOLD, align=PP_ALIGN.LEFT)
    add_textbox(slide, 17.4, 5.4, 15.0, 11.5,
                "Оригинальный алгоритм Schleimer et al. [9]\n"
                "(используется в MOSS Stanford).\n\n"
                "1. Токенизация: имена → __ID__, строки → __STR__\n"
                "   (устойчивость к переименованиям).\n"
                "2. Построение k-грамм (k=5).\n"
                "3. Rolling hash для каждой k-граммы.\n"
                "4. Скользящее окно w=4 — минимум в каждой позиции.\n"
                "5. Набор хешей = отпечаток документа.\n"
                "6. Сравнение отпечатков по Жаккару.\n\n"
                "Применяется к:\n"
                "• Python, Java, C++, PHP\n\n"
                "k=5, w=4 — классические значения из [9].",
                size=14, color=BLACK, align=PP_ALIGN.LEFT)

    add_page_number(slide, 10)


def slide_11_implementation(prs):
    slide = add_slide(prs)
    add_title(slide, "Реализация: структура кода и примеры")

    # Цифры сверху
    metrics = [("22", "PHP-файла"),
               ("4 362", "строки PHP"),
               ("5", "таблиц БД"),
               ("2", "языка локализации")]
    for i, (big, small) in enumerate(metrics):
        x = 0.8 + i * 8.1
        add_card(slide, x, 3.3, 7.7, 2.3, small,
                 title=big, title_size=30, size=14, fill=GOLD)

    # Две колонки: листинг + подпись
    add_textbox(slide, 0.8, 6.3, 15.8, 0.8,
                "Листинг 4.3 — Winnowing (classes/analyser.php)",
                size=16, bold=True, color=NAVY)
    add_card(slide, 0.8, 7.2, 15.8, 9.8,
             "public static function winnowing_fingerprints(\n"
             "        string $code,\n"
             "        int $k = 5, int $w = 4): array {\n"
             "    $tokens = self::tokenise_code($code);\n"
             "    $kgrams = self::kgrams($tokens, $k);\n"
             "    $hashes = array_map('crc32', $kgrams);\n"
             "    $fp = [];\n"
             "    for ($i = 0; $i + $w < count($hashes); $i++) {\n"
             "        $window = array_slice($hashes, $i, $w);\n"
             "        $fp[] = min($window);\n"
             "    }\n"
             "    return array_unique($fp);\n"
             "}",
             title="", size=11, fill=RGBColor(0xF6, 0xF6, 0xF0),
             border=GRAY, align=PP_ALIGN.LEFT)

    # Правая колонка: ключевые классы
    add_textbox(slide, 17.0, 6.3, 15.8, 0.8,
                "Ключевые классы плагина",
                size=16, bold=True, color=NAVY)
    add_table(slide, 17.0, 7.2, 15.8, 9.8,
              ["Файл", "Назначение"],
              [
                  ("lib.php", "Точка входа Plugin API, hook-и Moodle"),
                  ("classes/analyser.php", "Ядро: Жаккар + Winnowing, сравнение пар"),
                  ("classes/text_extractor.php", "Парсинг DOCX/PDF/XLSX/кода"),
                  ("classes/task/process_queue.php", "Фоновая scheduled-task"),
                  ("classes/file_types.php", "Определение типа файла"),
                  ("db/install.xml", "Схема 5 таблиц плагина"),
                  ("report.php, pair.php", "Страницы отчёта и пары"),
                  ("lang/ru, lang/en", "Локализация интерфейса"),
              ], body_size=12, header_size=13)

    add_page_number(slide, 11)


def slide_12_testing(prs):
    slide = add_slide(prs)
    add_title(slide, "Тестирование плагина")

    # Слева — результат PHPUnit
    add_textbox(slide, 0.8, 3.3, 15.8, 0.8,
                "29 модульных тестов, PHPUnit",
                size=18, bold=True, color=NAVY)
    add_image(slide, IMG / "pic_4_3_phpunit.png", 0.8, 4.3, w=15.8)

    add_card(slide, 0.8, 12.8, 15.8, 4.2,
             "• 12 тестов `analyser_test` — корректность метрики Жаккара и Winnowing\n"
             "• 10 тестов `text_extractor_test` — парсинг DOCX, PDF, кода\n"
             "• 7 тестов `queue_test` — жизненный цикл задач очереди\n"
             "• Coverage: строки 58,3 %, методы 73,1 %, классы 80 %",
             title="Покрытие",
             title_size=16, size=13, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    # Справа — интеграционные
    add_textbox(slide, 17.0, 3.3, 15.8, 0.8,
                "3 сценария интеграционного тестирования",
                size=18, bold=True, color=NAVY)
    add_table(slide, 17.0, 4.3, 15.8, 7.2,
              ["Сценарий", "Факт", "Статус"],
              [
                  ("Группа 5, 10 пар",
                   "1 high, 1 mid, 8 low",
                   "✓"),
                  ("Мусорный PDF",
                   "failed без падения",
                   "✓"),
                  ("Группа 30, 435 пар",
                   "18 мин 42 с, 387 МБ",
                   "✓"),
              ], body_size=14, header_size=14)
    add_card(slide, 17.0, 12.0, 15.8, 5.0,
             "Первая версия `extract_docx` падала на DOCX с OLE — исправлено через "
             "`libxml_use_internal_errors`. Большие архивы (>50 МБ) упирались в "
             "memory_limit=128M — поднято до 256 МБ. Шаблонные титульники давали "
             "ложные 25–30 % — тюнинг `shingle_size` до 7.",
             title="Найденные и устранённые дефекты",
             title_size=16, size=13, fill=GOLD, align=PP_ALIGN.LEFT)

    add_page_number(slide, 12)


def slide_13_efficiency(prs):
    slide = add_slide(prs)
    add_title(slide, "Оценка эффективности снижения трудозатрат")

    # Большая таблица сравнения
    headers = ["Размер группы", "Пар C(N, 2)", "Ручная проверка",
               "С плагином", "Экономия"]
    rows = [
        ("15", "105",   "2 ч 37 мин",  "7 мин",  "×22"),
        ("25", "300",   "7 ч 30 мин",  "15 мин", "×30"),
        ("30", "435",  "10 ч 53 мин",  "20 мин", "×32"),
        ("50", "1 225", "30 ч 37 мин", "45 мин", "×40"),
    ]
    add_table(slide, 0.8, 3.3, 17.0, 7.5, headers, rows,
              body_size=16, header_size=16)

    # Правая колонка: метрики точности
    add_textbox(slide, 18.2, 3.3, 14.8, 0.8,
                "Точность на синтетическом датасете",
                size=18, bold=True, color=NAVY)
    for i, (big, small, col) in enumerate([
        ("90%",  "recall (18/20)",  GOLD),
        ("95%",  "precision (18/19)", GOLD_LIGHT),
        ("0,925", "F1-score", GOLD_DARK),
    ]):
        y = 4.5 + i * 2.3
        add_card(slide, 18.2, y, 14.8, 2.0, small,
                 title=big, title_size=36, size=14,
                 fill=col, align=PP_ALIGN.CENTER)

    # Вывод снизу
    add_card(slide, 0.8, 11.3, 32.2, 5.7,
             "Типовой случай — группа 25 человек. Было: 7 часов 30 минут ручной работы. "
             "Стало: 15 минут фонового серверного времени. Экономия — в 30 раз. "
             "Качественный сдвиг: вместо выборочной проверки случайных работ "
             "преподаватель охватывает все задания курса.",
             title="Система готова к релизу",
             title_size=20, size=16, fill=GOLD, align=PP_ALIGN.LEFT, color=NAVY)

    add_page_number(slide, 13)


def slide_14_perspectives(prs):
    slide = add_slide(prs)
    add_title(slide, "Перспективы развития")

    perspectives = [
        ("1", "Семантические эмбеддинги",
         "SBERT вместо шинглов —\n"
         "ловит перефразирование,\n"
         "которое Жаккар пропускает\n"
         "по построению"),
        ("2", "OCR для изображений",
         "tesseract для PNG/JPG —\n"
         "распознавание рукописных\n"
         "решений по математике\n"
         "и физике"),
        ("3", "Кросс-проверка",
         "Интеграция с «Антиплагиат»\n"
         "для выпускных работ,\n"
         "требующих сравнения\n"
         "с внешней веб-базой"),
        ("4", "Админская аналитика",
         "Агрегированные метрики\n"
         "по курсам и факультетам:\n"
         "«курс с наиболее\n"
         "подозрительной активностью»"),
        ("5", "Moodle Workplace",
         "Адаптация под корпоративную\n"
         "версию Moodle для обучения\n"
         "сотрудников компаний —\n"
         "ролевая модель и learning paths"),
    ]
    for i, (num, title, desc) in enumerate(perspectives):
        x = 0.8 + i * 6.45
        circ = slide.shapes.add_shape(MSO_SHAPE.OVAL,
                                        Cm(x + 2.2), Cm(3.5), Cm(1.8), Cm(1.8))
        circ.fill.solid()
        circ.fill.fore_color.rgb = NAVY
        circ.line.fill.background()
        set_text(circ, num, size=28, bold=True, color=WHITE,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

        add_card(slide, x, 5.8, 6.2, 11.2, desc,
                 title=title, title_size=16, size=13, fill=GOLD_LIGHT,
                 align=PP_ALIGN.LEFT)

    add_page_number(slide, 14)


def slide_15_thanks(prs):
    slide = add_slide(prs)
    add_background_bar(slide, NAVY, 0.5)

    # Центральная плашка
    add_card(slide, 4.0, 5.5, 25.87, 7.0,
             "",
             title="Спасибо за внимание!",
             title_size=60, size=14, fill=GOLD, align=PP_ALIGN.CENTER, color=NAVY)

    add_textbox(slide, 4.0, 13.0, 25.87, 4.0,
                "Михайлов Денис Артемович · группа ЭФБО-06-22\n"
                "Научный руководитель: к.т.н., доцент Лукьянов П. В.\n"
                "Кафедра индустриального программирования ИПТИП МИРЭА\n\n"
                "Готов ответить на ваши вопросы",
                size=18, color=NAVY, align=PP_ALIGN.CENTER,
                anchor=MSO_ANCHOR.TOP)


# ---------- main ----------

def main():
    prs = Presentation()
    prs.slide_width = SLIDE_W
    prs.slide_height = SLIDE_H

    slide_01_title(prs)
    slide_02_relevance(prs)
    slide_03_goal_tasks(prs)
    slide_04_market(prs)
    slide_05_requirements(prs)
    slide_06_actors_processes(prs)
    slide_07_to_be(prs)
    slide_08_usecase(prs)
    slide_09_architecture(prs)
    slide_10_algorithms(prs)
    slide_11_implementation(prs)
    slide_12_testing(prs)
    slide_13_efficiency(prs)
    slide_14_perspectives(prs)
    slide_15_thanks(prs)

    OUT.parent.mkdir(parents=True, exist_ok=True)
    prs.save(str(OUT))
    print(f"✅ {OUT}")
    print(f"Всего слайдов: {len(prs.slides)}")


if __name__ == "__main__":
    main()
