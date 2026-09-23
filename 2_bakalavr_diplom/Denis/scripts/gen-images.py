"""Генерация всех рисунков для ВКР Дениса (14 изображений).

BPMN 2.0: круг = событие, прямоугольник со скруглёнными углами = задача, ромб = шлюз.
UML use case: эллипсы + стикмены, <<include>> и <<extend>> пунктиром.
ER (Crow's Foot): PK/FK, символы кардинальности.
User Story Map по Паттону: backbone сверху, ribs вниз, приоритет MoSCoW.

Принципы верстки:
- большой figsize с запасом
- явные координаты всех элементов
- проверка отсутствия перекрытий вручную (bbox_inches='tight' для полей)
- текст всегда внутри фигур с padding
"""
from __future__ import annotations

from pathlib import Path

import matplotlib.pyplot as plt
from matplotlib.patches import (
    FancyBboxPatch,
    FancyArrowPatch,
    Rectangle,
    Polygon,
    Circle,
    Ellipse,
    ConnectionPatch,
)
from matplotlib.lines import Line2D
import matplotlib.patheffects as pe

OUT = Path(__file__).resolve().parent.parent / "images"
OUT.mkdir(parents=True, exist_ok=True)

# Палитра — спокойные, под печать, контрастные
NAVY = "#1F3A5F"
NAVY_LIGHT = "#5A7199"
GOLD = "#F1D669"
GOLD_LIGHT = "#FAE9A8"
GOLD_DARK = "#D4B340"
CREAM = "#FDF6DD"
GRAY = "#6B6B6B"
GRAY_LIGHT = "#D0D0D0"
WHITE = "#FFFFFF"
BLACK = "#1A1A1A"
RED = "#C0392B"
RED_LIGHT = "#F1B0A7"
GREEN = "#27AE60"
GREEN_LIGHT = "#B2E0C4"
YELLOW = "#F39C12"
YELLOW_LIGHT = "#FAD89B"
BLUE = "#2980B9"
BLUE_LIGHT = "#AAD4E8"

plt.rcParams["font.family"] = "DejaVu Sans"
plt.rcParams["font.size"] = 11


def _save(fig, name: str) -> None:
    p = OUT / name
    fig.savefig(p, dpi=160, bbox_inches="tight", facecolor="white", pad_inches=0.15)
    print(f"saved: {p}")
    plt.close(fig)


# =============================================================
# BPMN-компоненты (общие для 2.1 и 2.2)
# =============================================================
def bpmn_event(ax, cx, cy, r=0.35, kind="start", label=""):
    lw = 2 if kind == "start" else 4
    circle = Circle((cx, cy), r, facecolor=WHITE, edgecolor=BLACK, linewidth=lw, zorder=3)
    ax.add_patch(circle)
    if label:
        ax.text(cx, cy - r - 0.3, label, ha="center", va="top", fontsize=10,
                color=BLACK, zorder=4)


def bpmn_task(ax, cx, cy, w=2.4, h=1.0, label="", fill=GOLD_LIGHT):
    shp = FancyBboxPatch((cx - w / 2, cy - h / 2), w, h,
                         boxstyle="round,pad=0.02,rounding_size=0.15",
                         facecolor=fill, edgecolor=BLACK, linewidth=1.4, zorder=3)
    ax.add_patch(shp)
    ax.text(cx, cy, label, ha="center", va="center", fontsize=10,
            color=BLACK, wrap=True, zorder=4)


def bpmn_gateway(ax, cx, cy, size=0.7, label=""):
    pts = [(cx, cy + size), (cx + size, cy), (cx, cy - size), (cx - size, cy)]
    gw = Polygon(pts, closed=True, facecolor=WHITE, edgecolor=BLACK,
                 linewidth=1.5, zorder=3)
    ax.add_patch(gw)
    ax.text(cx, cy, "X", ha="center", va="center", fontsize=14,
            fontweight="bold", color=BLACK, zorder=4)
    if label:
        ax.text(cx, cy - size - 0.3, label, ha="center", va="top", fontsize=9,
                color=BLACK, zorder=4)


def bpmn_arrow(ax, x1, y1, x2, y2, style="->", dashed=False):
    kwargs = dict(arrowstyle=style, color=BLACK, linewidth=1.4,
                  mutation_scale=15, zorder=2)
    if dashed:
        kwargs["linestyle"] = "dashed"
    arrow = FancyArrowPatch((x1, y1), (x2, y2), **kwargs)
    ax.add_patch(arrow)


def bpmn_lane(ax, x, y, w, h, label):
    lane = Rectangle((x, y), w, h, facecolor=GOLD_LIGHT, edgecolor=NAVY,
                     linewidth=1.5, alpha=0.25, zorder=1)
    ax.add_patch(lane)
    # Левая полоса с подписью
    hdr = Rectangle((x, y), 1.1, h, facecolor=NAVY, edgecolor=NAVY,
                    linewidth=1.5, zorder=1)
    ax.add_patch(hdr)
    ax.text(x + 0.55, y + h / 2, label, ha="center", va="center",
            fontsize=11, fontweight="bold", color=WHITE, rotation=90, zorder=2)


# =============================================================
# Рисунок 2.1 — BPMN «как есть»
# =============================================================
def make_bpmn_as_is():
    fig, ax = plt.subplots(figsize=(18, 7.5))
    ax.set_xlim(0, 22)
    ax.set_ylim(-0.5, 8.5)
    ax.axis("off")

    # Дорожки — оставляем слева 1.3 на подпись, контент начиная с x=1.7
    bpmn_lane(ax, 0.2, 5.3, 21.5, 1.8, "Студент")
    bpmn_lane(ax, 0.2, 1.6, 21.5, 3.3, "Преподаватель")

    # Студент
    bpmn_event(ax, 2.5, 6.2, kind="start", label="Есть\nзадание")
    bpmn_task(ax, 5.2, 6.2, w=2.6, h=0.9, label="Загрузить файл\nответа", fill=CREAM)
    bpmn_event(ax, 7.6, 6.2, kind="end", label="Сдано")
    bpmn_arrow(ax, 2.85, 6.2, 3.9, 6.2)
    bpmn_arrow(ax, 6.5, 6.2, 7.25, 6.2)

    # Преподаватель
    bpmn_event(ax, 2.5, 3.3, kind="start", label="Срок сдачи\nзакрыт")
    bpmn_task(ax, 5.3, 3.3, w=2.6, h=1.0, label="Открыть файл\nв браузере", fill=CREAM)
    bpmn_task(ax, 8.4, 3.3, w=2.8, h=1.0, label="Сверить глазами\nс прежними", fill=RED_LIGHT)
    bpmn_gateway(ax, 11.4, 3.3, label="Совпадения?")
    bpmn_task(ax, 14.5, 4.2, w=2.6, h=0.9, label="Снизить оценку", fill=YELLOW_LIGHT)
    bpmn_task(ax, 14.5, 2.4, w=2.6, h=0.9, label="Поставить оценку", fill=CREAM)
    bpmn_gateway(ax, 17.4, 3.3, label="Ещё файлы?")
    bpmn_event(ax, 20.0, 3.3, kind="end", label="Конец")

    bpmn_arrow(ax, 2.85, 3.3, 4.0, 3.3)
    bpmn_arrow(ax, 6.6, 3.3, 7.0, 3.3)
    bpmn_arrow(ax, 9.8, 3.3, 10.7, 3.3)
    bpmn_arrow(ax, 11.4, 4.0, 14.5, 4.2)
    ax.text(12.6, 4.35, "Да", fontsize=9.5, color=BLACK)
    bpmn_arrow(ax, 11.4, 2.6, 14.5, 2.4)
    ax.text(12.6, 2.25, "Нет", fontsize=9.5, color=BLACK)
    bpmn_arrow(ax, 15.8, 4.2, 17.4, 4.0)
    bpmn_arrow(ax, 15.8, 2.4, 17.4, 2.6)
    # Цикл «Да → к следующему файлу»
    bpmn_arrow(ax, 17.4, 2.6, 17.4, -0.2)
    bpmn_arrow(ax, 17.4, -0.2, 5.3, -0.2)
    bpmn_arrow(ax, 5.3, -0.2, 5.3, 2.8)
    ax.text(11.0, -0.45, "Да — к следующему файлу", fontsize=9.5,
            color=BLACK, ha="center")
    bpmn_arrow(ax, 18.1, 3.3, 19.65, 3.3)
    ax.text(18.85, 3.55, "Нет", fontsize=9.5, color=BLACK, ha="center")

    # Message flow: студент → преподаватель
    arrow = FancyArrowPatch((5.2, 5.75), (5.3, 3.8),
                             arrowstyle="->", color=GRAY, linewidth=1.2,
                             linestyle="dashed", mutation_scale=13, zorder=2)
    ax.add_patch(arrow)

    ax.text(11, 7.9, "Ручная попарная проверка работ (без плагина)",
            ha="center", va="center", fontsize=14, fontweight="bold", color=NAVY)

    _save(fig, "pic_2_1_bpmn_as_is.png")


