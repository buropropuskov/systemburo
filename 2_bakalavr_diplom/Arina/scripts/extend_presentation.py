"""Дополнение презентации Арины до 16 слайдов.
Сохраняет стиль существующих слайдов (Times New Roman, золотые карточки F1D669,
заголовок 40pt, тело 16-20pt). Добавляет 8 новых слайдов перед финальным.
"""
import copy
from pathlib import Path

from pptx import Presentation
from pptx.util import Cm, Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.oxml.ns import qn
from lxml import etree

SRC = Path("/home/washka/.var/app/org.telegram.desktop/data/TelegramDesktop/tdata/temp_data/presentation_arina.pptx")
DST = Path("/home/washka/project/diplom/Arina/presentation/presentation_arina_full.pptx")
IMG = Path("/home/washka/project/diplom/Arina/presentation")

GOLD = RGBColor(0xF1, 0xD6, 0x69)
GOLD_LIGHT = RGBColor(0xFA, 0xE9, 0xA8)
GOLD_DARK = RGBColor(0xD4, 0xB3, 0x40)
NAVY = RGBColor(0x1F, 0x3A, 0x5F)
BLACK = RGBColor(0x00, 0x00, 0x00)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
GRAY = RGBColor(0x70, 0x70, 0x70)
LIGHT_GRAY = RGBColor(0xE8, 0xE8, 0xE8)
RED = RGBColor(0xC0, 0x39, 0x2B)
GREEN = RGBColor(0x27, 0xAE, 0x60)

FONT = "Times New Roman"

# ---------- Вспомогательные ----------

def set_text(shape, text: str, *, size: int = 16, bold: bool = False,
             align=PP_ALIGN.LEFT, color=BLACK, anchor=MSO_ANCHOR.TOP):
    tf = shape.text_frame
    tf.clear()
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    lines = text.split("\n")
    for i, line in enumerate(lines):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = align
        r = p.add_run()
        r.text = line
        r.font.name = FONT
        r.font.size = Pt(size)
        r.font.bold = bold
        r.font.color.rgb = color


def add_textbox(slide, x, y, w, h, text, *, size=16, bold=False,
                align=PP_ALIGN.LEFT, color=BLACK, anchor=MSO_ANCHOR.TOP):
    tb = slide.shapes.add_textbox(Cm(x), Cm(y), Cm(w), Cm(h))
    set_text(tb, text, size=size, bold=bold, align=align, color=color, anchor=anchor)
    tb.text_frame.margin_left = Cm(0.1)
    tb.text_frame.margin_right = Cm(0.1)
    tb.text_frame.margin_top = Cm(0.05)
    tb.text_frame.margin_bottom = Cm(0.05)
    return tb


def add_card(slide, x, y, w, h, text, *, fill=GOLD, size=16,
             title_size=None, title=None, align=PP_ALIGN.CENTER,
             color=BLACK, border=None):
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
    parts.append((text, size, False))
    for i, (t, sz, b) in enumerate(parts):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = align
        for j, line in enumerate(t.split("\n")):
            if j > 0:
                p = tf.add_paragraph()
                p.alignment = align
            r = p.add_run()
            r.text = line
            r.font.name = FONT
            r.font.size = Pt(sz)
            r.font.bold = b
            r.font.color.rgb = color
    return shp


def add_title(slide, text: str):
    tb = slide.shapes.add_textbox(Cm(0.8), Cm(0.9), Cm(32), Cm(1.97))
    set_text(tb, text, size=40, bold=True, color=NAVY, anchor=MSO_ANCHOR.MIDDLE)
    return tb


def add_page_number(slide, n: int):
    # маленький блок с номером страницы внизу справа
    tb = slide.shapes.add_textbox(Cm(32.2), Cm(18.2), Cm(1.2), Cm(0.5))
    set_text(tb, str(n), size=11, align=PP_ALIGN.RIGHT, color=GRAY)


def add_logo(slide):
    logo = IMG.parent / "images" / "logo_mirea.png"
    # если логотипа нет — просто не добавляем; структура без него допустима


