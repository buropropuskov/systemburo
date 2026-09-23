#!/usr/bin/env python3
"""
Редактор diplom_myakotnykh.docx: применяет правки по списку требований руководителя.

Идемпотентный запуск: всегда читает diplom_myakotnykh_original.docx, применяет правки,
сохраняет в diplom_myakotnykh.docx.

Использование:
    python3 scripts/edit-docx.py
"""

from __future__ import annotations

import copy
import sys
from pathlib import Path
from typing import Iterable, Optional

from docx import Document
from docx.document import Document as DocumentCls
from docx.enum.table import WD_TABLE_ALIGNMENT
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.shared import Cm, Pt, RGBColor

BASE_DIR = Path("/home/washka/project/diplom/Sergey")
SRC_DOCX = BASE_DIR / "diplom_myakotnykh_original.docx"
OUT_DOCX = BASE_DIR / "diplom_myakotnykh_v5.docx"
IMAGES_DIR = BASE_DIR / "images"

FONT_NAME = "Times New Roman"
FONT_MONO = "Courier New"
BODY_SIZE = Pt(14)
TABLE_SIZE = Pt(12)
CODE_SIZE = Pt(10)


# ============================================================
# Хелперы работы с XML docx
# ============================================================

def _set_run_font(run, name: str = FONT_NAME, size: Optional[Pt] = None, bold: Optional[bool] = None,
                  italic: Optional[bool] = None):
    run.font.name = name
    rPr = run._element.get_or_add_rPr()
    rFonts = rPr.find(qn("w:rFonts"))
    if rFonts is None:
        rFonts = OxmlElement("w:rFonts")
        rPr.append(rFonts)
    for attr in ("w:ascii", "w:hAnsi", "w:cs", "w:eastAsia"):
        rFonts.set(qn(attr), name)
    if size is not None:
        run.font.size = size
    if bold is not None:
        run.bold = bold
    if italic is not None:
        run.italic = italic


STYLE_IDS = {
    "Heading 1": "1",
    "Heading 2": "21",
    "Heading 3": "31",
    "Normal": "Normal",
}


def _make_paragraph(text: str, style: str = "Normal", align=None, bold: bool = False,
                    italic: bool = False, size: Pt = BODY_SIZE,
                    first_line_indent: Optional[Cm] = None, font: str = FONT_NAME):
    """Создать <w:p> XML-элемент с нужным стилем/форматированием."""
    p = OxmlElement("w:p")
    pPr = OxmlElement("w:pPr")
    pStyle = OxmlElement("w:pStyle")
    # нормализуем стили: "Heading1"/"Heading 1" → style_id в STYLE_IDS
    key = style if style in STYLE_IDS else style.replace("Heading", "Heading ").strip()
    style_id = STYLE_IDS.get(key, STYLE_IDS.get(style, style))
    pStyle.set(qn("w:val"), style_id)
    pPr.append(pStyle)
    if align is not None:
        jc = OxmlElement("w:jc")
        jc.set(qn("w:val"), {
            WD_ALIGN_PARAGRAPH.CENTER: "center",
            WD_ALIGN_PARAGRAPH.LEFT: "left",
            WD_ALIGN_PARAGRAPH.RIGHT: "right",
            WD_ALIGN_PARAGRAPH.JUSTIFY: "both",
        }.get(align, "both"))
        pPr.append(jc)
    if first_line_indent is not None:
        ind = OxmlElement("w:ind")
        twips = int(first_line_indent.cm * 567)
        ind.set(qn("w:firstLine"), str(twips))
        pPr.append(ind)
    p.append(pPr)

    r = OxmlElement("w:r")
    rPr = OxmlElement("w:rPr")
    rFonts = OxmlElement("w:rFonts")
    for a in ("w:ascii", "w:hAnsi", "w:cs", "w:eastAsia"):
        rFonts.set(qn(a), font)
    rPr.append(rFonts)
    sz = OxmlElement("w:sz")
    sz.set(qn("w:val"), str(int(size.pt * 2)))
    rPr.append(sz)
    szCs = OxmlElement("w:szCs")
    szCs.set(qn("w:val"), str(int(size.pt * 2)))
    rPr.append(szCs)
    if bold:
        rPr.append(OxmlElement("w:b"))
    if italic:
        rPr.append(OxmlElement("w:i"))
    r.append(rPr)
    t = OxmlElement("w:t")
    t.text = text
    t.set(qn("xml:space"), "preserve")
    r.append(t)
    p.append(r)
    return p


def find_heading_idx(doc: DocumentCls, text: str, start: int = 0) -> int:
    for i in range(start, len(doc.paragraphs)):
        p = doc.paragraphs[i]
        if p.style.name.startswith("Heading") and p.text.strip().startswith(text):
            return i
    return -1


def find_para_idx(doc: DocumentCls, text: str, start: int = 0) -> int:
    for i in range(start, len(doc.paragraphs)):
        if doc.paragraphs[i].text.strip().startswith(text):
            return i
    return -1


def insert_after(anchor_element, new_element):
    """Вставить new_element сразу после anchor_element."""
    anchor_element.addnext(new_element)


def add_body_after(anchor_element, text: str, align=WD_ALIGN_PARAGRAPH.JUSTIFY,
                   first_line_indent: Cm = Cm(1.25), bold: bool = False, italic: bool = False):
    p = _make_paragraph(text, style="Normal", align=align, bold=bold, italic=italic,
                        first_line_indent=first_line_indent)
    anchor_element.addnext(p)
    return p


def add_heading_after(anchor_element, text: str, level: int = 2):
    style = {1: "Heading1", 2: "Heading2", 3: "Heading3"}[level]
    size = Pt({1: 18, 2: 16, 3: 14}[level])
    p = _make_paragraph(text, style=style, align=WD_ALIGN_PARAGRAPH.JUSTIFY, bold=True, size=size)
    anchor_element.addnext(p)
    return p


def add_fig_caption_after(anchor_element, text: str):
    """Подпись рисунка: TNR 12pt, полужирный, по центру, тире '–' (en dash)."""
    text = text.replace(" -- ", " – ")
    p = _make_paragraph(text, style="Normal", align=WD_ALIGN_PARAGRAPH.CENTER,
                        bold=True, size=TABLE_SIZE)
    anchor_element.addnext(p)
    return p


def add_table_caption_after(anchor_element, text: str):
    """Надпись таблицы: TNR 12pt, курсив, по левому краю, тире '–'."""
    text = text.replace(" -- ", " – ")
    p = _make_paragraph(text, style="Normal", align=WD_ALIGN_PARAGRAPH.LEFT,
                        italic=True, size=TABLE_SIZE)
    anchor_element.addnext(p)
    return p


def add_listing_caption_after(anchor_element, text: str):
    """Надпись листинга: TNR 12pt, курсив, по левому, тире '–'."""
    text = text.replace(" -- ", " – ")
    p = _make_paragraph(text, style="Normal", align=WD_ALIGN_PARAGRAPH.LEFT,
                        italic=True, size=TABLE_SIZE)
    anchor_element.addnext(p)
    return p


