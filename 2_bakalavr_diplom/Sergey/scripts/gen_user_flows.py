#!/usr/bin/env python3
"""
Генерация правильных user flow диаграмм — как путь пользователя через экраны,
а не как activity UML. Прямоугольники-экраны + прямые стрелки, стиль ч/б/синий.

Отличия от activity:
- экраны рисуются как обычные прямоугольники (без закруглений и ромбов);
- нет «чёрных кругов» начала и конца;
- вторичные ветки (ошибки, возвраты) показываются штриховыми стрелками;
- фокус на ПОСЛЕДОВАТЕЛЬНОСТИ ЭКРАНОВ, не на условных переходах.
"""
from pathlib import Path
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import Rectangle, FancyArrowPatch

IMAGES = Path("/home/washka/project/diplom/Sergey/images")
DPI = 150

BRAND_BLUE = "#6C67FD"
BLACK = "#000000"
WHITE = "#FFFFFF"
LIGHT = "#E6E6E6"
MUTED = "#A2A2A2"

plt.rcParams["font.family"] = ["DejaVu Sans", "Liberation Sans", "sans-serif"]


def _screen(ax, x, y, w, h, title, subtitle=None, accent=False):
    """Прямоугольник-экран с заголовком и (опционально) подзаголовком."""
    face = BRAND_BLUE if accent else WHITE
    text_color = WHITE if accent else BLACK
    rect = Rectangle((x, y), w, h, facecolor=face, edgecolor=BLACK, linewidth=1.3)
    ax.add_patch(rect)
    if subtitle:
        ax.text(x + w / 2, y + h / 2 + 0.12, title, ha="center", va="center",
                fontsize=10, fontweight="bold", color=text_color)
        ax.text(x + w / 2, y + h / 2 - 0.2, subtitle, ha="center", va="center",
                fontsize=8, color=text_color, style="italic")
    else:
        ax.text(x + w / 2, y + h / 2, title, ha="center", va="center",
                fontsize=10, fontweight="bold", color=text_color, wrap=True)


def _arrow(ax, x1, y1, x2, y2, label=None, dashed=False, ortho=True):
    style = "dashed" if dashed else "solid"
    if ortho and x1 != x2 and y1 != y2:
        # Г-образная ломаная
        ax.plot([x1, x2], [y1, y1], color=BLACK, linewidth=1.1, linestyle=style)
        arr = FancyArrowPatch((x2, y1), (x2, y2), arrowstyle="->",
                              mutation_scale=14, color=BLACK, linewidth=1.1,
                              linestyle=style)
        ax.add_patch(arr)
    else:
        arr = FancyArrowPatch((x1, y1), (x2, y2), arrowstyle="->",
                              mutation_scale=14, color=BLACK, linewidth=1.1,
                              linestyle=style)
        ax.add_patch(arr)
    if label:
        ax.text((x1 + x2) / 2, (y1 + y2) / 2 + 0.12, label, fontsize=9,
                color=BLACK, ha="center", va="bottom",
                bbox=dict(facecolor=WHITE, edgecolor="none", pad=1))


def _title(ax, text):
    ax.text(0.5, 0.98, text, transform=ax.transAxes,
            ha="center", va="top", fontsize=13, fontweight="bold", color=BLACK)


# ============================================================
# 2.5.1 — Регистрация и выбор тарифа
# ============================================================