# =============================================================
# Рисунок 2.2 — BPMN «как будет»
# =============================================================
def make_bpmn_to_be():
    fig, ax = plt.subplots(figsize=(18, 9))
    ax.set_xlim(0, 22)
    ax.set_ylim(0, 10)
    ax.axis("off")

    # Три дорожки
    bpmn_lane(ax, 0.2, 7.4, 21.5, 1.7, "Студент")
    bpmn_lane(ax, 0.2, 4.0, 21.5, 3.0, "Плагин samost")
    bpmn_lane(ax, 0.2, 1.0, 21.5, 2.7, "Преподаватель")

    # Студент
    bpmn_event(ax, 2.5, 8.25, kind="start", label="Есть задание")
    bpmn_task(ax, 5.2, 8.25, w=2.6, h=0.9, label="Загрузить файл\nответа", fill=CREAM)
    bpmn_event(ax, 7.6, 8.25, kind="end", label="Сдано")
    bpmn_arrow(ax, 2.85, 8.25, 3.9, 8.25)
    bpmn_arrow(ax, 6.5, 8.25, 7.25, 8.25)

    # Плагин
    bpmn_event(ax, 2.5, 5.5, kind="start", label="Cron:\nсрок истёк")
    bpmn_task(ax, 5.4, 5.5, w=2.8, h=0.9, label="Создать пары\nв очереди", fill=GOLD)
    bpmn_task(ax, 8.6, 5.5, w=2.8, h=0.9, label="Извлечь текст\nиз файлов", fill=GOLD)
    bpmn_task(ax, 11.8, 5.5, w=3.0, h=0.9, label="Вычислить\nшинглы / Winnowing", fill=GOLD)
    bpmn_task(ax, 15.2, 5.5, w=2.8, h=0.9, label="Сохранить\nрезультаты", fill=GOLD)
    bpmn_event(ax, 18.0, 5.5, kind="end", label="Готов отчёт")
    bpmn_arrow(ax, 2.85, 5.5, 4.0, 5.5)
    bpmn_arrow(ax, 6.8, 5.5, 7.2, 5.5)
    bpmn_arrow(ax, 10.0, 5.5, 10.3, 5.5)
    bpmn_arrow(ax, 13.3, 5.5, 13.8, 5.5)
    bpmn_arrow(ax, 16.6, 5.5, 17.65, 5.5)

    # Преподаватель — лёгкий путь
    bpmn_event(ax, 2.5, 2.3, kind="start", label="Получил\nуведомление")
    bpmn_task(ax, 5.2, 2.3, w=2.6, h=0.9, label="Открыть\nотчёт", fill=CREAM)
    bpmn_gateway(ax, 8.3, 2.3, label="Красные ячейки?")
    bpmn_task(ax, 11.6, 3.0, w=3.0, h=0.9, label="Раскрыть пару,\nсмотреть подсветку", fill=RED_LIGHT)
    bpmn_task(ax, 11.6, 1.6, w=3.0, h=0.9, label="Поставить оценки\nшаблонно", fill=CREAM)
    bpmn_task(ax, 15.2, 2.3, w=2.6, h=0.9, label="Выставить\nоценки", fill=CREAM)
    bpmn_event(ax, 17.8, 2.3, kind="end", label="Проверено")

    bpmn_arrow(ax, 2.85, 2.3, 3.9, 2.3)
    bpmn_arrow(ax, 6.5, 2.3, 7.6, 2.3)
    bpmn_arrow(ax, 8.3, 3.0, 10.1, 3.0)
    ax.text(9.1, 3.2, "Да", fontsize=9.5, color=BLACK)
    bpmn_arrow(ax, 8.3, 1.6, 10.1, 1.6)
    ax.text(9.1, 1.4, "Нет", fontsize=9.5, color=BLACK)
    bpmn_arrow(ax, 13.1, 3.0, 15.2, 2.55)
    bpmn_arrow(ax, 13.1, 1.6, 15.2, 2.05)
    bpmn_arrow(ax, 16.5, 2.3, 17.45, 2.3)

    # Message flows
    arrow1 = FancyArrowPatch((5.2, 7.8), (5.4, 6.0), arrowstyle="->",
                              color=GRAY, linewidth=1.2, linestyle="dashed",
                              mutation_scale=13, zorder=2)
    ax.add_patch(arrow1)
    arrow2 = FancyArrowPatch((18.0, 5.05), (2.9, 2.75), arrowstyle="->",
                              color=GRAY, linewidth=1.2, linestyle="dashed",
                              mutation_scale=13, zorder=2,
                              connectionstyle="arc3,rad=-0.18")
    ax.add_patch(arrow2)
    ax.text(10.5, 4.25, "уведомление о готовности отчёта", fontsize=9.5,
            color=GRAY, style="italic", ha="center")

    ax.text(11, 9.55, "Автоматическая попарная проверка работ (с плагином)",
            ha="center", va="center", fontsize=14, fontweight="bold", color=NAVY)

    _save(fig, "pic_2_2_bpmn_to_be.png")


# =============================================================
# Рисунок 2.3 — User Story Map
# =============================================================
def make_story_map():
    fig, ax = plt.subplots(figsize=(15, 8))
    ax.set_xlim(0, 20)
    ax.set_ylim(0, 9)
    ax.axis("off")

    # Backbone: 5 этапов
    stages = ["Настройка", "Загрузка работ", "Запуск проверки",
              "Просмотр результата", "Принятие решения"]
    stage_w = 3.6
    start_x = 0.5
    bb_y = 7.3
    for i, s in enumerate(stages):
        cx = start_x + i * stage_w
        ax.add_patch(FancyBboxPatch((cx, bb_y), stage_w - 0.15, 0.9,
                                      boxstyle="round,pad=0.02,rounding_size=0.08",
                                      facecolor=NAVY, edgecolor=NAVY, linewidth=1.5))
        ax.text(cx + (stage_w - 0.15) / 2, bb_y + 0.45, s,
                ha="center", va="center", fontsize=12,
                fontweight="bold", color=WHITE)

    # Подпись backbone слева
    ax.text(start_x - 0.4, bb_y + 0.45, "Backbone\n(этапы\nпроцесса)",
            ha="right", va="center", fontsize=10, style="italic",
            color=NAVY, fontweight="bold")

    # Приоритетные полосы (MoSCoW)
    band_labels = [("MUST (MVP)", 5.4, GREEN_LIGHT),
                   ("SHOULD",      3.3, YELLOW_LIGHT),
                   ("COULD",       1.2, BLUE_LIGHT)]
    for lbl, y, col in band_labels:
        ax.add_patch(Rectangle((start_x, y - 0.1), 5 * stage_w - 0.15, 1.9,
                                facecolor=col, edgecolor="none", alpha=0.35, zorder=1))
        ax.text(start_x - 0.4, y + 0.85, lbl, ha="right", va="center",
                fontsize=10, fontweight="bold", color=NAVY, style="italic")

    # Истории по ячейкам (x index, y band, text)
    stories = [
        # Настройка
        (0, 5.4, "US-01 Установить\nплагин за 5 мин"),
        (0, 3.3, "US-08 Настраивать\nпороги"),
        (0, 5.4, None),  # второй MVP
        # Загрузка
        (1, 5.4, "US-03 Студент\nзагружает как обычно"),
        # Запуск
        (2, 5.4, "US-02 Чекбокс\n«проверка включена»"),
        (2, 3.3, "US-04 Уведомление\nо завершении"),
        # Просмотр
        (3, 5.4, "US-05 Таблица пар\nс цветовой шкалой"),
        (3, 5.4, None),
        (3, 3.3, "US-06 Подсветка\nсовпадений"),
        # Решение
        (4, 5.4, None),
        (4, 1.2, "US-07 CSV-экспорт"),
    ]
    # Группируем и раскладываем одной в ячейке
    # Упрощаем: повторно рисуем только те, что с текстом
    filtered = [s for s in stories if s[2]]
    # counts per (col, row) для сдвига
    counts = {}
    for col, row, _ in filtered:
        counts[(col, row)] = counts.get((col, row), 0) + 1

    placed = {}
    for col, row, text in filtered:
        idx = placed.get((col, row), 0)
        placed[(col, row)] = idx + 1
        total = counts[(col, row)]
        cell_w = stage_w - 0.4
        card_w = cell_w
        card_h = 1.1
        cx = start_x + col * stage_w + (stage_w - 0.15) / 2
        cy = row + 0.45
        ax.add_patch(FancyBboxPatch((cx - card_w / 2, cy - card_h / 2), card_w, card_h,
                                      boxstyle="round,pad=0.02,rounding_size=0.08",
                                      facecolor=WHITE, edgecolor=NAVY, linewidth=1.2,
                                      zorder=3))
        ax.text(cx, cy, text, ha="center", va="center", fontsize=9.5,
                color=BLACK, zorder=4)

    ax.text(10, 8.55, "Карта пользовательских историй (story map по Паттону)",
            ha="center", va="center", fontsize=13, fontweight="bold", color=NAVY)

    # Walking skeleton подчёркнут
    ax.text(start_x, 0.3, "Walking skeleton = только MUST-линия (MVP первой итерации)",
            ha="left", va="center", fontsize=10, style="italic", color=GREEN)

    _save(fig, "pic_2_3_story_map.png")


# =============================================================
# Рисунок 2.4 — Use Case Diagram
# =============================================================
def stickman(ax, cx, cy, label, scale=1.0):
    # Голова
    ax.add_patch(Circle((cx, cy + 0.7 * scale), 0.18 * scale,
                         facecolor=WHITE, edgecolor=BLACK, linewidth=1.4, zorder=3))
    # Туловище
    ax.plot([cx, cx], [cy + 0.52 * scale, cy - 0.1 * scale],
            color=BLACK, linewidth=1.4, zorder=2)
    # Руки
    ax.plot([cx - 0.3 * scale, cx + 0.3 * scale],
            [cy + 0.25 * scale, cy + 0.25 * scale],
            color=BLACK, linewidth=1.4, zorder=2)
    # Ноги
    ax.plot([cx, cx - 0.25 * scale], [cy - 0.1 * scale, cy - 0.55 * scale],
            color=BLACK, linewidth=1.4, zorder=2)
    ax.plot([cx, cx + 0.25 * scale], [cy - 0.1 * scale, cy - 0.55 * scale],
            color=BLACK, linewidth=1.4, zorder=2)
    ax.text(cx, cy - 0.85 * scale, label, ha="center", va="top",
            fontsize=11, fontweight="bold", color=NAVY, zorder=3)