def add_picture_after(doc: DocumentCls, anchor_element, image_path: Path, width_cm: float = 15):
    tmp = doc.add_paragraph()
    tmp.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = tmp.add_run()
    if image_path.exists():
        run.add_picture(str(image_path), width=Cm(width_cm))
    else:
        run.add_text(f"[Изображение не найдено: {image_path.name}]")
        _set_run_font(run, italic=True, size=TABLE_SIZE)
    el = tmp._element
    el.getparent().remove(el)
    anchor_element.addnext(el)
    return el


CONTENT_WIDTH_TWIPS = 9355  # 165 мм при полях А4 30/15 мм = доступная ширина контента


def add_table_after(doc: DocumentCls, anchor_element, headers: list[str], rows: list[list[str]],
                    col_widths_cm: Optional[list[float]] = None, small: bool = False):
    """Таблица с точной twip-шириной. tblW=auto (как в эталоне), tcW суммируется в 9355 twips."""
    tbl = doc.add_table(rows=len(rows) + 1, cols=len(headers))
    tbl.alignment = WD_TABLE_ALIGNMENT.CENTER
    tbl.style = "Table Grid"
    size = Pt(11) if small else TABLE_SIZE
    for j, h in enumerate(headers):
        cell = tbl.rows[0].cells[j]
        cell.text = ""
        p = cell.paragraphs[0]
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        run = p.add_run(h)
        _set_run_font(run, size=size, bold=True)
    for i, row in enumerate(rows):
        for j, val in enumerate(row):
            cell = tbl.rows[i + 1].cells[j]
            cell.text = ""
            p = cell.paragraphs[0]
            p.alignment = WD_ALIGN_PARAGRAPH.LEFT
            run = p.add_run(val)
            _set_run_font(run, size=size)
    if col_widths_cm:
        # 1. tblW ставим в auto (w:w=0, w:type=auto) — как в эталонном docx
        tblPr = tbl._element.find(qn("w:tblPr"))
        if tblPr is not None:
            tblW = tblPr.find(qn("w:tblW"))
            if tblW is None:
                tblW = OxmlElement("w:tblW")
                tblPr.append(tblW)
            tblW.set(qn("w:w"), "0")
            tblW.set(qn("w:type"), "auto")
        # 2. tcW в twips с точной суммой CONTENT_WIDTH_TWIPS
        total_cm = sum(col_widths_cm)
        twips = [int(round(w / total_cm * CONTENT_WIDTH_TWIPS)) for w in col_widths_cm]
        # компенсируем накопленную ошибку округления в последней колонке
        twips[-1] += CONTENT_WIDTH_TWIPS - sum(twips)
        for j, w_tw in enumerate(twips):
            for row in tbl.rows:
                cell = row.cells[j]
                tcPr = cell._tc.find(qn("w:tcPr"))
                if tcPr is None:
                    tcPr = OxmlElement("w:tcPr")
                    cell._tc.insert(0, tcPr)
                old = tcPr.find(qn("w:tcW"))
                if old is not None:
                    tcPr.remove(old)
                tcW = OxmlElement("w:tcW")
                tcW.set(qn("w:w"), str(w_tw))
                tcW.set(qn("w:type"), "dxa")
                tcPr.append(tcW)
    el = tbl._element
    el.getparent().remove(el)
    anchor_element.addnext(el)
    return el


def _make_sect_p(orient: str) -> OxmlElement:
    """Параграф с sectPr заданной ориентации."""
    p = OxmlElement("w:p")
    pPr = OxmlElement("w:pPr")
    sectPr = OxmlElement("w:sectPr")
    pgSz = OxmlElement("w:pgSz")
    if orient == "landscape":
        pgSz.set(qn("w:w"), "16838")
        pgSz.set(qn("w:h"), "11906")
        pgSz.set(qn("w:orient"), "landscape")
    else:
        pgSz.set(qn("w:w"), "11906")
        pgSz.set(qn("w:h"), "16838")
        pgSz.set(qn("w:orient"), "portrait")
    sectPr.append(pgSz)
    pgMar = OxmlElement("w:pgMar")
    pgMar.set(qn("w:top"), "1134")
    pgMar.set(qn("w:right"), "851")
    pgMar.set(qn("w:bottom"), "1134")
    pgMar.set(qn("w:left"), "1701")
    pgMar.set(qn("w:header"), "708")
    pgMar.set(qn("w:footer"), "708")
    pgMar.set(qn("w:gutter"), "0")
    sectPr.append(pgMar)
    pPr.append(sectPr)
    p.append(pPr)
    return p


def add_landscape_section_after(doc: DocumentCls, anchor_element):
    """Вставить разрыв альбомной секции после anchor_element. Возвращает сам sectPr-параграф."""
    from docx.enum.section import WD_ORIENT, WD_SECTION
    # создаём временный параграф с pageBreakBefore+sectPr
    p = OxmlElement("w:p")
    pPr = OxmlElement("w:pPr")
    sectPr = OxmlElement("w:sectPr")
    pgSz = OxmlElement("w:pgSz")
    pgSz.set(qn("w:w"), "16838")  # A4 landscape width (twips)
    pgSz.set(qn("w:h"), "11906")
    pgSz.set(qn("w:orient"), "landscape")
    sectPr.append(pgSz)
    pgMar = OxmlElement("w:pgMar")
    pgMar.set(qn("w:top"), "1134")
    pgMar.set(qn("w:right"), "851")
    pgMar.set(qn("w:bottom"), "1134")
    pgMar.set(qn("w:left"), "1701")
    pgMar.set(qn("w:header"), "708")
    pgMar.set(qn("w:footer"), "708")
    pgMar.set(qn("w:gutter"), "0")
    sectPr.append(pgMar)
    pPr.append(sectPr)
    p.append(pPr)
    anchor_element.addnext(p)
    return p


def add_portrait_section_after(doc: DocumentCls, anchor_element):
    """Вернуть книжную ориентацию после альбомной вставки."""
    p = OxmlElement("w:p")
    pPr = OxmlElement("w:pPr")
    sectPr = OxmlElement("w:sectPr")
    pgSz = OxmlElement("w:pgSz")
    pgSz.set(qn("w:w"), "11906")
    pgSz.set(qn("w:h"), "16838")
    pgSz.set(qn("w:orient"), "portrait")
    sectPr.append(pgSz)
    pgMar = OxmlElement("w:pgMar")
    pgMar.set(qn("w:top"), "1134")
    pgMar.set(qn("w:right"), "851")
    pgMar.set(qn("w:bottom"), "1134")
    pgMar.set(qn("w:left"), "1701")
    pgMar.set(qn("w:header"), "708")
    pgMar.set(qn("w:footer"), "708")
    pgMar.set(qn("w:gutter"), "0")
    sectPr.append(pgMar)
    pPr.append(sectPr)
    p.append(pPr)
    anchor_element.addnext(p)
    return p