def add_table(slide, x, y, w, h, headers, rows, *,
              header_fill=NAVY, header_color=WHITE,
              alt_fill=GOLD_LIGHT, body_size=13, header_size=13):
    n_rows = len(rows) + 1
    n_cols = len(headers)
    tbl = slide.shapes.add_table(n_rows, n_cols, Cm(x), Cm(y), Cm(w), Cm(h)).table
    # заголовок
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
        cell.margin_left = Cm(0.15)
        cell.margin_right = Cm(0.15)
        cell.margin_top = Cm(0.05)
        cell.margin_bottom = Cm(0.05)
    # тело
    for ri, row in enumerate(rows):
        is_alt = ri % 2 == 1
        for ci, val in enumerate(row):
            cell = tbl.cell(ri + 1, ci)
            if is_alt:
                cell.fill.solid()
                cell.fill.fore_color.rgb = alt_fill
            else:
                cell.fill.solid()
                cell.fill.fore_color.rgb = WHITE
            tf = cell.text_frame
            tf.clear()
            # allow multi-paragraph cells via newlines
            lines = str(val).split("\n")
            for li, line in enumerate(lines):
                p = tf.paragraphs[0] if li == 0 else tf.add_paragraph()
                p.alignment = PP_ALIGN.LEFT if ci > 0 else PP_ALIGN.CENTER
                r = p.add_run()
                r.text = line
                r.font.name = FONT
                r.font.size = Pt(body_size)
                r.font.color.rgb = BLACK
            cell.vertical_anchor = MSO_ANCHOR.MIDDLE
            cell.margin_left = Cm(0.15)
            cell.margin_right = Cm(0.15)
            cell.margin_top = Cm(0.05)
            cell.margin_bottom = Cm(0.05)
    return tbl


def add_image(slide, path: Path, x, y, w=None, h=None):
    if w is None and h is None:
        pic = slide.shapes.add_picture(str(path), Cm(x), Cm(y))
    elif h is None:
        pic = slide.shapes.add_picture(str(path), Cm(x), Cm(y), width=Cm(w))
    elif w is None:
        pic = slide.shapes.add_picture(str(path), Cm(x), Cm(y), height=Cm(h))
    else:
        pic = slide.shapes.add_picture(str(path), Cm(x), Cm(y), width=Cm(w), height=Cm(h))
    return pic


def add_blank_slide(prs):
    # использует "Базовый слайд с диаграммой" если есть, иначе пустой
    target_layout_name = "Базовый слайд с диаграммой"
    layout = prs.slide_layouts[0]
    for l in prs.slide_layouts:
        if l.name == target_layout_name:
            layout = l
            break
    slide = prs.slides.add_slide(layout)
    # удалим все placeholder'ы с исходного layout'а чтобы контролировать состав
    for shape in list(slide.shapes):
        if shape.is_placeholder:
            sp = shape._element
            sp.getparent().remove(sp)
    return slide


# ---------- Построение слайдов ----------

def build_slide_pyramid(prs):
    slide = add_blank_slide(prs)
    add_title(slide, "Пирамида тестирования и план")

    # Картинка пирамиды слева
    add_image(slide, IMG / "pres_pyramid.png", 0.5, 3.5, w=19.0)

    # Справа — план тестирования (компактная таблица этапов)
    add_textbox(slide, 20.0, 3.3, 13.2, 0.9,
                "План тестирования: 5 этапов с критериями входа и выхода",
                size=16, bold=True, color=NAVY)

    stages = [
        ("1. Подготовка среды",
         "Docker-compose, CI, тестовые данные"),
        ("2. Разработка тестов",
         "Приоритет «Высокий» покрыт 100%, бэкенд ≥ 70%"),
        ("3. Выполнение",
         "Авто-прогон в CI + ручной юзабилити"),
        ("4. Анализ результатов",
         "Баг-репорты, приоритизация, исправления"),
        ("5. Отчётность",
         "Метрики, покрытие, архивация артефактов"),
    ]
    y = 4.3
    for t, s in stages:
        add_card(slide, 20.0, y, 13.2, 1.55, s,
                 title=t, size=12, title_size=14, fill=GOLD)
        y += 1.75
    # подпись внизу
    add_textbox(slide, 0.8, 17.6, 32.0, 0.8,
                "~350 модульных + ~80 интеграционных + ~25 E2E = суммарное время в CI ≤ 10 минут",
                size=14, bold=True, color=NAVY, align=PP_ALIGN.CENTER)
    add_page_number(slide, 8)


