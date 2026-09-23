#!/usr/bin/env python3
"""
Сборка краткой .docx-версии ВКР: Введение → Главы 1–3 → Заключение.
Без аннотации, оглавления, списка сокращений, источников и приложений.

Используется тот же конвертер из build-docx.py, меняются только INPUT_FILES,
OUTPUT_PATH и пропускается вставка автособираемого оглавления.
"""
from pathlib import Path

import build_docx_module  # импорт основного билдера через alias-модуль


# Подмена входных файлов и выходного пути
SHORT_INPUT_FILES = [
    build_docx_module.BASE_DIR / "chapters" / "introduction.md",
    build_docx_module.BASE_DIR / "chapters" / "chapter1.md",
    build_docx_module.BASE_DIR / "chapters" / "chapter2.md",
    build_docx_module.BASE_DIR / "chapters" / "chapter3.md",
    build_docx_module.BASE_DIR / "chapters" / "conclusion.md",
]

SHORT_OUTPUT = (
    build_docx_module.BASE_DIR
    / "output"
    / "vkr_nikita_short.docx"
)


def build_short():
    from docx import Document
    doc = Document()

    build_docx_module.setup_styles(doc)
    build_docx_module.setup_page(doc)

    converter = build_docx_module.MarkdownToDocx(doc)
    for fp in SHORT_INPUT_FILES:
        print(f"  Обработка: {fp.name}")
        converter.process_file(fp)

    SHORT_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    doc.save(str(SHORT_OUTPUT))

    size_kb = SHORT_OUTPUT.stat().st_size / 1024
    total_chars = 0
    for fp in SHORT_INPUT_FILES:
        with open(fp, encoding="utf-8") as f:
            total_chars += len(f.read())
    estimated_pages = max(1, total_chars // 1400)

    print()
    print(f"  Файл:    {SHORT_OUTPUT}")
    print(f"  Размер:  {size_kb:.1f} КБ")
    print(f"  Страниц: ~{estimated_pages} (оценка)")


if __name__ == "__main__":
    print()
    print("Сборка краткой ВКР (Введение → Заключение)...")
    print()
    missing = [f for f in SHORT_INPUT_FILES if not f.exists()]
    if missing:
        print("ОШИБКА: не найдены файлы:")
        for f in missing:
            print(f"  {f}")
        raise SystemExit(1)
    build_short()
    print("  Готово.")