def add_code_block_after(doc: DocumentCls, anchor_element, code: str):
    lines = code.rstrip().split("\n")
    tbl = doc.add_table(rows=1, cols=1)
    tbl.alignment = WD_TABLE_ALIGNMENT.LEFT
    tbl.style = "Table Grid"
    cell = tbl.rows[0].cells[0]
    cell.text = ""
    for i, line in enumerate(lines):
        p = cell.paragraphs[0] if i == 0 else cell.add_paragraph()
        p.alignment = WD_ALIGN_PARAGRAPH.LEFT
        p.paragraph_format.line_spacing = 1.0
        run = p.add_run(line.replace("\t", "    ") or " ")
        _set_run_font(run, name=FONT_MONO, size=CODE_SIZE)
    el = tbl._element
    el.getparent().remove(el)
    anchor_element.addnext(el)
    return el


def delete_paragraph(p):
    p._element.getparent().remove(p._element)


def delete_range(doc: DocumentCls, from_heading: str, to_heading: str, include_start: bool = True):
    """Удалить параграфы от from_heading (включая/не включая) до to_heading (не включая)."""
    s = find_heading_idx(doc, from_heading)
    if s < 0:
        return 0
    e = find_heading_idx(doc, to_heading, start=s + 1)
    if e < 0:
        e = len(doc.paragraphs)
    first = s if include_start else s + 1
    to_remove = [doc.paragraphs[i]._element for i in range(first, e)]
    for el in to_remove:
        el.getparent().remove(el)
    return len(to_remove)


def delete_body_range_between(start_element, end_element):
    """Удалить все элементы body (параграфы И таблицы) между start_element и end_element,
    не включая сами границы."""
    body = start_element.getparent()
    items = list(body)
    try:
        s_idx = items.index(start_element)
        e_idx = items.index(end_element)
    except ValueError:
        return 0
    if s_idx >= e_idx:
        return 0
    to_remove = items[s_idx + 1:e_idx]
    for el in to_remove:
        body.remove(el)
    return len(to_remove)


def replace_heading_text(doc: DocumentCls, old_text: str, new_text: str) -> bool:
    """Заменить текст заголовка."""
    idx = find_heading_idx(doc, old_text)
    if idx < 0:
        return False
    p = doc.paragraphs[idx]
    # заменим в runs
    full_text = p.text
    new_full = new_text if full_text.strip() == old_text.strip() else full_text.replace(old_text, new_text, 1)
    # пересобираем первый run
    runs = p.runs
    if runs:
        for r in runs[1:]:
            r._element.getparent().remove(r._element)
        runs[0].text = new_full
    return True


def replace_all_text(doc: DocumentCls, pairs: list[tuple[str, str]]):
    def process(p):
        full = "".join(r.text for r in p.runs)
        new_full = full
        for old, new in pairs:
            new_full = new_full.replace(old, new)
        if new_full != full and p.runs:
            for r in p.runs[1:]:
                r._element.getparent().remove(r._element)
            p.runs[0].text = new_full

    for p in doc.paragraphs:
        process(p)
    for t in doc.tables:
        for row in t.rows:
            for cell in row.cells:
                for p in cell.paragraphs:
                    process(p)


# ============================================================
# Применение правок
# ============================================================

def apply_chapter1(doc: DocumentCls):
    """Глава 1: дополняем 1.4, 1.6 обзорными блоками; расширяем 1.9 краткими обзорами."""
    # 1.4 — добавить обзорный блок со сравнением подходов в КОНЕЦ раздела
    append_to_section(doc, "1.4 ", "1.5 ", TEXT_1_4_APPENDIX)

    # 1.6 — добавить обзорный блок в конец
    append_to_section(doc, "1.6 ", "1.7 ", TEXT_1_6_APPENDIX)

    # 1.9 — заменить содержимое на обзор основных требований (без полных таблиц)
    insert_requirements_into_1_9(doc)


def append_to_section(doc: DocumentCls, start_heading: str, next_heading: str,
                      new_paragraphs: list[str]):
    """Добавить абзацы В КОНЕЦ раздела (перед next_heading), сохранив оригинальное содержимое."""
    s = find_heading_idx(doc, start_heading)
    if s < 0:
        print(f"  ! Не найден раздел {start_heading}")
        return
    e = find_heading_idx(doc, next_heading, start=s + 1)
    if e < 0:
        e = len(doc.paragraphs)
    # anchor — последний параграф раздела (перед next_heading)
    anchor = doc.paragraphs[e - 1]._element
    for txt in new_paragraphs:
        anchor = add_body_after(anchor, txt)


def insert_requirements_into_1_9(doc: DocumentCls):
    """В разделе 1.9 делаем краткий обзор основных групп требований.
    Полные таблицы остаются в Приложении Б (не трогаем).
    """
    s = find_heading_idx(doc, "1.9 ")
    if s < 0:
        return
    e = find_heading_idx(doc, "Выводы по главе 1", start=s + 1)
    if e < 0:
        return

    # удалим существующее содержимое раздела (параграфы И таблицы)
    delete_body_range_between(doc.paragraphs[s]._element, doc.paragraphs[e]._element)

    anchor = doc.paragraphs[s]._element

    # вводный абзац
    anchor = add_body_after(anchor, TEXT_1_9_INTRO)

    # 1.9.1 Основные функциональные требования (краткий обзор)
    anchor = add_heading_after(anchor, "1.9.1 Основные функциональные требования", level=3)
    for para in TEXT_1_9_FR_SUMMARY:
        anchor = add_body_after(anchor, para)
    # краткая таблица с ключевыми ФТ
    anchor = add_table_caption_after(anchor,
                                     "Таблица 1.9.1 – Ключевые функциональные требования (обзор)")
    anchor = add_table_after(doc, anchor,
                             headers=["ID", "Требование", "Приоритет"],
                             rows=KEY_FUNCTIONAL_REQS,
                             col_widths_cm=[1.6, 12.6, 2.31],
                             small=True)
    anchor = add_body_after(anchor, TEXT_1_9_FR_LINK)

    # 1.9.2 Основные нефункциональные требования
    anchor = add_heading_after(anchor, "1.9.2 Основные нефункциональные требования", level=3)
    for para in TEXT_1_9_NFR_SUMMARY:
        anchor = add_body_after(anchor, para)
    anchor = add_table_caption_after(anchor,
                                     "Таблица 1.9.2 – Ключевые нефункциональные требования (обзор)")
    anchor = add_table_after(doc, anchor,
                             headers=["ID", "Категория", "Требование", "Целевое значение"],
                             rows=KEY_NON_FUNCTIONAL_REQS,
                             col_widths_cm=[1.3, 3.5, 7.2, 4.51],
                             small=True)
    anchor = add_body_after(anchor, TEXT_1_9_NFR_LINK)


def _find_table_by_header(doc: DocumentCls, header_tuple: tuple) -> Optional[object]:
    """Найти XML-элемент <w:tbl>, у которого первая строка начинается с заданной тройки заголовков."""
    for t in doc.tables:
        if len(t.rows) == 0:
            continue
        cells = [c.text.strip() for c in t.rows[0].cells]
        if len(cells) >= len(header_tuple) and all(
                cells[i].startswith(header_tuple[i]) for i in range(len(header_tuple))):
            return t._element
    return None