def build_slide_tools(prs):
    slide = add_blank_slide(prs)
    add_title(slide, "Выбор инструментов автоматизации")

    # подзаголовок
    add_textbox(slide, 0.8, 3.2, 32.0, 0.9,
                "Сравнили по 4–6 критериев для каждого уровня — победители внизу таблицы",
                size=16, color=GRAY, align=PP_ALIGN.LEFT)

    headers = ["Уровень", "Кандидаты", "Выбор", "Ключевой аргумент"]
    rows = [
        ("Модульный\n(фронтенд)", "Jest / Vitest / Mocha", "Vitest",
         "Общий автор с Vite, холодный старт 1.8 с (вдвое быстрее Jest)"),
        ("Модульный,\nинтеграц. (Go)", "testing / testify / gomega / gocheck",
         "testing + testify",
         "Читаемые assert-ы, моки «из коробки», флаг -race"),
        ("API", "Postman / Insomnia / curl / Hoppscotch",
         "Postman + Newman",
         "Коллекции в Git, pre-request скрипт на JWT, CI через Newman"),
        ("E2E", "Playwright / Cypress / Selenium",
         "Playwright",
         "Автоожидание, Chromium+Firefox+WebKit, шардирование на 3 воркера"),
        ("CI/CD", "GitHub Actions / GitLab CI / Jenkins / CircleCI",
         "GitHub Actions",
         "Репозитории уже на GitHub, 2000 мин/мес бесплатно, service containers"),
        ("Нагрузочный",
         "k6 / Apache JMeter",
         "k6",
         "JS-сценарии, легковесный; запуск вручную перед релизом"),
    ]
    add_table(slide, 0.8, 4.4, 32.4, 13.5,
              headers, rows, body_size=14, header_size=14)

    add_page_number(slide, 9)


