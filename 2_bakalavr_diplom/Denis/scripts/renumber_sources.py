"""Перенумерация источников в references.md по первому упоминанию в тексте.

Алгоритм:
1. Читаем главы в фиксированном порядке (annotation → introduction → chapter1..4 → conclusion).
2. Сканируем каждый файл, находим все [N] в порядке появления.
3. Строим маппинг old_N → new_N (1..k) по первому появлению.
4. Для всех N, не упомянутых в главах, — удаляем из references.md.
5. Переписываем references.md в новом порядке, с новыми номерами.
6. Обновляем все главы: заменяем [old_N] → [new_N].
"""
from __future__ import annotations

import re
from pathlib import Path

BASE = Path("/home/washka/project/diplom/Denis")
CHAPTERS_DIR = BASE / "chapters"
REF_PATH = BASE / "sources" / "references.md"

# Порядок сканирования = порядок появления в финальном документе
SCAN_ORDER = [
    "annotation.md",
    "introduction.md",
    "chapter1.md",
    "chapter2.md",
    "chapter3.md",
    "chapter4.md",
    "conclusion.md",
    "appendices.md",
]

REF_ENTRY_RE = re.compile(r"^\*\*(\d+)\.\*\*\s*(.*)$", re.MULTILINE)
# Ссылки в тексте: [N], [N, с. X], [N, X, Y]. Берём все комбинированные: [1], [2, 3], [4, 14]
CITATION_RE = re.compile(r"\[(\d+(?:\s*,\s*\d+)*(?:\s*,\s*с\.?\s*[\d\-]+)?)\]")
SINGLE_N_RE = re.compile(r"(\d+)")


def parse_references(text: str) -> dict[int, tuple[str, str]]:
    """Возвращает dict: old_num → (header_line, комментарий-блок + подзаголовки)."""
    result: dict[int, tuple[str, str]] = {}
    # Бьём на блоки — каждый блок начинается с **N.**
    entries = list(REF_ENTRY_RE.finditer(text))
    for i, m in enumerate(entries):
        num = int(m.group(1))
        header_raw = m.group(2).strip()
        start = m.start()
        end = entries[i + 1].start() if i + 1 < len(entries) else len(text)
        block = text[start:end].rstrip() + "\n"
        # Комментарий под записью (> *Используется ...*)
        result[num] = (header_raw, block)
    return result


def find_first_mentions() -> list[int]:
    """Возвращает список old_N в порядке первого появления в главах."""
    seen: list[int] = []
    seen_set: set[int] = set()
    for fname in SCAN_ORDER:
        fpath = CHAPTERS_DIR / fname
        if not fpath.exists():
            continue
        text = fpath.read_text(encoding="utf-8")
        for match in CITATION_RE.finditer(text):
            content = match.group(1)
            # В скобках может быть несколько источников: "1, 4, 7"
            # Берём только цифры, отсекаем "с. 42"
            before_page = content.split(",с.")[0].split(",с")[0]
            before_page = re.sub(r",\s*с\.?\s*[\d\-]+", "", before_page)
            for n_str in before_page.split(","):
                n_str = n_str.strip()
                if n_str.isdigit():
                    n = int(n_str)
                    if n not in seen_set:
                        seen_set.add(n)
                        seen.append(n)
    return seen


def rewrite_references(used: list[int], refs: dict[int, tuple[str, str]]) -> str:
    """Строит новый text references.md с перенумерацией."""
    header = [
        "# Список использованных источников",
        "",
        "> Оформлен по ГОСТ Р 7.0.5-2008. Сортировка: **по порядку упоминания в тексте**.",
        "> Все записи используются как минимум в одной главе. Источники с пометкой [ПРОВЕРИТЬ]",
        "> требуют верификации ISBN / DOI / страниц перед сдачей на кафедру.",
        "",
        "---",
        "",
    ]
    body: list[str] = []
    for new_n, old_n in enumerate(used, start=1):
        if old_n not in refs:
            body.append(f"**{new_n}.** [ИСТОЧНИК {old_n} ОТСУТСТВУЕТ В references.md — ВОССТАНОВИТЬ]\n")
            continue
        header_raw, block = refs[old_n]
        # Заменяем старый номер на новый
        new_block = re.sub(
            rf"^\*\*{old_n}\.\*\*",
            f"**{new_n}.**",
            block,
            count=1,
            flags=re.MULTILINE,
        )
        body.append(new_block)
    return "\n".join(header) + "\n".join(body)


def renumber_chapter(text: str, mapping: dict[int, int]) -> str:
    """Заменяет все [old_N] на [new_N] в тексте главы."""

    def replace_match(m: re.Match) -> str:
        content = m.group(1)
        # Разбить "1, 4, с. 42" на части
        # Отделяем "с. XXX" часть, если есть
        page_match = re.search(r",\s*с\.?\s*([\d\-]+)\s*$", content)
        page_suffix = ""
        if page_match:
            page_suffix = f", с. {page_match.group(1)}"
            content = content[: page_match.start()]
        nums = []
        for n_str in content.split(","):
            n_str = n_str.strip()
            if n_str.isdigit():
                old = int(n_str)
                new = mapping.get(old)
                if new is None:
                    nums.append(f"?{old}?")
                else:
                    nums.append(str(new))
            else:
                nums.append(n_str)
        return "[" + ", ".join(nums) + page_suffix + "]"

    return CITATION_RE.sub(replace_match, text)


def main() -> None:
    ref_text = REF_PATH.read_text(encoding="utf-8")
    refs = parse_references(ref_text)
    print(f"Найдено записей в references.md: {len(refs)}")

    used = find_first_mentions()
    print(f"Уникальных [N] в главах: {len(used)}")
    print("Порядок первого упоминания:", used[:15], "..." if len(used) > 15 else "")

    # Mapping для замены
    mapping = {old: new for new, old in enumerate(used, start=1)}
    print(f"Mapping old→new: {sorted(mapping.items())[:10]}...")

    # Перенумеровать references.md
    new_ref_text = rewrite_references(used, refs)
    REF_PATH.write_text(new_ref_text, encoding="utf-8")
    print(f"✅ references.md обновлён: {len(used)} записей (было {len(refs)})")

    # Переписать главы с новыми номерами
    for fname in SCAN_ORDER:
        fpath = CHAPTERS_DIR / fname
        if not fpath.exists():
            continue
        t = fpath.read_text(encoding="utf-8")
        new_t = renumber_chapter(t, mapping)
        if new_t != t:
            fpath.write_text(new_t, encoding="utf-8")
            # Подсчёт изменений
            old_count = len(CITATION_RE.findall(t))
            print(f"  {fname}: обновлено {old_count} ссылок")

    # Проверка: нет ли неразрешённых [?N?]
    unresolved = 0
    for fname in SCAN_ORDER:
        fpath = CHAPTERS_DIR / fname
        if not fpath.exists():
            continue
        t = fpath.read_text(encoding="utf-8")
        unresolved += t.count("?")  # эвристика
    print(f"\nГотово. Итоговый счёт источников: {len(used)}")


if __name__ == "__main__":
    main()