def usecase_oval(ax, cx, cy, w, h, label):
    ax.add_patch(Ellipse((cx, cy), w, h, facecolor=GOLD_LIGHT,
                           edgecolor=NAVY, linewidth=1.4, zorder=3))
    ax.text(cx, cy, label, ha="center", va="center", fontsize=10.5,
            color=BLACK, zorder=4)


def make_use_case():
    # Разносим use cases по 1 на ряд в пределах колонки; системная рамка высокая и узкая по горизонтали.
    fig, ax = plt.subplots(figsize=(18, 13))
    ax.set_xlim(0, 22)
    ax.set_ylim(0, 14)
    ax.axis("off")

    # Системная рамка
    ax.add_patch(Rectangle((4.5, 0.6), 14.5, 12.6,
                            facecolor="white", edgecolor=NAVY,
                            linewidth=2, zorder=1))
    ax.text(11.75, 12.8, "Moodle + плагин samost",
            ha="center", va="center", fontsize=13, fontweight="bold",
            color=NAVY, zorder=2)

    # Акторы слева
    stickman(ax, 2.0, 11.0, "Преподаватель")
    stickman(ax, 2.0, 6.7, "Студент")
    stickman(ax, 2.0, 2.5, "Администратор")

    # Системный актор справа (Cron)
    stickman(ax, 20.2, 6.7, "Moodle Cron")

    # Use cases (cx, cy, w, h, label). Разносим по ряду так, чтобы эллипсы не перекрывались.
    uc = {
        # колонка 1 — пользовательские (центр сетки x≈7.5)
        "setup":    (7.5, 11.7, 4.0, 0.9, "Настроить задание"),
        "run":      (7.5, 10.3, 4.0, 0.9, "Запустить проверку вручную"),
        "report":   (7.5,  8.9, 4.0, 0.9, "Просмотреть отчёт"),
        "upload":   (7.5,  6.7, 4.0, 0.9, "Загрузить работу"),
        "install":  (7.5,  3.3, 4.0, 0.9, "Установить/обновить плагин"),
        "tune":     (7.5,  1.9, 4.0, 0.9, "Настроить пороги плагина"),
        # колонка 2 — расширения и включаемые (x≈12.5)
        "detail":   (12.5, 9.7, 4.4, 0.9, "Раскрыть детализацию пары"),
        "export":   (12.5, 8.1, 4.4, 0.9, "Экспортировать в CSV"),
        "extract":  (12.5, 7.2, 4.4, 0.9, "Извлечь текст"),
        "hash":     (12.5, 5.9, 4.4, 0.9, "Вычислить отпечатки"),
        "store":    (12.5, 4.6, 4.4, 0.9, "Сохранить результаты"),
        # колонка 3 — Cron (x≈17)
        "cron":     (17.0, 6.7, 4.0, 0.9, "Запустить проверку по расписанию"),
    }
    for key, (cx, cy, w, h, label) in uc.items():
        usecase_oval(ax, cx, cy, w, h, label)

    def connect_actor(actor_xy, uc_edge, rad=0):
        arr = FancyArrowPatch(actor_xy, uc_edge,
                              arrowstyle="-", color=BLACK, linewidth=1.1,
                              mutation_scale=10, zorder=2,
                              connectionstyle=f"arc3,rad={rad}")
        ax.add_patch(arr)

    # Преподаватель → setup / run / report (линия к левой кромке каждого эллипса x=5.5)
    connect_actor((2.4, 11.0), (5.5, 11.7), rad=-0.05)
    connect_actor((2.4, 11.0), (5.5, 10.3),  rad=0.02)
    connect_actor((2.4, 11.0), (5.5,  8.9),  rad=0.1)
    # Студент → upload
    connect_actor((2.4, 6.7), (5.5, 6.7))
    # Админ → install / tune
    connect_actor((2.4, 2.5), (5.5, 3.3), rad=-0.08)
    connect_actor((2.4, 2.5), (5.5, 1.9), rad=0.03)
    # Cron → run by schedule
    connect_actor((19.8, 6.7), (19.0, 6.7))

    # Include (из run и cron в цепочку обработки)
    def include_arrow(a_xy, b_xy, label_offset=(0.1, 0.15), color=NAVY):
        arr = FancyArrowPatch(a_xy, b_xy,
                              arrowstyle="-|>", linestyle="dashed",
                              color=color, linewidth=1.0,
                              mutation_scale=10, zorder=2)
        ax.add_patch(arr)
        mx = (a_xy[0] + b_xy[0]) / 2
        my = (a_xy[1] + b_xy[1]) / 2
        ax.text(mx + label_offset[0], my + label_offset[1], "«include»",
                fontsize=8.5, color=color, style="italic")

    # Run → extract, extract → hash, hash → store
    include_arrow((9.5, 10.3), (10.3, 7.2))          # run → extract
    include_arrow((12.5, 6.75), (12.5, 6.35))        # extract → hash
    include_arrow((12.5, 5.45), (12.5, 5.05))        # hash → store
    # Cron → extract (шаред)
    include_arrow((15.0, 6.7), (14.7, 7.2), color=NAVY)

    # Extend: Экспорт → Report (условно)
    ext = FancyArrowPatch((12.5, 8.55), (9.5, 8.9),
                          arrowstyle="-|>", linestyle="dashed",
                          color=BLUE, linewidth=1.0, mutation_scale=10, zorder=2,
                          connectionstyle="arc3,rad=0.1")
    ax.add_patch(ext)
    ax.text(10.8, 8.65, "«extend»", fontsize=9, color=BLUE, style="italic")

    # Detail → report (include) — раскрытие детализации включает просмотр
    det_arr = FancyArrowPatch((12.5, 10.15), (9.5, 8.9),
                              arrowstyle="-|>", linestyle="dashed",
                              color=NAVY, linewidth=1.0, mutation_scale=10, zorder=2,
                              connectionstyle="arc3,rad=-0.1")
    ax.add_patch(det_arr)
    ax.text(10.8, 9.8, "«include»", fontsize=9, color=NAVY, style="italic")

    ax.text(11.75, 0.2,
            "Пунктирная стрелка с пустым наконечником — отношение <<include>> / <<extend>>",
            ha="center", va="center", fontsize=10, style="italic", color=GRAY)

    _save(fig, "pic_2_4_use_case.png")


# =============================================================
# Рисунок 3.1 — Архитектура плагина (слоистая)
# =============================================================
def make_architecture():
    # Каждый слой = верхняя узкая плашка (title + subtitle) + отдельный ряд белых карточек под ней.
    # Никаких наложений.
    fig, ax = plt.subplots(figsize=(16, 12))
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 14.5)
    ax.axis("off")

    def layer_header(x, y, w, h, title, subtitle, fill):
        ax.add_patch(FancyBboxPatch((x, y), w, h,
                                       boxstyle="round,pad=0.03,rounding_size=0.12",
                                       facecolor=fill, edgecolor=NAVY, linewidth=1.6))
        ax.text(x + w / 2, y + h * 0.68, title, ha="center", va="center",
                fontsize=14, fontweight="bold", color=NAVY)
        ax.text(x + w / 2, y + h * 0.28, subtitle, ha="center", va="center",
                fontsize=10.5, color=NAVY, style="italic")

    def card(x, y, w, h, title, sub="", family="sans-serif"):
        ax.add_patch(FancyBboxPatch((x, y), w, h,
                                      boxstyle="round,pad=0.02,rounding_size=0.08",
                                      facecolor=WHITE, edgecolor=NAVY, linewidth=1.1))
        if sub:
            ax.text(x + w / 2, y + h * 0.68, title, ha="center", va="center",
                    fontsize=10.5, color=BLACK)
            ax.text(x + w / 2, y + h * 0.28, sub, ha="center", va="center",
                    fontsize=9, color=GRAY, family="monospace")
        else:
            ax.text(x + w / 2, y + h / 2, title, ha="center", va="center",
                    fontsize=10.5, color=BLACK, family=family)

    # Слой UI: заголовок (13.2 → 14.1) + карточки (11.8 → 12.9)
    layer_header(0.8, 13.2, 16.4, 0.95, "Слой интерфейса (UI)",
                 "Mustache-шаблоны  ·  Bootstrap 5  ·  AJAX", GOLD_LIGHT)
    for i, (label, file) in enumerate([
        ("Отчёт по заданию",   "report.php"),
        ("Попарное сравнение", "pair.php"),
        ("Вкладки и шапка",    "tabs/"),
        ("Админ-настройки",    "settings.php"),
    ]):
        x = 0.95 + i * 4.07
        card(x, 11.7, 3.85, 1.3, label, file)

    # Стрелка вниз
    arr = FancyArrowPatch((9, 11.6), (9, 11.0),
                           arrowstyle="<->", color=NAVY, linewidth=1.3,
                           mutation_scale=12)
    ax.add_patch(arr)
    ax.text(9.2, 11.3, "HTTP / AJAX", fontsize=9.5, color=NAVY, style="italic")

    # Слой обработки: заголовок (10.0) + карточки (8.2 → 9.8)
    layer_header(0.8, 10.0, 16.4, 0.95, "Слой обработки данных",
                 "Извлечение текста  ·  Нормализация  ·  Очередь задач", GOLD)
    for i, (label, file) in enumerate([
        ("lib.php",                  "точка входа Plugin API"),
        ("text_extractor.php",       "DOCX · PDF · XLSX · код"),
        ("task/process_queue.php",   "scheduled-task"),
        ("file_types.php",           "определение типа"),
    ]):
        x = 0.95 + i * 4.07
        ax.add_patch(FancyBboxPatch((x, 8.3), 3.85, 1.5,
                                      boxstyle="round,pad=0.02,rounding_size=0.08",
                                      facecolor=WHITE, edgecolor=NAVY, linewidth=1.1))
        ax.text(x + 1.925, 9.28, label, ha="center", va="center",
                fontsize=10, color=BLACK, family="monospace", fontweight="bold")
        ax.text(x + 1.925, 8.65, file, ha="center", va="center",
                fontsize=9, color=GRAY, style="italic")

    arr = FancyArrowPatch((9, 8.2), (9, 7.6),
                           arrowstyle="<->", color=NAVY, linewidth=1.3,
                           mutation_scale=12)
    ax.add_patch(arr)
    ax.text(9.2, 7.9, "вызов функций", fontsize=9.5, color=NAVY, style="italic")

    # Слой анализа: заголовок (6.6) + карточка (5.0 → 6.3)
    layer_header(0.8, 6.6, 16.4, 0.95, "Слой анализа",
                 "Шинглы + коэффициент Жаккара  ·  Winnowing", GOLD_DARK)
    ax.add_patch(FancyBboxPatch((3.0, 4.9), 12.0, 1.5,
                                   boxstyle="round,pad=0.02,rounding_size=0.08",
                                   facecolor=WHITE, edgecolor=NAVY, linewidth=1.1))
    ax.text(9, 5.85, "classes/analyser.php",
            ha="center", va="center",
            fontsize=11, color=BLACK, family="monospace", fontweight="bold")
    ax.text(9, 5.25, "jaccard_similarity() · winnowing_similarity() · tokenise_code()",
            ha="center", va="center",
            fontsize=9.5, color=GRAY, family="monospace")

    arr = FancyArrowPatch((9, 4.8), (9, 4.2),
                           arrowstyle="<->", color=NAVY, linewidth=1.3,
                           mutation_scale=12)
    ax.add_patch(arr)
    ax.text(9.2, 4.5, "SQL через DB API", fontsize=9.5, color=NAVY, style="italic")

    # Хранилище: заголовок (3.2) + карточки (1.8 → 2.9)
    layer_header(0.8, 3.2, 16.4, 0.95, "Хранилище",
                 "Moodle Database API  ·  PostgreSQL / MySQL", CREAM)
    for i, tbl in enumerate([
        "plagiarism_samost_config",
        "plagiarism_samost_queue",
        "plagiarism_samost_files",
        "plagiarism_samost_results",
    ]):
        x = 0.95 + i * 4.07
        ax.add_patch(FancyBboxPatch((x, 1.8), 3.85, 1.15,
                                      boxstyle="round,pad=0.02,rounding_size=0.08",
                                      facecolor=CREAM, edgecolor=NAVY, linewidth=1.1))
        ax.text(x + 1.925, 2.37, tbl, ha="center", va="center",
                fontsize=9.5, color=BLACK, family="monospace")

    # Внешний актор — Moodle core (сверху)
    ax.add_patch(FancyBboxPatch((13.5, 14.2), 3.6, 0.6,
                                   boxstyle="round,pad=0.02,rounding_size=0.08",
                                   facecolor=NAVY, edgecolor=NAVY, linewidth=1))
    ax.text(15.3, 14.5, "ядро Moodle / Plugin API",
            ha="center", va="center", fontsize=10.5, color=WHITE, fontweight="bold")
    arr = FancyArrowPatch((15.3, 14.2), (15.3, 14.15),
                           arrowstyle="->", color=NAVY, linewidth=1.3,
                           mutation_scale=12)
    ax.add_patch(arr)

    # Общий заголовок снизу
    ax.text(9, 0.9,
            "Слоистая архитектура плагина: UI → обработка → анализ → хранилище. Зависимости направлены вниз.",
            ha="center", va="center", fontsize=10.5, color=GRAY, style="italic")

    _save(fig, "pic_3_1_architecture.png")