def _restyle_table_body(tbl_el):
    """Установить TNR 12pt для содержимого перемещённой таблицы."""
    # пройдём по всем w:r внутри таблицы и установим шрифт/размер
    ns = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
    for r in tbl_el.iter(ns + "r"):
        rPr = r.find(ns + "rPr")
        if rPr is None:
            rPr = OxmlElement("w:rPr")
            r.insert(0, rPr)
        # rFonts
        rFonts = rPr.find(ns + "rFonts")
        if rFonts is None:
            rFonts = OxmlElement("w:rFonts")
            rPr.insert(0, rFonts)
        for a in ("w:ascii", "w:hAnsi", "w:cs", "w:eastAsia"):
            rFonts.set(qn(a), FONT_NAME)
        # sz
        sz = rPr.find(ns + "sz")
        if sz is None:
            sz = OxmlElement("w:sz")
            rPr.append(sz)
        sz.set(qn("w:val"), "24")  # 12pt
        szCs = rPr.find(ns + "szCs")
        if szCs is None:
            szCs = OxmlElement("w:szCs")
            rPr.append(szCs)
        szCs.set(qn("w:val"), "24")


def apply_chapter2(doc: DocumentCls):
    """Глава 2: переструктурирование.

    Порядок действий критичен: сначала переименовываем ВСЕ заголовки (чтобы их новые имена
    использовались как якоря), только потом вызываем операции, которые опираются на диапазоны.
    """
    # Шаг 1. Все переименования (сразу). После этого якоря поиска уже используют новые имена.
    replace_heading_text(doc, "2.3 Пользовательские сценарии",
                         "2.3 Пользовательские истории (user stories)")
    # 2.3.1 Use Case → 2.4 (пока что в Heading3, повысим уровень позже)
    replace_heading_text(doc, "2.3.1 Диаграммы прецедентов (Use Case)",
                         "2.4 Диаграммы прецедентов (Use Case)")
    replace_heading_text(doc, "2.4 Диаграммы активности", "2.6 Диаграммы активности")
    replace_heading_text(doc, "2.5 Архитектура и структура интерфейса",
                         "2.7 Структура приложения (site map)")
    replace_heading_text(doc, "Карта экранов", "2.7.1 Структура приложения")
    replace_heading_text(doc, "Навигационная структура", "2.7.2 Навигация по приложению")
    replace_heading_text(doc, "Рабочий экран анализа", "2.7.3 Основной экран анализа документа")
    replace_heading_text(doc, "2.6 Разработка каркаса (вайрфреймов) лендинга и сервиса",
                         "2.8 Разработка каркаса (вайрфреймов)")
    replace_heading_text(doc, "Вайрфрейм лендинга", "2.8.2 Вайрфрейм лендинга")
    replace_heading_text(doc, "Вайрфрейм основного интерфейса",
                         "2.8.3 Вайрфрейм основного интерфейса")
    replace_heading_text(doc, "2.7 Разработка прототипов и дизайн-макетов",
                         "2.9 Разработка прототипов и дизайн-макетов")
    replace_heading_text(doc, "Цветовая палитра", "2.9.1 Выбор цветовой палитры")
    replace_heading_text(doc, "Целевые разрешения", "2.9.2 Целевые разрешения")
    replace_heading_text(doc, "Интерактивный прототип", "2.9.3 Интерактивный прототип")
    replace_heading_text(doc, "От макета к коду", "2.9.4 От макета к коду")

    # Шаг 2. Повысить/понизить уровни заголовков после переименований.
    # «2.4 Диаграммы прецедентов» был Heading 3 → Heading 2.
    idx = find_heading_idx(doc, "2.4 Диаграммы прецедентов")
    if idx >= 0:
        _set_paragraph_style(doc.paragraphs[idx], "Heading 2", "21")

    # Подзаголовки 2.7.X, 2.8.X, 2.9.X должны быть Heading 3.
    for h3_title in ("2.7.1 Структура приложения",
                     "2.7.2 Навигация по приложению",
                     "2.7.3 Основной экран анализа документа",
                     "2.8.2 Вайрфрейм лендинга",
                     "2.8.3 Вайрфрейм основного интерфейса",
                     "2.9.1 Выбор цветовой палитры",
                     "2.9.2 Целевые разрешения",
                     "2.9.3 Интерактивный прототип",
                     "2.9.4 От макета к коду"):
        idx = find_heading_idx(doc, h3_title)
        if idx >= 0:
            _set_paragraph_style(doc.paragraphs[idx], "Heading 3", "31")

    # Шаг 3. Заменить содержимое 2.3 на user stories (удаляет всё между 2.3 и 2.4).
    rewrite_2_3_user_stories(doc)

    # Шаг 4. Вставить новый раздел 2.5 User flow перед 2.6 (делаем ДО шага 5,
    # чтобы add_usecase_admin_diagram мог найти якорь «2.5 Пользовательские потоки»).
    insert_user_flow_section(doc)

    # Шаг 5. Добавить дополнительные Use Case диаграммы в конец раздела 2.4
    # (перед заголовком 2.5).
    add_usecase_admin_diagram(doc)

    # Шаг 6. Добавить swimlane биллинга в 2.6.
    append_billing_swimlane(doc)

    # Шаг 7. Добавить 2.8.1 Сравнение инструментов проектирования.
    insert_tools_comparison(doc)

    # Шаг 8. Расширить 2.9.1 Выбор цветовой палитры.
    expand_color_palette(doc)


def _set_paragraph_style(p, style_name: str, style_id: str):
    """Установить стиль параграфа по style_id (в XML)."""
    pPr = p._element.find(qn("w:pPr"))
    if pPr is None:
        pPr = OxmlElement("w:pPr")
        p._element.insert(0, pPr)
    pStyle = pPr.find(qn("w:pStyle"))
    if pStyle is None:
        pStyle = OxmlElement("w:pStyle")
        pPr.insert(0, pStyle)
    pStyle.set(qn("w:val"), style_id)


def rewrite_2_3_user_stories(doc: DocumentCls):
    """Заменить содержимое 2.3 (старые «Сценарии») на user stories по ролям."""
    s = find_heading_idx(doc, "2.3 Пользовательские истории")
    if s < 0:
        return
    # следующий заголовок — 2.4 (уже переименовано из 2.3.1)
    e = find_heading_idx(doc, "2.4 Диаграммы прецедентов", start=s + 1)
    if e < 0:
        e = len(doc.paragraphs)
    to_remove = [doc.paragraphs[i]._element for i in range(s + 1, e)]
    for el in to_remove:
        el.getparent().remove(el)

    anchor = doc.paragraphs[s]._element
    anchor = add_body_after(anchor, TEXT_2_3_INTRO)

    # Пользователь
    anchor = add_heading_after(anchor, "2.3.1 Истории пользователя (клиента сервиса)", level=3)
    for story in STORIES_USER:
        anchor = add_body_after(anchor, story)

    # Администратор
    anchor = add_heading_after(anchor, "2.3.2 Истории администратора", level=3)
    for story in STORIES_ADMIN:
        anchor = add_body_after(anchor, story)

    # Менеджер
    anchor = add_heading_after(anchor, "2.3.3 Истории менеджера поддержки", level=3)
    for story in STORIES_MANAGER:
        anchor = add_body_after(anchor, story)

    # Биллинг (системные сценарии)
    anchor = add_heading_after(anchor, "2.3.4 Сценарии подсистемы биллинга", level=3)
    for story in STORIES_BILLING:
        anchor = add_body_after(anchor, story)

    anchor = add_body_after(anchor, TEXT_2_3_OUTRO)

    # User Story Mapping
    anchor = add_heading_after(anchor, "2.3.5 Карта пользовательских историй (user story mapping)", level=3)
    anchor = add_body_after(anchor, TEXT_2_3_MAP_INTRO)
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.3.5_user_story_map.png", width_cm=16)
    anchor = add_fig_caption_after(anchor,
                                   "Рисунок 2.3.5 – Карта пользовательских историй сервиса Dockee")
    anchor = add_body_after(anchor, TEXT_2_3_MAP_OUTRO)