def flow_registration():
    fig, ax = plt.subplots(figsize=(14, 4.5), dpi=DPI)
    ax.set_xlim(0, 14)
    ax.set_ylim(0, 4)
    ax.axis("off")
    _title(ax, "Пользовательский поток: регистрация и выбор тарифа")

    screens = [
        (0.3, 2.0, "Лендинг",           "главная страница"),
        (2.6, 2.0, "Форма входа",       "«Зарегистрироваться»"),
        (4.9, 2.0, "Форма\nрегистрации", "email + пароль"),
        (7.2, 2.0, "Письмо\nподтверждения", "клик по ссылке"),
        (9.5, 2.0, "Выбор тарифа",      "3 плана"),
        (11.8, 2.0, "Рабочий стол",     "сервис готов"),
    ]
    for i, (x, y, title, sub) in enumerate(screens):
        _screen(ax, x, y, 2.1, 1.2, title, sub, accent=(i == len(screens) - 1))

    # прямые стрелки между экранами
    for i in range(len(screens) - 1):
        x1 = screens[i][0] + 2.1
        x2 = screens[i + 1][0]
        y = 2.6
        arr = FancyArrowPatch((x1, y), (x2, y), arrowstyle="->",
                              mutation_scale=14, color=BLACK, linewidth=1.2)
        ax.add_patch(arr)

    # ветка: платный тариф → форма оплаты → возврат
    _screen(ax, 9.5, 0.3, 2.1, 1.2, "Форма оплаты", "платёжный шлюз")
    ax.plot([10.55, 10.55], [2.0, 1.5], color=BLACK, linewidth=1.2, linestyle="--")
    arr = FancyArrowPatch((10.55, 1.5), (10.55, 1.5), arrowstyle="->",
                          mutation_scale=1, color=BLACK, linewidth=1.2, linestyle="--")
    ax.add_patch(arr)
    ax.text(10.7, 1.75, "платный", fontsize=8, color=MUTED, style="italic")
    # возврат из оплаты
    ax.plot([11.6, 12.85], [0.9, 0.9], color=BLACK, linewidth=1.2, linestyle="--")
    ax.plot([12.85, 12.85], [0.9, 2.0], color=BLACK, linewidth=1.2, linestyle="--")

    out = IMAGES / "2.9_user_flow_registration.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"  OK {out.name}  ({out.stat().st_size // 1024} KB)")


# ============================================================
# 2.5.2 — Загрузка и анализ документа
# ============================================================

def flow_analysis():
    fig, ax = plt.subplots(figsize=(14, 4.5), dpi=DPI)
    ax.set_xlim(0, 14)
    ax.set_ylim(0, 4)
    ax.axis("off")
    _title(ax, "Пользовательский поток: загрузка и анализ документа")

    screens = [
        (0.3, 2.0, "Рабочий стол",     "кнопка загрузки"),
        (2.6, 2.0, "Выбор файла",      "drag-n-drop или диалог"),
        (4.9, 2.0, "Прогресс",         "загрузка и анализ"),
        (7.2, 2.0, "Документ\nс подсветкой", "риски выделены"),
        (9.5, 2.0, "Панель рисков",    "список замечаний"),
        (11.8, 2.0, "Действия\nс рисками", "статусы и заметки"),
    ]
    for i, (x, y, title, sub) in enumerate(screens):
        _screen(ax, x, y, 2.1, 1.2, title, sub, accent=(i in (3, 5)))

    for i in range(len(screens) - 1):
        x1 = screens[i][0] + 2.1
        x2 = screens[i + 1][0]
        arr = FancyArrowPatch((x1, 2.6), (x2, 2.6), arrowstyle="->",
                              mutation_scale=14, color=BLACK, linewidth=1.2)
        ax.add_patch(arr)

    # вторичная ветка ошибок
    _screen(ax, 2.6, 0.3, 2.1, 1.2, "Ошибка",
            "формат/размер не подходит")
    ax.plot([3.65, 3.65], [2.0, 1.5], color=BLACK, linewidth=1.2, linestyle="--")
    ax.text(3.8, 1.7, "невалидный", fontsize=8, color=MUTED, style="italic")

    out = IMAGES / "2.10_user_flow_analysis.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"  OK {out.name}  ({out.stat().st_size // 1024} KB)")


# ============================================================
# 2.5.3 — Работа с результатами
# ============================================================

def flow_results():
    fig, ax = plt.subplots(figsize=(14, 5), dpi=DPI)
    ax.set_xlim(0, 14)
    ax.set_ylim(0, 4.5)
    ax.axis("off")
    _title(ax, "Пользовательский поток: работа с результатами анализа")

    main_screens = [
        (0.3, 2.5, "Документ\nс рисками",   "подсветка фрагментов"),
        (2.6, 2.5, "Выбор риска",           "клик в тексте/списке"),
        (4.9, 2.5, "Карточка риска",        "пояснение ИИ"),
        (7.2, 2.5, "Действия",              "статус и заметка"),
        (11.8, 2.5, "PDF-отчёт",            "скачать на диск"),
    ]
    for i, (x, y, title, sub) in enumerate(main_screens):
        _screen(ax, x, y, 2.1, 1.2, title, sub, accent=(i == 4))

    # основные стрелки
    for i in range(3):
        x1 = main_screens[i][0] + 2.1
        x2 = main_screens[i + 1][0]
        arr = FancyArrowPatch((x1, 3.1), (x2, 3.1), arrowstyle="->",
                              mutation_scale=14, color=BLACK, linewidth=1.2)
        ax.add_patch(arr)
    # Действия → Отчёт с пропуском
    arr = FancyArrowPatch((9.3, 3.1), (11.8, 3.1), arrowstyle="->",
                          mutation_scale=14, color=BLACK, linewidth=1.2)
    ax.add_patch(arr)

    # побочная ветка: ИИ-помощник
    _screen(ax, 4.9, 0.7, 2.1, 1.2, "Чат с ИИ",
            "уточняющий вопрос")
    ax.plot([5.95, 5.95], [2.5, 1.9], color=BLACK, linewidth=1.2, linestyle="--")
    ax.text(6.1, 2.1, "при сложном риске", fontsize=8, color=MUTED, style="italic")
    # возврат из ИИ
    ax.plot([7.0, 8.25], [1.3, 1.3], color=BLACK, linewidth=1.2, linestyle="--")
    ax.plot([8.25, 8.25], [1.3, 2.5], color=BLACK, linewidth=1.2, linestyle="--")

    # цикл возврата к риску
    ax.plot([8.25, 8.25], [3.7, 4.0], color=MUTED, linewidth=1.0, linestyle=":")
    ax.plot([8.25, 3.65], [4.0, 4.0], color=MUTED, linewidth=1.0, linestyle=":")
    arr = FancyArrowPatch((3.65, 4.0), (3.65, 3.7), arrowstyle="->",
                          mutation_scale=12, color=MUTED, linewidth=1.0, linestyle=":")
    ax.add_patch(arr)
    ax.text(6.0, 4.1, "следующий риск", fontsize=8, color=MUTED, style="italic", ha="center")

    out = IMAGES / "2.11_user_flow_results.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"  OK {out.name}  ({out.stat().st_size // 1024} KB)")


if __name__ == "__main__":
    flow_registration()
    flow_analysis()
    flow_results()