# =============================================================
# Рисунок 3.2 — ER-диаграмма (crow's foot)
# =============================================================
def er_entity(ax, x, y, w, h, name, cols):
    # Заголовок
    ax.add_patch(Rectangle((x, y + h - 0.7), w, 0.7,
                            facecolor=NAVY, edgecolor=NAVY, linewidth=1.4))
    ax.text(x + w / 2, y + h - 0.35, name, ha="center", va="center",
            fontsize=11, fontweight="bold", color=WHITE, family="monospace")
    # Тело
    ax.add_patch(Rectangle((x, y), w, h - 0.7,
                            facecolor=WHITE, edgecolor=NAVY, linewidth=1.4))
    for i, (flag, col, typ) in enumerate(cols):
        yy = y + h - 1.0 - i * 0.38
        if flag == "PK":
            fcolor = GOLD
        elif flag == "FK":
            fcolor = BLUE_LIGHT
        else:
            fcolor = WHITE
        ax.add_patch(Rectangle((x + 0.05, yy - 0.15), 0.5, 0.3,
                                facecolor=fcolor, edgecolor="none"))
        ax.text(x + 0.3, yy, flag, ha="center", va="center",
                fontsize=8, fontweight="bold", color=BLACK)
        ax.text(x + 0.65, yy, col, ha="left", va="center",
                fontsize=9.5, color=BLACK, family="monospace")
        ax.text(x + w - 0.1, yy, typ, ha="right", va="center",
                fontsize=8.5, color=GRAY, family="monospace")


def crow_foot(ax, x, y, direction="left", kind="many"):
    """Рисует символ crow's foot (многие) или одну линию (один)."""
    size = 0.2
    if direction == "left":
        if kind == "many":
            ax.plot([x, x + size], [y, y + size], color=BLACK, linewidth=1.2)
            ax.plot([x, x + size], [y, y],         color=BLACK, linewidth=1.2)
            ax.plot([x, x + size], [y, y - size], color=BLACK, linewidth=1.2)
        else:  # one
            ax.plot([x + size / 2, x + size / 2], [y - 0.15, y + 0.15],
                    color=BLACK, linewidth=1.4)
    else:  # right
        if kind == "many":
            ax.plot([x, x - size], [y, y + size], color=BLACK, linewidth=1.2)
            ax.plot([x, x - size], [y, y],         color=BLACK, linewidth=1.2)
            ax.plot([x, x - size], [y, y - size], color=BLACK, linewidth=1.2)
        else:
            ax.plot([x - size / 2, x - size / 2], [y - 0.15, y + 0.15],
                    color=BLACK, linewidth=1.4)