def add_usecase_admin_diagram(doc: DocumentCls):
    """Добавить дополнительные Use Case-диаграммы в конец раздела 2.4 (перед 2.5)."""
    # якорь — заголовок 2.5; вставляем перед ним
    next_idx = find_heading_idx(doc, "2.5 Пользовательские потоки")
    if next_idx < 0:
        return
    anchor = doc.paragraphs[next_idx - 1]._element if next_idx > 0 else doc.paragraphs[next_idx]._element

    # 2.4.1 — Use Case «Администрирование и биллинг»
    anchor = add_body_after(anchor, TEXT_UC_ADMIN)
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.13_usecase_admin.png", width_cm=15)
    anchor = add_fig_caption_after(anchor,
        "Рисунок 2.4.4 – Диаграмма прецедентов «Администрирование»")
    anchor = add_body_after(anchor, TEXT_UC_ADMIN_OUTRO)

    # 2.4.5 — Use Case «Менеджер поддержки»
    anchor = add_body_after(anchor, TEXT_UC_MANAGER)
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.13b_usecase_manager.png", width_cm=14)
    anchor = add_fig_caption_after(anchor,
        "Рисунок 2.4.5 – Диаграмма прецедентов «Менеджер поддержки»")
    anchor = add_body_after(anchor, TEXT_UC_MANAGER_OUTRO)

    # 2.4.6 — Use Case «Биллинг»
    anchor = add_body_after(anchor, TEXT_UC_BILLING)
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.13c_usecase_billing.png", width_cm=14)
    anchor = add_fig_caption_after(anchor,
        "Рисунок 2.4.6 – Диаграмма прецедентов «Биллинг»")
    anchor = add_body_after(anchor, TEXT_UC_BILLING_OUTRO)

    # 2.4.7 — Сводная Use Case «Все акторы системы»
    anchor = add_body_after(anchor, TEXT_UC_OVERVIEW)
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.13d_usecase_overview.png", width_cm=16)
    anchor = add_fig_caption_after(anchor,
        "Рисунок 2.4.7 – Сводная диаграмма прецедентов сервиса Dockee")
    anchor = add_body_after(anchor, TEXT_UC_OVERVIEW_OUTRO)


def insert_user_flow_section(doc: DocumentCls):
    """Вставить раздел 2.5 User flow перед 2.6 Диаграммы активности."""
    idx = find_heading_idx(doc, "2.6 Диаграммы активности")
    if idx < 0:
        return
    anchor_el = doc.paragraphs[idx]._element
    # добавляем ПЕРЕД 2.6 — значит через addprevious
    # но мы добавляем последовательно, значит создадим параграфы и вставим addprevious
    prev = anchor_el

    # Заголовок 2.5
    h = _make_paragraph("2.5 Пользовательские потоки (user flow)", style="Heading2",
                        align=WD_ALIGN_PARAGRAPH.JUSTIFY, bold=True, size=Pt(16))
    prev.addprevious(h)

    # Интро
    p = _make_paragraph(TEXT_2_5_INTRO, style="Normal", align=WD_ALIGN_PARAGRAPH.JUSTIFY,
                        first_line_indent=Cm(1.25))
    prev.addprevious(p)

    # 3 user flow диаграммы
    for (intro, img_name, caption, outro) in USER_FLOWS:
        p = _make_paragraph(intro, style="Normal", align=WD_ALIGN_PARAGRAPH.JUSTIFY,
                            first_line_indent=Cm(1.25))
        prev.addprevious(p)
        # рисунок
        tmp = doc.add_paragraph()
        tmp.alignment = WD_ALIGN_PARAGRAPH.CENTER
        run = tmp.add_run()
        img_path = IMAGES_DIR / img_name
        if img_path.exists():
            run.add_picture(str(img_path), width=Cm(15))
        else:
            run.add_text(f"[Изображение: {img_name}]")
            _set_run_font(run, italic=True, size=TABLE_SIZE)
        img_el = tmp._element
        img_el.getparent().remove(img_el)
        prev.addprevious(img_el)
        # подпись
        cap = _make_paragraph(caption, style="Normal", align=WD_ALIGN_PARAGRAPH.CENTER,
                              bold=True, size=TABLE_SIZE)
        prev.addprevious(cap)
        # послерисуночный абзац
        p = _make_paragraph(outro, style="Normal", align=WD_ALIGN_PARAGRAPH.JUSTIFY,
                            first_line_indent=Cm(1.25))
        prev.addprevious(p)


def append_billing_swimlane(doc: DocumentCls):
    """Добавить в 2.6 Диаграммы активности swimlane биллинга."""
    # найти последнюю подпись рисунка в разделе (либо текст про существующую диаграмму активности)
    idx = find_heading_idx(doc, "2.6 Диаграммы активности")
    if idx < 0:
        return
    next_h = find_heading_idx(doc, "2.7 Структура приложения", start=idx + 1)
    if next_h < 0:
        next_h = find_heading_idx(doc, "2.7", start=idx + 1)
    if next_h < 0:
        return
    # anchor — последний параграф раздела 2.6 (перед 2.7)
    anchor = doc.paragraphs[next_h - 1]._element

    # вводный абзац
    anchor = add_body_after(anchor, TEXT_2_6_BILLING_INTRO)
    # рисунок
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.12_activity_billing_swimlane.png",
                               width_cm=16)
    anchor = add_fig_caption_after(anchor,
                                   "Рисунок 2.8 -- Диаграмма активности «Обработка платежа» (swimlane)")
    anchor = add_body_after(anchor, TEXT_2_6_BILLING_OUTRO)


def insert_tools_comparison(doc: DocumentCls):
    """Вставить 2.8.1 сразу после заголовка 2.8, перед 2.8.2."""
    idx = find_heading_idx(doc, "2.8 Разработка каркаса")
    if idx < 0:
        return
    next_h = find_heading_idx(doc, "2.8.2 Вайрфрейм лендинга", start=idx + 1)
    if next_h < 0:
        return

    # удалить параграфы И таблицы между 2.8 и 2.8.2
    delete_body_range_between(doc.paragraphs[idx]._element, doc.paragraphs[next_h]._element)

    anchor = doc.paragraphs[idx]._element

    # 2.8.1 — сравнение Figma / Adobe Illustrator / XDesign
    anchor = add_heading_after(anchor, "2.8.1 Сравнение инструментов проектирования", level=3)
    anchor = add_body_after(anchor, TEXT_2_8_1_INTRO)
    anchor = add_table_caption_after(anchor,
                                     "Таблица 2.8.1 – Сравнение инструментов проектирования вайрфреймов")
    anchor = add_table_after(doc, anchor,
                             headers=["Критерий", "Figma", "Adobe Illustrator", "XDesign"],
                             rows=TOOLS_COMPARISON,
                             col_widths_cm=[4.13, 4.13, 4.13, 4.12],
                             small=True)
    anchor = add_body_after(anchor, TEXT_2_8_1_OUTRO)