def build_slide_unit(prs):
    slide = add_blank_slide(prs)
    add_title(slide, "Реализация: среда и модульные тесты")

    # левая колонка — docker-среда
    add_card(slide, 0.8, 3.3, 15.8, 5.2,
             "• bekkend-тест (Go + Gin)\n"
             "• frontend-тест (Vue 3)\n"
             "• PostgreSQL :5433 (изолированная)\n"
             "• S3-совместимое хранилище (MinIO)\n"
             "• Миграции через golang-migrate\n"
             "• .env.test + godotenv для TestMain",
             title="Тестовая среда — docker-compose.test.yml",
             title_size=17, size=15, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    # правая колонка — что проверяем
    add_card(slide, 17.2, 3.3, 16.0, 5.2,
             "Фронтенд (47 тестов):\n"
             "  • DocumentUpload — формат, размер 15 МБ, блокировка кнопки\n"
             "  • RiskHighlighter — подсветка, перекрытие диапазонов\n"
             "  • useDocumentStore — Pinia-стор, fetch через vi.fn()\n\n"
             "Бэкенд (38 тестов):\n"
             "  • Handler.Upload через httptest + multipart\n"
             "  • NER: [PHONE] для «8(999)...» и «+7-999-...»\n"
             "  • JWT middleware — валидный, истёкший, отсутствие",
             title="Объекты тестирования и найденные баги",
             title_size=17, size=13, fill=GOLD, align=PP_ALIGN.LEFT)

    # примеры кода-миниатюры
    add_textbox(slide, 0.8, 8.8, 16.0, 0.8,
                "Пример 1 — Vitest (DocumentUpload.vue)",
                size=14, bold=True, color=NAVY)
    add_code_box(slide, 0.8, 9.7, 16.0, 7.8,
                 "it('отклоняет PNG и блокирует кнопку', async () => {\n"
                 "  const wrapper = mount(DocumentUpload)\n"
                 "  const file = new File(['x'], 'img.png',\n"
                 "      { type: 'image/png' })\n"
                 "  await input.trigger('change')\n"
                 "  expect(wrapper.text())\n"
                 "    .toContain('Формат не поддерживается')\n"
                 "  expect(analyzeBtn.attributes('disabled'))\n"
                 "    .toBeDefined()\n"
                 "})")

    add_textbox(slide, 17.2, 8.8, 16.0, 0.8,
                "Пример 2 — Go testify (handler.Upload)",
                size=14, bold=True, color=NAVY)
    add_code_box(slide, 17.2, 9.7, 16.0, 7.8,
                 "func TestUploadDocument_Success(t *testing.T) {\n"
                 "  mockSvc := new(mocks.DocumentService)\n"
                 "  mockSvc.On(\"Upload\", ...).\n"
                 "      Return(\"doc-uuid-1234\", nil)\n"
                 "  req := httptest.NewRequest(POST, \"/upload\",\n"
                 "      body)\n"
                 "  router.ServeHTTP(w, req)\n"
                 "  assert.Equal(t, http.StatusCreated, w.Code)\n"
                 "  mockSvc.AssertExpectations(t)\n"
                 "}")

    add_page_number(slide, 10)


def add_code_box(slide, x, y, w, h, code: str):
    shp = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, Cm(x), Cm(y), Cm(w), Cm(h))
    shp.fill.solid()
    shp.fill.fore_color.rgb = RGBColor(0xF6, 0xF6, 0xF0)
    shp.line.color.rgb = GRAY
    shp.line.width = Pt(0.75)
    tf = shp.text_frame
    tf.clear()
    tf.word_wrap = True
    tf.margin_left = Cm(0.25)
    tf.margin_right = Cm(0.15)
    tf.margin_top = Cm(0.15)
    tf.margin_bottom = Cm(0.15)
    for i, line in enumerate(code.split("\n")):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = PP_ALIGN.LEFT
        r = p.add_run()
        r.text = line if line else " "
        r.font.name = "Consolas"
        r.font.size = Pt(12)
        r.font.color.rgb = BLACK