def make_er():
    # Раскладка: Moodle-ядро слева узкой колонкой; плагин-таблицы справа в сетке 2×3.
    # Связи идут ортогонально, через «шины» — без пересечений.
    fig, ax = plt.subplots(figsize=(18, 10))
    ax.set_xlim(0, 22)
    ax.set_ylim(0, 12)
    ax.axis("off")

    # Рамка-фон для Moodle core
    ax.add_patch(Rectangle((0.3, 0.8), 4.8, 10.5,
                            facecolor=GRAY_LIGHT, edgecolor=GRAY,
                            linewidth=1, alpha=0.25, zorder=0))
    ax.text(2.7, 11.0, "Ядро Moodle", ha="center", va="center",
            fontsize=11.5, fontweight="bold", color=GRAY, style="italic", zorder=1)

    # Три основные сущности Moodle
    er_entity(ax, 0.5, 8.6, 4.4, 2.0, "mdl_user", [
        ("PK", "id", "BIGINT"),
        ("", "username", "VARCHAR"),
        ("", "email", "VARCHAR"),
    ])
    er_entity(ax, 0.5, 5.3, 4.4, 2.0, "mdl_assign", [
        ("PK", "id", "BIGINT"),
        ("FK", "course", "BIGINT"),
        ("", "name", "VARCHAR"),
    ])
    er_entity(ax, 0.5, 2.0, 4.4, 2.0, "mdl_files", [
        ("PK", "id", "BIGINT"),
        ("", "contenthash", "VARCHAR"),
        ("", "filename", "VARCHAR"),
    ])

    ax.text(13.5, 11.0,
            "Собственные таблицы плагина (префикс plagiarism_samost_)",
            ha="center", va="center", fontsize=11.5, fontweight="bold", color=NAVY)

    # Правая сетка 2 колонки × 3 ряда: samost_config | samost_queue
    #                                 samost_files  | samost_results
    #                                        samost_feedback (по центру, уже)
    er_entity(ax, 7.5, 8.6, 4.8, 2.0, "samost_config", [
        ("PK", "id", "BIGINT"),
        ("FK", "cm", "BIGINT"),
        ("", "name", "VARCHAR"),
        ("", "value", "TEXT"),
    ])
    er_entity(ax, 13.5, 8.6, 4.8, 2.0, "samost_queue", [
        ("PK", "id", "BIGINT"),
        ("FK", "cm", "BIGINT"),
        ("", "status", "VARCHAR"),
        ("", "timecreated", "BIGINT"),
    ])
    er_entity(ax, 7.5, 4.6, 4.8, 2.6, "samost_files", [
        ("PK", "id", "BIGINT"),
        ("FK", "cm", "BIGINT"),
        ("FK", "userid", "BIGINT"),
        ("FK", "fileid", "BIGINT"),
        ("", "textcontent", "TEXT"),
    ])
    er_entity(ax, 13.5, 4.6, 4.8, 2.6, "samost_results", [
        ("PK", "id", "BIGINT"),
        ("FK", "cm", "BIGINT"),
        ("FK", "userid1", "BIGINT"),
        ("FK", "userid2", "BIGINT"),
        ("", "similarity", "DECIMAL"),
    ])
    er_entity(ax, 10.3, 1.0, 5.4, 2.5, "samost_feedback", [
        ("PK", "id", "BIGINT"),
        ("FK", "resultid", "BIGINT"),
        ("FK", "userid", "BIGINT"),
        ("", "comment", "TEXT"),
    ])

    # Ортогональные связи — через точки на правой кромке Moodle-таблиц и левой кромке плагин-таблиц.
    def ortho_relation(start_entity_right_x, start_y,
                       via_x,
                       end_entity_left_x, end_y,
                       left_kind, right_kind):
        """Рисует L-путь: start → (via_x, start_y) → (via_x, end_y) → end."""
        x0 = start_entity_right_x
        x1 = via_x
        x2 = end_entity_left_x
        ax.plot([x0 + 0.2, x1], [start_y, start_y], color=BLACK, linewidth=1.2, zorder=2)
        ax.plot([x1, x1], [start_y, end_y],          color=BLACK, linewidth=1.2, zorder=2)
        ax.plot([x1, x2 - 0.2], [end_y, end_y],      color=BLACK, linewidth=1.2, zorder=2)
        crow_foot(ax, x0, start_y, "left", left_kind)
        crow_foot(ax, x2, end_y, "right", right_kind)

    # mdl_user → samost_files (1:M) — по шине x=6.8
    ortho_relation(4.9, 9.0, 5.5, 7.5, 5.3, "one", "many")
    # mdl_user → samost_results (1:M) на userid1/userid2 логически → шина x=6.2 (выше других)
    ortho_relation(4.9, 9.3, 6.2, 13.5, 5.3, "one", "many")
    # mdl_assign → samost_config (1:M) — шина x=5.7
    ortho_relation(4.9, 6.0, 5.7, 7.5, 9.3, "one", "many")
    # mdl_assign → samost_queue (1:M) — шина x=6.5
    ortho_relation(4.9, 5.7, 6.5, 13.5, 9.3, "one", "many")
    # mdl_assign → samost_files (1:M) — шина x=5.9
    ortho_relation(4.9, 6.3, 5.9, 7.5, 6.8, "one", "many")
    # mdl_assign → samost_results (1:M) — шина x=6.3
    ortho_relation(4.9, 6.6, 6.3, 13.5, 6.8, "one", "many")
    # mdl_files → samost_files (1:1) — шина x=5.5
    ortho_relation(4.9, 3.0, 5.5, 7.5, 4.8, "one", "one")
    # samost_results → samost_feedback (1:M) — прямо вниз
    ax.plot([15.5, 15.5], [4.6, 3.0], color=BLACK, linewidth=1.2)
    ax.plot([15.5, 13.0], [3.0, 3.0], color=BLACK, linewidth=1.2)
    crow_foot(ax, 15.5, 4.6, "right", "one")  # сверху от линии, rotated visually
    crow_foot(ax, 13.0, 3.0, "right", "many")

    # Легенда
    ax.add_patch(FancyBboxPatch((18.8, 1.0), 3.0, 3.5,
                                   boxstyle="round,pad=0.03,rounding_size=0.1",
                                   facecolor=CREAM, edgecolor=NAVY, linewidth=1))
    ax.text(20.3, 4.1, "Легенда", ha="center", va="center",
            fontsize=11, fontweight="bold", color=NAVY)
    ax.add_patch(Rectangle((19.1, 3.4), 0.55, 0.3, facecolor=GOLD, edgecolor="none"))
    ax.text(19.8, 3.55, "PK — первичный ключ", fontsize=9.5, va="center")
    ax.add_patch(Rectangle((19.1, 2.85), 0.55, 0.3, facecolor=BLUE_LIGHT, edgecolor="none"))
    ax.text(19.8, 3.0, "FK — внешний ключ", fontsize=9.5, va="center")
    ax.plot([19.2, 19.55], [2.35, 2.35], color=BLACK, linewidth=1.4)
    ax.plot([19.375, 19.375], [2.22, 2.48], color=BLACK, linewidth=1.4)
    ax.text(19.8, 2.35, "«|» — один", fontsize=9.5, va="center")
    ax.plot([19.2, 19.4], [1.75, 1.75], color=BLACK, linewidth=1.2)
    ax.plot([19.2, 19.4], [1.75, 1.6], color=BLACK, linewidth=1.2)
    ax.plot([19.2, 19.4], [1.75, 1.9], color=BLACK, linewidth=1.2)
    ax.text(19.8, 1.75, "«\u2720» — много", fontsize=9.5, va="center")
    ax.text(19.8, 1.25, "(crow's foot)", fontsize=8.5, color=GRAY,
            va="center", style="italic")

    _save(fig, "pic_3_2_er.png")


# =============================================================
# Рисунок 3.3 — Макет страницы отчёта
# =============================================================
def make_report_mockup():
    fig, ax = plt.subplots(figsize=(14, 9))
    ax.set_xlim(0, 20)
    ax.set_ylim(0, 12)
    ax.axis("off")

    # Браузерная рамка
    ax.add_patch(FancyBboxPatch((0.3, 0.3), 19.4, 11.4,
                                   boxstyle="round,pad=0.05,rounding_size=0.15",
                                   facecolor=WHITE, edgecolor=GRAY, linewidth=1.5))
    # Верхняя панель Moodle
    ax.add_patch(Rectangle((0.3, 10.7), 19.4, 1.0, facecolor=NAVY,
                            edgecolor=NAVY))
    ax.text(0.8, 11.2, "Moodle · Курс: Архитектура ПО · Задание: Эссе 3",
            fontsize=11, color=WHITE, va="center")
    ax.text(19.2, 11.2, "Преподаватель ▾", fontsize=10, color=WHITE,
            va="center", ha="right")

    # Заголовок отчёта
    ax.text(0.8, 10.1, "Проверка самостоятельности — отчёт",
            fontsize=16, fontweight="bold", color=NAVY, va="center")

    # Карточки метрик
    metrics = [
        ("28", "работ", GOLD),
        ("378", "пар", GOLD),
        ("3", "красных (>60%)", RED_LIGHT),
        ("14", "жёлтых (20–60%)", YELLOW_LIGHT),
    ]
    for i, (big, small, col) in enumerate(metrics):
        x = 0.8 + i * 4.7
        ax.add_patch(FancyBboxPatch((x, 8.6), 4.3, 1.2,
                                      boxstyle="round,pad=0.02,rounding_size=0.1",
                                      facecolor=col, edgecolor=NAVY, linewidth=1))
        ax.text(x + 2.15, 9.35, big, ha="center", va="center",
                fontsize=20, fontweight="bold", color=NAVY)
        ax.text(x + 2.15, 8.85, small, ha="center", va="center",
                fontsize=10, color=NAVY)

    # Матрица пар
    ax.text(0.8, 8.1, "Матрица попарной схожести", fontsize=12,
            fontweight="bold", color=NAVY, va="center")

    cell_size = 0.42
    n = 12  # показываем 12×12 для компактности
    m_x = 1.2
    m_y = 7.6
    import random
    random.seed(7)
    names = [f"S{i+1}" for i in range(n)]
    # Матрица значений, симметричная
    matrix = [[0] * n for _ in range(n)]
    for i in range(n):
        for j in range(i + 1, n):
            v = random.random()
            matrix[i][j] = v
            matrix[j][i] = v
    # Заставим несколько высоких для демонстрации
    for (i, j, v) in [(2, 5, 0.78), (7, 9, 0.65), (4, 11, 0.82), (0, 3, 0.35)]:
        matrix[i][j] = matrix[j][i] = v

    for i in range(n):
        ax.text(m_x - 0.2, m_y - (i + 0.5) * cell_size, names[i],
                ha="right", va="center", fontsize=8, color=GRAY, family="monospace")
        ax.text(m_x + (i + 0.5) * cell_size, m_y + 0.1, names[i],
                ha="center", va="bottom", fontsize=8, color=GRAY, family="monospace")
        for j in range(n):
            if i == j:
                col = GRAY_LIGHT
            elif matrix[i][j] > 0.6:
                col = "#E74C3C"
            elif matrix[i][j] > 0.2:
                col = "#F1C40F"
            else:
                col = "#2ECC71"
            ax.add_patch(Rectangle((m_x + j * cell_size, m_y - (i + 1) * cell_size),
                                    cell_size, cell_size,
                                    facecolor=col, edgecolor=WHITE, linewidth=0.5))

    # Легенда цветов — под матрицей
    lx = 1.2
    ly = 1.7
    ax.add_patch(FancyBboxPatch((lx, ly), 6.5, 1.1,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=CREAM, edgecolor=NAVY, linewidth=1))
    ax.text(lx + 0.2, ly + 0.85, "Цветовая шкала схожести",
            fontsize=10, fontweight="bold", color=NAVY)
    ax.add_patch(Rectangle((lx + 0.2, ly + 0.25), 0.4, 0.3, facecolor="#2ECC71"))
    ax.text(lx + 0.7, ly + 0.4, "< 20% — шум", fontsize=9, va="center")
    ax.add_patch(Rectangle((lx + 2.3, ly + 0.25), 0.4, 0.3, facecolor="#F1C40F"))
    ax.text(lx + 2.8, ly + 0.4, "20–60% — внимание", fontsize=9, va="center")
    ax.add_patch(Rectangle((lx + 4.9, ly + 0.25), 0.4, 0.3, facecolor="#E74C3C"))
    ax.text(lx + 5.4, ly + 0.4, "> 60% — риск", fontsize=9, va="center")

    # Правая панель: список пар — начинается ниже карточек метрик
    panel_x = 13.6
    panel_y = 1.3
    ax.add_patch(FancyBboxPatch((panel_x, panel_y), 6.0, 7.2,
                                  boxstyle="round,pad=0.03,rounding_size=0.1",
                                  facecolor=GOLD_LIGHT, edgecolor=NAVY, linewidth=1))
    ax.text(panel_x + 3.0, panel_y + 6.8, "Требуют внимания",
            ha="center", fontsize=11, fontweight="bold", color=NAVY)

    red_pairs = [
        ("Иванов И. ↔ Петров П.", "82%"),
        ("Сидоров С. ↔ Кузнецов А.", "78%"),
        ("Смирнов М. ↔ Волков Д.", "65%"),
        ("Попов Г. ↔ Мельник О.", "63%"),
    ]
    for i, (pair, pct) in enumerate(red_pairs):
        y = panel_y + 5.8 - i * 1.15
        ax.add_patch(FancyBboxPatch((panel_x + 0.2, y - 0.5), 5.6, 0.9,
                                      boxstyle="round,pad=0.02,rounding_size=0.06",
                                      facecolor=WHITE, edgecolor=RED, linewidth=1.2))
        ax.text(panel_x + 0.4, y + 0.05, pair, fontsize=10, va="center")
        ax.text(panel_x + 5.6, y + 0.05, pct, fontsize=11, fontweight="bold",
                color=RED, va="center", ha="right")
        ax.text(panel_x + 0.4, y - 0.3, "открыть детализацию →", fontsize=8.5,
                color=BLUE, va="center", style="italic")

    # Кнопки внизу — в левой половине (под легендой)
    for i, (label, col) in enumerate([
        ("Скачать CSV", GOLD),
        ("Пометить проверенным", GREEN_LIGHT),
        ("Перезапустить проверку", YELLOW_LIGHT),
    ]):
        x = 1.2 + i * 4.1
        ax.add_patch(FancyBboxPatch((x, 0.55), 3.9, 0.7,
                                      boxstyle="round,pad=0.02,rounding_size=0.08",
                                      facecolor=col, edgecolor=NAVY, linewidth=1))
        ax.text(x + 1.95, 0.9, label, ha="center", va="center",
                fontsize=10, fontweight="bold", color=NAVY)

    _save(fig, "pic_3_3_report_mockup.png")