def expand_color_palette(doc: DocumentCls):
    """Расширить раздел 2.9.1 — добавить сравнение трёх палитр."""
    s = find_heading_idx(doc, "2.9.1 Выбор цветовой палитры")
    if s < 0:
        return
    e = find_heading_idx(doc, "2.9.2 Целевые разрешения", start=s + 1)
    if e < 0:
        e = len(doc.paragraphs)

    # удалим старое содержимое раздела (оставим заголовок) — собираем сразу
    to_remove = [doc.paragraphs[i]._element for i in range(s + 1, e)]
    for el in to_remove:
        el.getparent().remove(el)

    anchor = doc.paragraphs[s]._element
    # описание задачи и критериев идёт ПЕРЕД изображением
    anchor = add_body_after(anchor, TEXT_2_9_1_INTRO)
    anchor = add_body_after(anchor, TEXT_2_9_1_COMPARE_INTRO)
    # затем сравнительная таблица
    anchor = add_table_caption_after(anchor,
                                     "Таблица 2.9.1 – Критерии выбора цветовой палитры")
    anchor = add_table_after(doc, anchor,
                             headers=["Критерий", "А — Сдержанная\nс синим акцентом",
                                      "Б — Юридическая\nзелёная", "В — Нейтральная\nтёмная"],
                             rows=PALETTE_COMPARE,
                             col_widths_cm=[4.13, 4.13, 4.13, 4.12],
                             small=True)
    # после таблицы — визуализация, затем подпись, затем выбор
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.15_color_palettes.png", width_cm=16)
    anchor = add_fig_caption_after(anchor,
                                   "Рисунок 2.9.1 – Сравнение вариантов цветовой палитры")
    anchor = add_body_after(anchor, TEXT_2_9_1_CHOICE)


def apply_chapter3(doc: DocumentCls):
    """Глава 3: краткие листинги в разделы."""
    # В 3.1 добавить краткий листинг (Vue-компонент карточки риска)
    idx = find_heading_idx(doc, "3.1 Разработка пользовательского интерфейса лендинга")
    if idx >= 0:
        next_h = find_heading_idx(doc, "3.2 ", start=idx + 1)
        anchor = doc.paragraphs[next_h - 1]._element if next_h > 0 else doc.paragraphs[idx]._element
        anchor = add_body_after(anchor, TEXT_3_1_LISTING_INTRO)
        anchor = add_listing_caption_after(anchor,
                                           "Листинг 3.1 -- Фрагмент Vue-компонента секции лендинга")
        anchor = add_code_block_after(doc, anchor, LISTING_3_1)
        anchor = add_body_after(anchor, TEXT_3_1_LISTING_OUTRO)

    # В 3.2 добавить краткий листинг компонента карточки риска
    idx = find_heading_idx(doc, "3.2 Разработка пользовательского интерфейса сервиса")
    if idx >= 0:
        next_h = find_heading_idx(doc, "3.3 ", start=idx + 1)
        anchor = doc.paragraphs[next_h - 1]._element if next_h > 0 else doc.paragraphs[idx]._element
        anchor = add_body_after(anchor, TEXT_3_2_LISTING_INTRO)
        anchor = add_listing_caption_after(anchor,
                                           "Листинг 3.2 -- Компонент карточки риска (Vue 3, Composition API)")
        anchor = add_code_block_after(doc, anchor, LISTING_3_2)
        anchor = add_body_after(anchor, TEXT_3_2_LISTING_OUTRO)

    # В 3.3 — фрагмент интеграции с API
    idx = find_heading_idx(doc, "3.3 Интеграция с серверной частью сервиса")
    if idx >= 0:
        next_h = find_heading_idx(doc, "3.4 ", start=idx + 1)
        anchor = doc.paragraphs[next_h - 1]._element if next_h > 0 else doc.paragraphs[idx]._element
        anchor = add_body_after(anchor, TEXT_3_3_LISTING_INTRO)
        anchor = add_listing_caption_after(anchor,
                                           "Листинг 3.3 -- Pinia-store для работы с анализом документов")
        anchor = add_code_block_after(doc, anchor, LISTING_3_3)
        anchor = add_body_after(anchor, TEXT_3_3_LISTING_OUTRO)

    # В 3.4 — CSP-заголовки и XSS-защита
    idx = find_heading_idx(doc, "3.4 Обеспечение безопасности")
    if idx >= 0:
        next_h = find_heading_idx(doc, "3.5 ", start=idx + 1)
        anchor = doc.paragraphs[next_h - 1]._element if next_h > 0 else doc.paragraphs[idx]._element
        anchor = add_body_after(anchor, TEXT_3_4_LISTING_INTRO)
        anchor = add_listing_caption_after(anchor,
                                           "Листинг 3.4 -- Настройка Content Security Policy и санитизация ввода")
        anchor = add_code_block_after(doc, anchor, LISTING_3_4)
        anchor = add_body_after(anchor, TEXT_3_4_LISTING_OUTRO)


def apply_appendices(doc: DocumentCls):
    """Приложения: дополнить перечень, дополнить Приложение А, почистить Приложение В, добавить Приложение Д."""
    # Обновить перечень приложений на первой странице раздела ПРИЛОЖЕНИЯ
    add_appendix_list_entry(doc)

    # Дополнить Приложение А новыми диаграммами
    extend_appendix_a(doc)

    # Приложение Б оставляем в исходном виде (Б.1, Б.2, Б.3 + полные таблицы)

    # Почистить Приложение В (убрать «пустые» листинги В.1-В.4, заполнить содержимым)
    cleanup_appendix_v(doc)

    # Добавить Приложение Д в конец
    add_appendix_d(doc)


def extend_appendix_a(doc: DocumentCls):
    """Добавить в Приложение А дополнительные диаграммы: активность «Работа с заметками»
    и sequence-диаграмму процесса анализа."""
    # anchor — последний параграф Приложения А (перед Приложением Б)
    idx_a = find_heading_idx(doc, "Приложение А")
    if idx_a < 0:
        return
    idx_b = find_heading_idx(doc, "Приложение Б", start=idx_a + 1)
    if idx_b < 0:
        return
    anchor = doc.paragraphs[idx_b - 1]._element

    # А.5 — работа с заметками
    anchor = add_body_after(anchor,
        "Диаграмма активности «Работа с заметками и статусами замечаний» показывает, как "
        "пользователь итеративно проходит список замечаний, добавляет заметки, изменяет статусы.")
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.16_activity_notes.png", width_cm=15)
    anchor = add_fig_caption_after(anchor,
        "Рисунок А.5 – Диаграмма активности «Работа с заметками и статусами»")

    # А.6 — sequence-диаграмма процесса анализа
    anchor = add_body_after(anchor,
        "Диаграмма последовательности процесса анализа документа демонстрирует взаимодействие "
        "фронтенда, API-сервиса, файлового хранилища и внешнего ИИ-сервиса во времени.")
    anchor = add_picture_after(doc, anchor, IMAGES_DIR / "2.17_sequence_analyze.png", width_cm=15)
    anchor = add_fig_caption_after(anchor,
        "Рисунок А.6 – Диаграмма последовательности «Анализ документа»")