def build_slide_integration(prs):
    slide = add_blank_slide(prs)
    add_title(slide, "Интеграционные, API и сквозные тесты")

    # три карточки — по одной на уровень
    add_card(slide, 0.5, 3.3, 10.8, 6.2,
             "34 сценария в 5 папках:\n"
             "авторизация (6) • документы (9) •\n"
             "анализ (7) • отчёты (5) • негативные (7)\n\n"
             "Цепочка: регистрация → JWT\n"
             "сохраняется в окружение → используется\n"
             "в защищённых запросах\n\n"
             "Запуск через Newman в CI",
             title="API (Postman / Newman)",
             title_size=17, size=13, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    add_card(slide, 11.6, 3.3, 10.8, 6.2,
             "Реальная PostgreSQL в docker-compose:\n"
             "один тест = 3,4 с (против 12 мс модульного)\n\n"
             "Ловит невидимое на моках:\n"
             "• ошибки миграций\n"
             "• нарушения FK-ограничений\n"
             "• кривые SQL-запросы\n\n"
             "Go testing + testcontainers",
             title="Интеграционные (Go)",
             title_size=17, size=13, fill=GOLD, align=PP_ALIGN.LEFT)

    add_card(slide, 22.7, 3.3, 10.8, 6.2,
             "Playwright, 4 ключевых сценария:\n"
             "• регистрация и вход\n"
             "• загрузка + AI-анализ\n"
             "• чат с AI-ассистентом\n"
             "• формирование PDF-отчёта\n\n"
             "Параллелизм 2 воркера,\n"
             "таймаут 30 с, Chromium + Firefox",
             title="E2E (Playwright)",
             title_size=17, size=13, fill=GOLD_DARK, align=PP_ALIGN.LEFT)

    # критический баг 403 vs 404
    ax = add_card(slide, 0.5, 10.1, 17.0, 4.7,
             "Postman-тест из негативного набора запросил чужой документ.\n"
             "Сервер вернул 404 вместо 403: злоумышленник мог бы перебором\n"
             "идентификаторов определить существующие документы в системе.\n\n"
             "Дефект классифицирован как критический, исправлен за 2 часа.",
             title="Критический дефект #1: 404 вместо 403 для чужого документа",
             title_size=15, size=13, fill=RGBColor(0xFF, 0xEC, 0xE8),
             align=PP_ALIGN.LEFT, border=RED)

    # примерный playwright-кусок
    add_textbox(slide, 17.8, 10.1, 15.8, 0.7,
                "Playwright: ключевой шаг E2E-сценария",
                size=14, bold=True, color=NAVY)
    add_code_box(slide, 17.8, 10.9, 15.8, 6.8,
                 "await page.getByRole('button',\n"
                 "    { name: 'Анализировать' }).click()\n\n"
                 "// AI-анализ до 15 с\n"
                 "await expect(page.getByTestId('risk-panel'))\n"
                 "    .toBeVisible({ timeout: 20000 })\n\n"
                 "const count = await page\n"
                 "    .getByTestId('risk-item').count()\n"
                 "expect(count).toBeGreaterThan(0)")

    add_page_number(slide, 11)


def build_slide_security(prs):
    slide = add_blank_slide(prs)
    add_title(slide, "Безопасность и юзабилити-тестирование")

    # секция безопасности (слева)
    add_textbox(slide, 0.8, 3.3, 16.0, 0.9,
                "Безопасность — проверено по 5 направлениям",
                size=18, bold=True, color=NAVY)
    sec = [
        ("JWT", "подмена payload, истёкший токен → 401", "исправлен 500→401 за 40 мин"),
        ("XSS", "<script> в заметках и ответах AI", "v-html → ChatMessage (marked + sanitize)"),
        ("SQL-инъекции", "'; DROP TABLE documents; --", "12 точек ввода чистые (sqlx)"),
        ("Изоляция", "запрос чужого документа", "403 на 4 эндпоинтах"),
        ("OWASP ZAP", "1 847 запросов за 6 мин",
         "критических 0, 2 low: HSTS, X-Content-Type"),
    ]
    y = 4.3
    for cat, attack, result in sec:
        add_card(slide, 0.8, y, 16.0, 1.5,
                 f"Атака: {attack}\nРезультат: {result}",
                 title=cat, title_size=13, size=11, fill=GOLD_LIGHT,
                 align=PP_ALIGN.LEFT)
        y += 1.7

    # секция юзабилити (справа)
    add_textbox(slide, 17.2, 3.3, 16.0, 0.9,
                "Юзабилити — 5 респондентов, основной сценарий",
                size=18, bold=True, color=NAVY)

    # цифры
    add_card(slide, 17.2, 4.3, 7.8, 3.5, "",
             title="2:14", title_size=42,
             fill=GOLD, size=14)
    add_textbox(slide, 17.2, 7.5, 7.8, 0.7,
                "среднее время прохождения (требование 3:00)",
                size=12, color=NAVY, align=PP_ALIGN.CENTER)

    add_card(slide, 25.4, 4.3, 7.8, 3.5, "",
             title="2:51", title_size=42,
             fill=GOLD_DARK, size=14)
    add_textbox(slide, 25.4, 7.5, 7.8, 0.7,
                "самый медленный участник",
                size=12, color=NAVY, align=PP_ALIGN.CENTER)

    # список улучшений
    add_card(slide, 17.2, 9.2, 16.0, 7.8,
             "• Прогресс-бар увеличен на 40%, добавлена текстовая подпись «Загрузка...»\n"
             "• Tabindex=0 у панели рисков — теперь доступна через клавиатуру\n"
             "• Tooltip не уходит за правый край экрана\n"
             "• Панель рисков: width: min(280px, 90vw) — не перекрывает кнопку на 320 px\n"
             "• Текст документа max-width: 900 px — читаемо на QHD-мониторах",
             title="Внесённые улучшения интерфейса",
             title_size=16, size=13, fill=GOLD_LIGHT, align=PP_ALIGN.LEFT)

    add_page_number(slide, 12)


def build_slide_cicd(prs):
    slide = add_blank_slide(prs)
    add_title(slide, "CI/CD-пайплайн на GitHub Actions")
    add_image(slide, IMG / "pres_cicd.png", 0.5, 3.4, w=33.0)
    add_page_number(slide, 13)


def build_slide_results(prs):
    slide = add_blank_slide(prs)
    add_title(slide, "Результаты тестирования")

    add_image(slide, IMG / "pres_results.png", 0.5, 3.3, w=22.0)

    # справа — pie/bar дефектов
    add_image(slide, IMG / "pres_defects.png", 22.5, 3.3, w=11.0)

    # выводы снизу
    add_card(slide, 0.5, 15.0, 33.0, 3.3,
             "• 246 тестов, покрытие ~73% — выше планового порога 70%/60%\n"
             "• Все 3 критических дефекта устранены до выпуска\n"
             "• Все эндпоинты (кроме AI-чата) укладываются в p95 ≤ 200 мс\n"
             "• Стабильность пайплайна 96,5% за 143 прогона — тестам можно доверять",
             title="Итог: система готова к релизу",
             title_size=18, size=14, fill=GOLD, align=PP_ALIGN.LEFT)

    add_page_number(slide, 14)


def build_slide_architecture_detail(prs):
    # заменяем слайд 6 картинкой полной архитектуры + стеком
    pass  # оставим слайд 6 как есть, добавим архитектуру как отдельный ребёнок 6-го


# ---------- main ----------

def main():
    prs = Presentation(str(SRC))
    print(f"Открыл презентацию, слайдов: {len(prs.slides)}")

    # Удалим финальный "Спасибо за внимание" — потом добавим заново последним
    # чтобы вставить новые слайды перед ним
    thanks_slide_idx = None
    for i, s in enumerate(prs.slides):
        for shape in s.shapes:
            if shape.has_text_frame and "Спасибо за внимание" in shape.text_frame.text:
                thanks_slide_idx = i
                break
    # удаляем слайд (через sldIdLst)
    if thanks_slide_idx is not None:
        xml_slides = prs.slides._sldIdLst
        slides = list(xml_slides)
        rId = slides[thanks_slide_idx].get(qn("r:id"))
        prs.part.drop_rel(rId)
        xml_slides.remove(slides[thanks_slide_idx])
        print(f"Удалён финальный слайд (был #{thanks_slide_idx + 1})")

    # Добавляем новые слайды
    build_slide_pyramid(prs)           # 8
    build_slide_tools(prs)             # 9
    build_slide_unit(prs)              # 10
    build_slide_integration(prs)       # 11
    build_slide_security(prs)          # 12
    build_slide_cicd(prs)              # 13
    build_slide_results(prs)           # 14

    # Финальный слайд
    layout_names = [l.name for l in prs.slide_layouts]
    final_layout = None
    for l in prs.slide_layouts:
        if l.name in ("ИТХТ", "Финальный слайд"):
            final_layout = l
            break
    if final_layout is None:
        final_layout = prs.slide_layouts[0]
    slide = prs.slides.add_slide(final_layout)
    # почистим placeholder-ы и добавим свои
    for shape in list(slide.shapes):
        if shape.is_placeholder:
            sp = shape._element
            sp.getparent().remove(sp)
    tb = slide.shapes.add_textbox(Cm(6.5), Cm(7.5), Cm(21), Cm(3.5))
    set_text(tb, "Спасибо за внимание!", size=54, bold=True,
             color=NAVY, align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    tb2 = slide.shapes.add_textbox(Cm(4), Cm(11.2), Cm(26), Cm(4))
    set_text(tb2,
             "Кафанова А. С., группа ЭФБО-02-22\n"
             "Научный руководитель: к.пед.н., доцент Громов Е. В.\n"
             "Кафедра индустриального программирования ИПТИП МИРЭА",
             size=18, color=BLACK, align=PP_ALIGN.CENTER,
             anchor=MSO_ANCHOR.TOP)

    DST.parent.mkdir(parents=True, exist_ok=True)
    prs.save(str(DST))
    print(f"\n✅ Сохранено: {DST}")
    print(f"Всего слайдов: {len(prs.slides)}")


if __name__ == "__main__":
    main()