# =============================================================
# Рисунок 4.1 — Админ-панель настроек плагина
# =============================================================
def make_admin_panel():
    fig, ax = plt.subplots(figsize=(13, 8))
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 11)
    ax.axis("off")

    # Рамка браузера
    ax.add_patch(FancyBboxPatch((0.3, 0.3), 17.4, 10.4,
                                  boxstyle="round,pad=0.05,rounding_size=0.15",
                                  facecolor=WHITE, edgecolor=GRAY, linewidth=1.5))
    ax.add_patch(Rectangle((0.3, 9.7), 17.4, 1.0, facecolor=NAVY))
    ax.text(0.8, 10.2, "Moodle · Управление сайтом · Плагины › Плагиаризм › Samost",
            fontsize=11, color=WHITE, va="center")

    ax.text(0.8, 9.0, "Настройки плагина «Проверка самостоятельности»",
            fontsize=15, fontweight="bold", color=NAVY, va="center")

    fields = [
        ("Включить плагин по умолчанию", "✔", "checkbox"),
        ("Порог «высокая схожесть», %", "60", "input"),
        ("Порог «средняя схожесть», %", "20", "input"),
        ("Размер шингла (слов)",         "7", "input"),
        ("k для Winnowing",              "5", "input"),
        ("Окно w для Winnowing",         "4", "input"),
        ("Размер батча очереди",        "100", "input"),
        ("Период запуска задачи (мин)",   "1", "input"),
        ("Максимальный размер файла, МБ", "20", "input"),
        ("Вести журнал аудита",           "✔", "checkbox"),
    ]
    y_start = 8.2
    for i, (label, value, kind) in enumerate(fields):
        y = y_start - i * 0.65
        ax.text(0.8, y, label, fontsize=11, color=BLACK, va="center")
        if kind == "checkbox":
            ax.add_patch(Rectangle((10.5, y - 0.18), 0.4, 0.4,
                                    facecolor=GOLD, edgecolor=NAVY, linewidth=1.2))
            ax.text(10.7, y, value, ha="center", va="center",
                    fontsize=13, fontweight="bold", color=NAVY)
        else:
            ax.add_patch(FancyBboxPatch((10.5, y - 0.22), 3.0, 0.45,
                                          boxstyle="round,pad=0.02,rounding_size=0.05",
                                          facecolor=WHITE, edgecolor=NAVY, linewidth=1))
            ax.text(10.7, y, value, fontsize=11, color=BLACK, va="center",
                    family="monospace")

    # Сохранить / отмена
    ax.add_patch(FancyBboxPatch((14.2, 0.9), 2.8, 0.7,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=GOLD, edgecolor=NAVY, linewidth=1))
    ax.text(15.6, 1.25, "Сохранить", ha="center", va="center",
            fontsize=11, fontweight="bold", color=NAVY)

    _save(fig, "pic_4_1_admin_panel.png")


# =============================================================
# Рисунок 4.2 — Включение плагина в настройках задания
# =============================================================
def make_task_settings():
    fig, ax = plt.subplots(figsize=(13, 7))
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 10)
    ax.axis("off")

    ax.add_patch(FancyBboxPatch((0.3, 0.3), 17.4, 9.4,
                                  boxstyle="round,pad=0.05,rounding_size=0.15",
                                  facecolor=WHITE, edgecolor=GRAY, linewidth=1.5))
    ax.add_patch(Rectangle((0.3, 8.7), 17.4, 1.0, facecolor=NAVY))
    ax.text(0.8, 9.2, "Moodle · Курс › Настройки задания «Эссе 3»",
            fontsize=11, color=WHITE, va="center")

    ax.text(0.8, 8.0, "Плагин «Проверка самостоятельности»",
            fontsize=14, fontweight="bold", color=NAVY, va="center")

    # чекбокс "Включить"
    ax.add_patch(Rectangle((0.8, 7.1), 0.4, 0.4,
                            facecolor=GOLD, edgecolor=NAVY, linewidth=1.2))
    ax.text(1.0, 7.3, "✔", ha="center", va="center",
            fontsize=13, fontweight="bold", color=NAVY)
    ax.text(1.5, 7.3, "Включить проверку самостоятельности для этого задания",
            fontsize=11, va="center")

    # Пороги
    ax.text(0.8, 6.3, "Порог «высокая схожесть»:", fontsize=11, va="center")
    ax.add_patch(FancyBboxPatch((7.5, 6.1), 3, 0.45,
                                  boxstyle="round,pad=0.02,rounding_size=0.05",
                                  facecolor=WHITE, edgecolor=NAVY, linewidth=1))
    ax.text(7.7, 6.3, "60", fontsize=11, va="center", family="monospace")
    ax.text(10.7, 6.3, "%", fontsize=11, va="center")

    ax.text(0.8, 5.55, "Порог «средняя схожесть»:", fontsize=11, va="center")
    ax.add_patch(FancyBboxPatch((7.5, 5.35), 3, 0.45,
                                  boxstyle="round,pad=0.02,rounding_size=0.05",
                                  facecolor=WHITE, edgecolor=NAVY, linewidth=1))
    ax.text(7.7, 5.55, "20", fontsize=11, va="center", family="monospace")
    ax.text(10.7, 5.55, "%", fontsize=11, va="center")

    # Режимы
    ax.text(0.8, 4.7, "Алгоритмы, применяемые для этого задания:",
            fontsize=11, va="center")
    ax.add_patch(Rectangle((0.8, 3.8), 0.4, 0.4, facecolor=GOLD,
                            edgecolor=NAVY, linewidth=1.2))
    ax.text(1.0, 4.0, "✔", ha="center", va="center",
            fontsize=13, fontweight="bold", color=NAVY)
    ax.text(1.5, 4.0, "Шинглы + коэффициент Жаккара (для текста)",
            fontsize=11, va="center")

    ax.add_patch(Rectangle((0.8, 3.2), 0.4, 0.4, facecolor=GOLD,
                            edgecolor=NAVY, linewidth=1.2))
    ax.text(1.0, 3.4, "✔", ha="center", va="center",
            fontsize=13, fontweight="bold", color=NAVY)
    ax.text(1.5, 3.4, "Winnowing (для исходного кода)",
            fontsize=11, va="center")

    ax.add_patch(Rectangle((0.8, 2.6), 0.4, 0.4, facecolor=WHITE,
                            edgecolor=NAVY, linewidth=1.2))
    ax.text(1.5, 2.8, "Включить OCR для изображений (бета)",
            fontsize=11, va="center", color=GRAY)

    # Уведомления
    ax.text(0.8, 1.85, "Уведомлять о завершении проверки:",
            fontsize=11, va="center")
    ax.add_patch(Rectangle((0.8, 1.0), 0.4, 0.4, facecolor=GOLD,
                            edgecolor=NAVY, linewidth=1.2))
    ax.text(1.0, 1.2, "✔", ha="center", va="center",
            fontsize=13, fontweight="bold", color=NAVY)
    ax.text(1.5, 1.2, "email автору задания",
            fontsize=11, va="center")

    # Сохранить
    ax.add_patch(FancyBboxPatch((14.2, 0.6), 2.8, 0.7,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=GOLD, edgecolor=NAVY, linewidth=1))
    ax.text(15.6, 0.95, "Сохранить", ha="center", va="center",
            fontsize=11, fontweight="bold", color=NAVY)

    _save(fig, "pic_4_2_task_settings.png")