def cleanup_appendix_b(doc: DocumentCls):
    """Удалить заголовки Б.1 и Б.2 и их подписи; оставить только Б.3 UX-требования."""
    # ищем заголовок «Приложение Б» → следующий Heading 2 «Приложение В»
    idx_b = find_heading_idx(doc, "Приложение Б")
    if idx_b < 0:
        return
    idx_v = find_heading_idx(doc, "Приложение В", start=idx_b + 1)
    if idx_v < 0:
        idx_v = len(doc.paragraphs)

    # собрать индексы параграфов в зоне Б, которые относятся к Б.1 и Б.2
    to_remove = []
    inside_b12 = False
    for i in range(idx_b + 1, idx_v):
        p = doc.paragraphs[i]
        txt = p.text.strip()
        if p.style.name == "Heading 2" and (txt.startswith("Б.1") or txt.startswith("Б.2")):
            inside_b12 = True
            to_remove.append(p._element)
            continue
        if p.style.name == "Heading 2" and txt.startswith("Б.3"):
            inside_b12 = False
            continue
        if inside_b12:
            to_remove.append(p._element)
    for el in to_remove:
        el.getparent().remove(el)


def cleanup_appendix_v(doc: DocumentCls):
    """Убрать дубликаты Приложения В. Оставить единый блок с полными листингами В.1-В.8."""
    idx_v = find_heading_idx(doc, "Приложение В")
    if idx_v < 0:
        return
    # граница — Приложение Г
    idx_g = find_heading_idx(doc, "Приложение Г", start=idx_v + 1)
    if idx_g < 0:
        idx_g = len(doc.paragraphs)

    # удалить ВСЁ между заголовком «Приложение В» и «Приложение Г»
    to_remove = [doc.paragraphs[i]._element for i in range(idx_v + 1, idx_g)]
    for el in to_remove:
        el.getparent().remove(el)

    # вставить новый структурированный список листингов
    anchor = doc.paragraphs[idx_v]._element
    anchor = add_body_after(anchor, "Листинги кода пользовательского интерфейса")

    # В.1 — Vue-компонент лендинга (полный)
    anchor = add_listing_caption_after(anchor, "Листинг В.1 -- Полный Vue-компонент секции «Как работает» лендинга")
    anchor = add_code_block_after(doc, anchor, LISTING_V_1)

    # В.2 — Карточка риска
    anchor = add_listing_caption_after(anchor, "Листинг В.2 -- Компонент карточки риска с полной разметкой и стилями")
    anchor = add_code_block_after(doc, anchor, LISTING_V_2)

    # В.3 — Pinia-store
    anchor = add_listing_caption_after(anchor, "Листинг В.3 -- Pinia-store для работы с документами и анализом (полная версия)")
    anchor = add_code_block_after(doc, anchor, LISTING_V_3)

    # В.4 — CSP и санитизация
    anchor = add_listing_caption_after(anchor, "Листинг В.4 -- Настройка CSP, санитизация и защита от XSS")
    anchor = add_code_block_after(doc, anchor, LISTING_V_4)

    # В.5 — Функция экранирования HTML (оставили из оригинала)
    anchor = add_listing_caption_after(anchor, "Листинг В.5 -- Функция экранирования HTML")
    anchor = add_code_block_after(doc, anchor, LISTING_V_5)

    # В.6 — Обёртка localStorage
    anchor = add_listing_caption_after(anchor, "Листинг В.6 -- Типизированная обёртка над localStorage")
    anchor = add_code_block_after(doc, anchor, LISTING_V_6)

    # В.7 — Ленивая загрузка маршрутов
    anchor = add_listing_caption_after(anchor, "Листинг В.7 -- Ленивая загрузка маршрутов Vue Router")
    anchor = add_code_block_after(doc, anchor, LISTING_V_7)

    # В.8 — Настройка Vite
    anchor = add_listing_caption_after(anchor, "Листинг В.8 -- Конфигурация Vite для production-сборки")
    anchor = add_code_block_after(doc, anchor, LISTING_V_8)


def add_appendix_list_entry(doc: DocumentCls):
    """Добавить строку «Приложение Д – Руководство пользователя и инструкция по эксплуатации»."""
    idx = find_para_idx(doc, "Приложение Г – Экраны интерфейса сервиса Dockee")
    if idx < 0:
        return
    anchor = doc.paragraphs[idx]._element
    anchor = add_body_after(anchor,
                            "Приложение Д – Руководство пользователя и инструкция по эксплуатации",
                            first_line_indent=Cm(0))


def add_appendix_d(doc: DocumentCls):
    """В конец документа добавить Приложение Д с 4 разделами."""
    # последний параграф документа
    body = doc.paragraphs[-1]._element.getparent()
    # работаем в конце
    last = doc.paragraphs[-1]._element

    # Заголовок «Приложение Д» (Heading 2 для единообразия с А/Б/В/Г этого документа)
    h = _make_paragraph("Приложение Д", style="Heading 2", align=WD_ALIGN_PARAGRAPH.CENTER,
                        bold=True, size=Pt(16))
    last.addnext(h)
    last = h

    # Название приложения (основной текст, по центру)
    p = _make_paragraph("Руководство пользователя и инструкция по эксплуатации",
                        style="Normal", align=WD_ALIGN_PARAGRAPH.CENTER)
    last.addnext(p)
    last = p

    # Д.1
    last = add_heading_after(last, "Д.1 Руководство пользователя", level=2)
    for para in APPENDIX_D_USER_GUIDE:
        last = add_body_after(last, para)

    # Д.2
    last = add_heading_after(last, "Д.2 Руководство администратора", level=2)
    for para in APPENDIX_D_ADMIN_GUIDE:
        last = add_body_after(last, para)

    # Д.3
    last = add_heading_after(last, "Д.3 Инструкция по развёртыванию", level=2)
    for para in APPENDIX_D_DEPLOY[:2]:
        last = add_body_after(last, para)
    last = add_listing_caption_after(last, "Листинг Д.1 -- docker-compose.yml для развёртывания сервиса")
    last = add_code_block_after(doc, last, APPENDIX_D_DOCKER_COMPOSE)
    for para in APPENDIX_D_DEPLOY[2:]:
        last = add_body_after(last, para)

    # Д.4
    last = add_heading_after(last, "Д.4 Инструкция по сопровождению", level=2)
    for para in APPENDIX_D_MAINTAIN:
        last = add_body_after(last, para)


# ============================================================
# Содержимое правок (большие тексты вынесены вниз)
# ============================================================

from edits_content import (
    TEXT_1_4_APPENDIX, TEXT_1_6_APPENDIX,
    TEXT_1_9_INTRO,
    TEXT_1_9_FR_SUMMARY, TEXT_1_9_FR_LINK, KEY_FUNCTIONAL_REQS,
    TEXT_1_9_NFR_SUMMARY, TEXT_1_9_NFR_LINK, KEY_NON_FUNCTIONAL_REQS,
    TEXT_2_3_INTRO, TEXT_2_3_OUTRO, STORIES_USER, STORIES_ADMIN, STORIES_MANAGER, STORIES_BILLING,
    TEXT_2_3_MAP_INTRO, TEXT_2_3_MAP_OUTRO,
    TEXT_UC_ADMIN, TEXT_UC_ADMIN_OUTRO,
    TEXT_UC_MANAGER, TEXT_UC_MANAGER_OUTRO,
    TEXT_UC_BILLING, TEXT_UC_BILLING_OUTRO,
    TEXT_UC_OVERVIEW, TEXT_UC_OVERVIEW_OUTRO,
    TEXT_2_5_INTRO, USER_FLOWS,
    TEXT_2_6_BILLING_INTRO, TEXT_2_6_BILLING_OUTRO,
    TEXT_2_8_1_INTRO, TEXT_2_8_1_OUTRO, TOOLS_COMPARISON,
    TEXT_2_9_1_INTRO, TEXT_2_9_1_COMPARE_INTRO, TEXT_2_9_1_CHOICE, PALETTE_COMPARE,
    TEXT_3_1_LISTING_INTRO, TEXT_3_1_LISTING_OUTRO, LISTING_3_1,
    TEXT_3_2_LISTING_INTRO, TEXT_3_2_LISTING_OUTRO, LISTING_3_2,
    TEXT_3_3_LISTING_INTRO, TEXT_3_3_LISTING_OUTRO, LISTING_3_3,
    TEXT_3_4_LISTING_INTRO, TEXT_3_4_LISTING_OUTRO, LISTING_3_4,
    LISTING_V_1, LISTING_V_2, LISTING_V_3, LISTING_V_4, LISTING_V_5, LISTING_V_6, LISTING_V_7, LISTING_V_8,
    APPENDIX_D_USER_GUIDE, APPENDIX_D_ADMIN_GUIDE, APPENDIX_D_DEPLOY, APPENDIX_D_DOCKER_COMPOSE,
    APPENDIX_D_MAINTAIN,
)


# ============================================================
# Main
# ============================================================

def main():
    if not SRC_DOCX.exists():
        sys.exit(f"Исходный файл не найден: {SRC_DOCX}")

    doc = Document(str(SRC_DOCX))
    print("[1/6] Глава 1: 1.4, 1.6, 1.9...")
    apply_chapter1(doc)
    print("[2/6] Глава 2: переструктурирование, новые разделы...")
    apply_chapter2(doc)
    print("[3/6] Глава 3: краткие листинги...")
    apply_chapter3(doc)
    print("[4/6] Приложение Д и обновление списка приложений...")
    apply_appendices(doc)
    print("[5/6] Замена нестандартных терминов и фикс нумерации...")
    replace_all_text(doc, [
        # фикс «документа документа» после предыдущих замен
        ("анализа документа документа", "анализа документа"),
        # фикс конфликтов нумерации моих вставок с оригиналом
        ("Таблица 1.1 -- Функциональные", "Таблица 1.9.1 -- Функциональные"),
        ("Таблица 1.2 -- Нефункциональные", "Таблица 1.9.2 -- Нефункциональные"),
        # Таблицы 2.8.1 и 2.9.1 теперь создаются сразу с правильным номером и тире,
        # поэтому отдельные замены не нужны — оставлены ниже для старых подписей.
        ("Таблица 2.2 -- Сравнение инструментов", "Таблица 2.8.1 – Сравнение инструментов"),
        ("Таблица 2.3 -- Критерии выбора цветовой", "Таблица 2.9.1 – Критерии выбора цветовой"),
        ("Таблица 1.1 охватывает 33 функциональных требования",
         "Таблица 1.9.1 охватывает 33 функциональных требования"),
        ("таблицы 1.1 и 1.2", "таблицы 1.9.1 и 1.9.2"),
        ("Рисунок 2.4 -- Диаграмма прецедентов «Администрирование",
         "Рисунок 2.3.4 -- Диаграмма прецедентов «Администрирование"),
        ("Рисунок 2.5 – Пользовательский поток «Регистрация",
         "Рисунок 2.5.1 – Пользовательский поток «Регистрация"),
        ("Рисунок 2.6 – Пользовательский поток «Загрузка",
         "Рисунок 2.5.2 – Пользовательский поток «Загрузка"),
        ("Рисунок 2.7 – Пользовательский поток «Работа с результатами",
         "Рисунок 2.5.3 – Пользовательский поток «Работа с результатами"),
        ("Рисунок 2.8 – Диаграмма активности «Обработка платежа»",
         "Рисунок 2.6.1 – Диаграмма активности «Обработка платежа»"),
        ("Рисунок 2.11 – Сравнение вариантов цветовой палитры",
         "Рисунок 2.9.1 – Сравнение вариантов цветовой палитры"),
        # финальные замены двойного тире
        ("Рисунок 2.5 -- ", "Рисунок 2.5.1 – "),
        ("Рисунок 2.6 -- ", "Рисунок 2.5.2 – "),
        ("Рисунок 2.7 -- ", "Рисунок 2.5.3 – "),
        (" -- ", " – "),
        ("Карты экранов", "Структуры приложения"),
        ("Карте экранов", "Структуре приложения"),
        ("Карту экранов", "Структуру приложения"),
        ("Карта экранов", "Структура приложения"),
        ("карты экранов", "структуры приложения"),
        ("карте экранов", "структуре приложения"),
        ("карту экранов", "структуру приложения"),
        ("карта экранов", "структура приложения"),
        ("Навигационной структуре", "Навигации по приложению"),
        ("Навигационной структуры", "Навигации по приложению"),
        ("Навигационную структуру", "Навигацию по приложению"),
        ("Навигационная структура", "Навигация по приложению"),
        ("навигационной структуре", "навигации по приложению"),
        ("навигационной структуры", "навигации по приложению"),
        ("навигационную структуру", "навигацию по приложению"),
        ("навигационная структура", "навигация по приложению"),
        ("Рабочем экране анализа", "Основном экране анализа документа"),
        ("Рабочего экрана анализа", "Основного экрана анализа документа"),
        ("Рабочий экран анализа", "Основной экран анализа документа"),
        ("рабочем экране анализа", "основном экране анализа документа"),
        ("рабочего экрана анализа", "основного экрана анализа документа"),
        ("рабочий экран анализа", "основной экран анализа документа"),
    ])
    # финальный проход: убрать дублирующиеся слова, возникшие из-за последовательных замен
    replace_all_text(doc, [
        ("анализа документа документа", "анализа документа"),
        ("документа документа", "документа"),
    ])
    print("[6/6] Сохранение...")
    doc.save(str(OUT_DOCX))
    print(f"Готово: {OUT_DOCX}")


if __name__ == "__main__":
    sys.path.insert(0, str(Path(__file__).parent))
    main()