# =============================================================
# Рисунок 4.3 — Результаты PHPUnit
# =============================================================
def make_phpunit_output():
    fig, ax = plt.subplots(figsize=(14, 7))
    ax.set_xlim(0, 20)
    ax.set_ylim(0, 10)
    ax.axis("off")

    # Окно терминала
    ax.add_patch(FancyBboxPatch((0.3, 0.3), 19.4, 9.4,
                                  boxstyle="round,pad=0.03,rounding_size=0.1",
                                  facecolor="#1E1E1E", edgecolor="#333", linewidth=1))
    ax.add_patch(Rectangle((0.3, 9.0), 19.4, 0.7, facecolor="#3C3C3C"))
    # Кнопочки макоси
    for i, col in enumerate(["#FF5F56", "#FFBD2E", "#27C93F"]):
        ax.add_patch(Circle((0.7 + i * 0.4, 9.35), 0.12, facecolor=col, edgecolor="none"))
    ax.text(10, 9.35, "terminal — PHPUnit", ha="center", va="center",
            fontsize=10, color="#DDD")

    lines = [
        ("$ vendor/bin/phpunit plagiarism/samost/tests", "#DDD"),
        ("", ""),
        ("PHPUnit 10.5 by Sebastian Bergmann and contributors.", "#888"),
        ("Configuration: plagiarism/samost/phpunit.xml", "#888"),
        ("", ""),
        ("Testing plagiarism_samost_test_suite", "#DDD"),
        ("...........  (12/12) analyser_test", "#2ECC71"),
        ("...........  (10/10) text_extractor_test", "#2ECC71"),
        (".......      (7/7)   queue_test", "#2ECC71"),
        ("", ""),
        ("Time: 00:08.342, Memory: 68.50 MB", "#DDD"),
        ("", ""),
        ("OK (29 tests, 97 assertions)", "#27C93F"),
        ("", ""),
        ("Coverage report:", "#DDD"),
        ("  Lines:     58.3% (2542/4362)", "#F1C40F"),
        ("  Methods:   73.1% (87/119)", "#2ECC71"),
        ("  Classes:   80.0% (8/10)", "#2ECC71"),
        ("", ""),
        ("$ _", "#DDD"),
    ]
    y = 8.5
    for text, col in lines:
        ax.text(0.7, y, text, fontsize=11, color=col or "#DDD",
                family="monospace", va="top")
        y -= 0.38

    _save(fig, "pic_4_3_phpunit.png")


# =============================================================
# Рисунок 4.4 — Таблица пар (крупный план)
# =============================================================
def make_pairs_table():
    make_report_mockup_big()


def make_report_mockup_big():
    fig, ax = plt.subplots(figsize=(14, 9))
    ax.set_xlim(0, 20)
    ax.set_ylim(0, 12)
    ax.axis("off")

    # Используем тот же стиль, но крупнее матрицу
    ax.add_patch(FancyBboxPatch((0.3, 0.3), 19.4, 11.4,
                                  boxstyle="round,pad=0.05,rounding_size=0.15",
                                  facecolor=WHITE, edgecolor=GRAY, linewidth=1.5))
    ax.add_patch(Rectangle((0.3, 10.8), 19.4, 0.9, facecolor=NAVY))
    ax.text(0.8, 11.25, "Moodle · Отчёт плагина samost · Задание «Эссе 3»",
            fontsize=11, color=WHITE, va="center")

    ax.text(0.8, 10.1, "Попарная схожесть работ",
            fontsize=15, fontweight="bold", color=NAVY, va="center")

    # Таблица пар как список
    headers = ["Студент A", "Студент B", "Жаккар, %", "Winnowing, %", "Статус"]
    col_x = [0.8, 5.0, 9.2, 12.0, 15.0]
    col_w = [4.2, 4.2, 2.8, 2.8, 4.4]

    # Заголовок
    ax.add_patch(Rectangle((0.5, 8.9), 19.0, 0.8, facecolor=NAVY, edgecolor=NAVY))
    for i, h in enumerate(headers):
        ax.text(col_x[i] + 0.2, 9.3, h, fontsize=11, fontweight="bold",
                color=WHITE, va="center")

    rows = [
        ("Иванов И. А.",     "Петров П. Д.",      "82", "—",  "Высокая схожесть", RED),
        ("Кузнецова А. М.",  "Смирнов О. Б.",     "78", "—",  "Высокая схожесть", RED),
        ("Волков Д. С.",     "Попов Г. В.",       "65", "—",  "Высокая схожесть", RED),
        ("Козлов Е. К.",     "Тарасова Н. И.",    "63", "—",  "Высокая схожесть", RED),
        ("Соколов М. О.",    "Новикова А. Р.",    "—",  "71", "Высокая схожесть", RED),
        ("Михайлов В. П.",   "Лебедев И. Д.",     "48", "—",  "Средняя",         YELLOW),
        ("Ковалёв Р. А.",    "Морозов Т. Е.",     "39", "—",  "Средняя",         YELLOW),
        ("Николаев К. С.",   "Богданов Е. В.",    "22", "—",  "Средняя",         YELLOW),
        ("Андреев А. П.",    "Зайцев Ю. К.",      "17", "—",  "Незначительная",  GREEN),
        ("Степанова М. В.",  "Павлова О. С.",     "12", "—",  "Незначительная",  GREEN),
        ("Мельник К. С.",    "Гусев В. Д.",        "9", "—",  "Незначительная",  GREEN),
    ]
    for i, (a, b, j, w, st, stc) in enumerate(rows):
        y = 8.5 - i * 0.55
        row_col = WHITE if i % 2 == 0 else CREAM
        ax.add_patch(Rectangle((0.5, y - 0.25), 19.0, 0.5,
                                facecolor=row_col, edgecolor=GRAY_LIGHT, linewidth=0.5))
        ax.text(col_x[0] + 0.2, y, a, fontsize=10, va="center")
        ax.text(col_x[1] + 0.2, y, b, fontsize=10, va="center")
        ax.text(col_x[2] + 0.2, y, j, fontsize=10, va="center", family="monospace")
        ax.text(col_x[3] + 0.2, y, w, fontsize=10, va="center", family="monospace")
        # бейдж статуса
        ax.add_patch(FancyBboxPatch((col_x[4] + 0.1, y - 0.17), 3.5, 0.34,
                                      boxstyle="round,pad=0.02,rounding_size=0.08",
                                      facecolor=stc, edgecolor="none", alpha=0.6))
        ax.text(col_x[4] + 1.85, y, st, fontsize=9.5, va="center",
                ha="center", color=BLACK, fontweight="bold")

    # Подвал
    ax.text(0.8, 1.0, "Показано 11 из 378 пар. Сортировка по убыванию схожести.",
            fontsize=9.5, color=GRAY, va="center", style="italic")

    _save(fig, "pic_4_4_pairs_table.png")


# =============================================================
# Рисунок 4.5 — Раскрытая пара с подсветкой
# =============================================================
def make_pair_detail():
    fig, ax = plt.subplots(figsize=(15, 9))
    ax.set_xlim(0, 22)
    ax.set_ylim(0, 12)
    ax.axis("off")

    ax.add_patch(FancyBboxPatch((0.3, 0.3), 21.4, 11.4,
                                  boxstyle="round,pad=0.05,rounding_size=0.15",
                                  facecolor=WHITE, edgecolor=GRAY, linewidth=1.5))
    ax.add_patch(Rectangle((0.3, 10.8), 21.4, 0.9, facecolor=NAVY))
    ax.text(0.8, 11.25, "Moodle · samost · Пара: Иванов И. А. ↔ Петров П. Д.",
            fontsize=11, color=WHITE, va="center")

    # Метаданные пары
    ax.text(0.8, 10.1, "Иванов И. А.  ↔  Петров П. Д.",
            fontsize=14, fontweight="bold", color=NAVY, va="center")
    ax.add_patch(FancyBboxPatch((16.5, 9.7), 4.9, 0.8,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=RED, edgecolor=NAVY, linewidth=1))
    ax.text(18.95, 10.1, "Жаккар: 82% (высокая)",
            ha="center", va="center", fontsize=12,
            fontweight="bold", color=WHITE)

    # Две колонки текста
    left_x = 0.6
    right_x = 11.3
    col_w = 10.2
    col_h = 8.2
    col_y = 1.2

    for x, name in [(left_x, "Иванов И. А. — essay3.docx"),
                    (right_x, "Петров П. Д. — essay3.docx")]:
        ax.add_patch(FancyBboxPatch((x, col_y), col_w, col_h,
                                      boxstyle="round,pad=0.03,rounding_size=0.1",
                                      facecolor=CREAM, edgecolor=NAVY, linewidth=1.2))
        ax.text(x + 0.25, col_y + col_h - 0.35, name, fontsize=10.5,
                fontweight="bold", color=NAVY, va="center")

    # Пара абзацев, совпадающие фрагменты подсвечены
    common_a = ("Микросервисная архитектура разбивает монолит на независимые\n"
                "сервисы, каждый из которых отвечает за один ограниченный\n"
                "контекст и взаимодействует через HTTP или асинхронные\n"
                "очереди сообщений.")
    common_b = ("Микросервисная архитектура разбивает монолит на независимые\n"
                "сервисы, каждый из которых отвечает за один ограниченный\n"
                "контекст и взаимодействует через HTTP или очереди сообщений.")
    unique_a = ("Автор добавляет уточнение про event-driven сценарии\n"
                "и приводит пример из практики платёжной системы,\n"
                "где сервисы биллинга и подтверждения работают независимо.")
    unique_b = ("Далее разбирается частный случай Saga-паттерна для\n"
                "распределённых транзакций в торговой площадке и\n"
                "приводится сравнение с двухфазным коммитом.")

    # Подсветка совпадающих фрагментов
    for (x, text_yellow, text_rest) in [
        (left_x,  common_a, unique_a),
        (right_x, common_b, unique_b),
    ]:
        ax.add_patch(FancyBboxPatch((x + 0.25, col_y + 5.4), col_w - 0.5, 2.8,
                                      boxstyle="round,pad=0.02,rounding_size=0.05",
                                      facecolor="#FFD966", edgecolor="#E8AA1E",
                                      linewidth=1))
        ax.text(x + 0.4, col_y + 6.8, text_yellow, fontsize=10.5,
                color=BLACK, va="center", family="serif")
        ax.text(x + 0.4, col_y + 3.5, text_rest, fontsize=10.5,
                color=BLACK, va="center", family="serif")

    # Легенда подсветки
    ax.add_patch(Rectangle((0.8, 0.7), 0.5, 0.3, facecolor="#FFD966",
                            edgecolor="#E8AA1E"))
    ax.text(1.5, 0.85, "жёлтый — совпадающий фрагмент", fontsize=10, va="center")

    # Кнопки действий
    ax.add_patch(FancyBboxPatch((13.5, 0.6), 2.8, 0.7,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=GREEN_LIGHT, edgecolor=NAVY, linewidth=1))
    ax.text(14.9, 0.95, "Самостоятельно", ha="center", va="center",
            fontsize=10, fontweight="bold", color=NAVY)
    ax.add_patch(FancyBboxPatch((16.6, 0.6), 2.3, 0.7,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=RED_LIGHT, edgecolor=NAVY, linewidth=1))
    ax.text(17.75, 0.95, "Заимствование", ha="center", va="center",
            fontsize=10, fontweight="bold", color=NAVY)
    ax.add_patch(FancyBboxPatch((19.1, 0.6), 2.2, 0.7,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=GOLD, edgecolor=NAVY, linewidth=1))
    ax.text(20.2, 0.95, "Отметить", ha="center", va="center",
            fontsize=10, fontweight="bold", color=NAVY)

    _save(fig, "pic_4_5_pair_detail.png")


# =============================================================
# Рисунок 4.6 — Панель настроек порогов
# =============================================================
def make_thresholds_panel():
    fig, ax = plt.subplots(figsize=(13, 7))
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 10)
    ax.axis("off")

    ax.add_patch(FancyBboxPatch((0.3, 0.3), 17.4, 9.4,
                                  boxstyle="round,pad=0.05,rounding_size=0.15",
                                  facecolor=WHITE, edgecolor=GRAY, linewidth=1.5))
    ax.add_patch(Rectangle((0.3, 8.7), 17.4, 1.0, facecolor=NAVY))
    ax.text(0.8, 9.2, "Moodle · samost › Пороги и цветовая шкала",
            fontsize=11, color=WHITE, va="center")

    ax.text(0.8, 7.9, "Пороги срабатывания",
            fontsize=14, fontweight="bold", color=NAVY, va="center")

    # Слайдер визуализация: полоса от 0 до 100, на ней ползунки
    bar_x = 0.8
    bar_y = 6.0
    bar_w = 16.0
    # Зоны
    th_low = 20
    th_high = 60
    # Зелёная зона
    ax.add_patch(Rectangle((bar_x, bar_y), bar_w * th_low / 100, 0.7,
                            facecolor=GREEN_LIGHT, edgecolor=NAVY, linewidth=1))
    # Жёлтая
    ax.add_patch(Rectangle((bar_x + bar_w * th_low / 100, bar_y),
                            bar_w * (th_high - th_low) / 100, 0.7,
                            facecolor=YELLOW_LIGHT, edgecolor=NAVY, linewidth=1))
    # Красная
    ax.add_patch(Rectangle((bar_x + bar_w * th_high / 100, bar_y),
                            bar_w * (100 - th_high) / 100, 0.7,
                            facecolor=RED_LIGHT, edgecolor=NAVY, linewidth=1))

    ax.text(bar_x + bar_w * 0.1, bar_y + 0.35, "Незначительная",
            ha="center", va="center", fontsize=10, fontweight="bold", color=NAVY)
    ax.text(bar_x + bar_w * 0.4, bar_y + 0.35, "Средняя",
            ha="center", va="center", fontsize=10, fontweight="bold", color=NAVY)
    ax.text(bar_x + bar_w * 0.8, bar_y + 0.35, "Высокая",
            ha="center", va="center", fontsize=10, fontweight="bold", color=NAVY)

    # Ползунки с подписями
    for pct, y_lbl in [(th_low, 6.0), (th_high, 6.0)]:
        x = bar_x + bar_w * pct / 100
        ax.add_patch(Circle((x, y_lbl + 0.35), 0.22,
                            facecolor=NAVY, edgecolor=WHITE, linewidth=2, zorder=5))
        ax.text(x, y_lbl - 0.4, f"{pct}%", ha="center", va="top",
                fontsize=11, fontweight="bold", color=NAVY)

    # Шкала (0-100)
    for t in [0, 20, 40, 60, 80, 100]:
        x = bar_x + bar_w * t / 100
        ax.plot([x, x], [bar_y - 0.05, bar_y - 0.2], color=GRAY, linewidth=1)
        ax.text(x, bar_y - 0.5, str(t), ha="center", va="top",
                fontsize=9, color=GRAY)

    # Параметры
    params = [
        ("Размер шингла (shingle_size)",    "7"),
        ("k-грамма Winnowing",              "5"),
        ("Окно w Winnowing",                "4"),
        ("Batch size очереди",            "100"),
    ]
    for i, (label, val) in enumerate(params):
        y = 4.5 - i * 0.6
        ax.text(0.8, y, label, fontsize=11, va="center")
        ax.add_patch(FancyBboxPatch((11.0, y - 0.22), 2.5, 0.45,
                                      boxstyle="round,pad=0.02,rounding_size=0.05",
                                      facecolor=WHITE, edgecolor=NAVY, linewidth=1))
        ax.text(11.2, y, val, fontsize=11, va="center", family="monospace")

    # Сохранить
    ax.add_patch(FancyBboxPatch((14.2, 0.6), 2.8, 0.7,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=GOLD, edgecolor=NAVY, linewidth=1))
    ax.text(15.6, 0.95, "Сохранить", ha="center", va="center",
            fontsize=11, fontweight="bold", color=NAVY)

    _save(fig, "pic_4_6_thresholds.png")


# =============================================================
# Рисунок 4.7 — CSV-экспорт
# =============================================================
def make_csv_export():
    fig, ax = plt.subplots(figsize=(13, 7))
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 10)
    ax.axis("off")

    # Рамка «Excel»
    ax.add_patch(FancyBboxPatch((0.3, 0.3), 17.4, 9.4,
                                  boxstyle="round,pad=0.05,rounding_size=0.15",
                                  facecolor=WHITE, edgecolor=GRAY, linewidth=1.5))
    ax.add_patch(Rectangle((0.3, 8.7), 17.4, 1.0, facecolor="#217346"))
    ax.text(0.8, 9.2, "Excel · samost_export_2026-04-16.csv",
            fontsize=11, color=WHITE, va="center", fontweight="bold")

    # Заголовки столбцов
    cols = ["A", "B", "C", "D", "E", "F"]
    col_x = [0.8, 3.2, 5.6, 8.0, 10.4, 12.8]
    col_w = 2.4

    for i, c in enumerate(cols):
        ax.add_patch(Rectangle((col_x[i], 7.8), col_w, 0.5,
                                facecolor="#E7E6E6", edgecolor=GRAY_LIGHT))
        ax.text(col_x[i] + col_w / 2, 8.05, c, ha="center", va="center",
                fontsize=10, fontweight="bold", color=GRAY)

    # Ряд заголовка таблицы данных
    headers = ["pair_id", "student_a", "student_b", "jaccard", "winnowing", "status"]
    for i, h in enumerate(headers):
        ax.add_patch(Rectangle((col_x[i], 7.3), col_w, 0.5,
                                facecolor=NAVY, edgecolor=WHITE, linewidth=1))
        ax.text(col_x[i] + col_w / 2, 7.55, h, ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE, family="monospace")

    rows = [
        ("1", "Иванов И.А.", "Петров П.Д.", "0.82", "—",    "high"),
        ("2", "Кузнецова А.М.", "Смирнов О.Б.", "0.78", "—", "high"),
        ("3", "Волков Д.С.", "Попов Г.В.", "0.65", "—",    "high"),
        ("4", "Соколов М.О.", "Новикова А.Р.", "—", "0.71", "high"),
        ("5", "Михайлов В.П.", "Лебедев И.Д.", "0.48", "—", "medium"),
        ("6", "Ковалёв Р.А.", "Морозов Т.Е.", "0.39", "—",  "medium"),
        ("7", "Андреев А.П.", "Зайцев Ю.К.", "0.17", "—",   "low"),
        ("8", "Степанова М.В.", "Павлова О.С.", "0.12", "—","low"),
    ]
    for i, row in enumerate(rows):
        y = 6.8 - i * 0.5
        row_col = WHITE if i % 2 == 0 else "#F6F6F6"
        for j, v in enumerate(row):
            ax.add_patch(Rectangle((col_x[j], y - 0.25), col_w, 0.5,
                                    facecolor=row_col, edgecolor=GRAY_LIGHT,
                                    linewidth=0.5))
            if j == 5 and v == "high":
                ax.add_patch(Rectangle((col_x[j] + 0.1, y - 0.17), col_w - 0.2, 0.34,
                                        facecolor=RED_LIGHT, edgecolor="none",
                                        alpha=0.7))
            elif j == 5 and v == "medium":
                ax.add_patch(Rectangle((col_x[j] + 0.1, y - 0.17), col_w - 0.2, 0.34,
                                        facecolor=YELLOW_LIGHT, edgecolor="none",
                                        alpha=0.7))
            elif j == 5 and v == "low":
                ax.add_patch(Rectangle((col_x[j] + 0.1, y - 0.17), col_w - 0.2, 0.34,
                                        facecolor=GREEN_LIGHT, edgecolor="none",
                                        alpha=0.7))
            ax.text(col_x[j] + col_w / 2, y, v, ha="center", va="center",
                    fontsize=10, color=BLACK,
                    family="monospace" if j in (0, 3, 4, 5) else "sans-serif")

    ax.text(0.8, 0.8, "UTF-8 with BOM · разделитель «;» (совместимость с Excel)",
            fontsize=10, color=GRAY, style="italic", va="center")

    _save(fig, "pic_4_7_csv_export.png")


# =============================================================
# main
# =============================================================
if __name__ == "__main__":
    make_bpmn_as_is()
    make_bpmn_to_be()
    make_story_map()
    make_use_case()
    make_architecture()
    make_er()
    make_report_mockup()
    make_admin_panel()
    make_task_settings()
    make_phpunit_output()
    make_report_mockup_big()
    make_pair_detail()
    make_thresholds_panel()
    make_csv_export()
    print("\n✅ Все рисунки сохранены в", OUT)
