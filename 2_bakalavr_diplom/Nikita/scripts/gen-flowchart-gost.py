#!/usr/bin/env python3
"""
Блок-схема обработки заявки на ремонт по ГОСТ 19.701-90.
Формы:
    овал         -> начало/конец
    прямоугольник-> процесс/действие
    ромб         -> решение
    параллелограмм-> ввод/вывод данных
"""
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.patches as patches
from matplotlib.path import Path as MplPath

OUTPUT = Path("/home/washka/project/diplom/Nikita/images/A.1_request_flow.png")

LINE_COLOR = "#000000"
FILL_COLOR = "#FFFFFF"
TEXT_COLOR = "#000000"
LINE_WIDTH = 1.6
FONT_SIZE = 11
ARROW_KW = dict(arrowstyle="-|>,head_length=0.35,head_width=0.22",
                color=LINE_COLOR, linewidth=LINE_WIDTH, mutation_scale=1)

W = 3.4
H = 0.9
DH = 0.25
DV = 0.65


def terminator(ax, x, y, text):
    e = patches.FancyBboxPatch(
        (x - W / 2, y - H / 2), W, H,
        boxstyle="round,pad=0.02,rounding_size=0.45",
        linewidth=LINE_WIDTH, edgecolor=LINE_COLOR, facecolor=FILL_COLOR,
    )
    ax.add_patch(e)
    ax.text(x, y, text, ha="center", va="center", fontsize=FONT_SIZE,
            color=TEXT_COLOR)


def process(ax, x, y, text, w=W, h=H):
    r = patches.Rectangle((x - w / 2, y - h / 2), w, h,
                          linewidth=LINE_WIDTH, edgecolor=LINE_COLOR,
                          facecolor=FILL_COLOR)
    ax.add_patch(r)
    ax.text(x, y, text, ha="center", va="center", fontsize=FONT_SIZE,
            color=TEXT_COLOR, wrap=True)


def io_data(ax, x, y, text, w=W, h=H):
    skew = 0.35
    poly = [
        (x - w / 2 + skew, y - h / 2),
        (x + w / 2,        y - h / 2),
        (x + w / 2 - skew, y + h / 2),
        (x - w / 2,        y + h / 2),
    ]
    p = patches.Polygon(poly, closed=True, linewidth=LINE_WIDTH,
                        edgecolor=LINE_COLOR, facecolor=FILL_COLOR)
    ax.add_patch(p)
    ax.text(x, y, text, ha="center", va="center", fontsize=FONT_SIZE,
            color=TEXT_COLOR)


def decision(ax, x, y, text, w=3.6, h=1.6):
    poly = [(x, y - h / 2), (x + w / 2, y), (x, y + h / 2), (x - w / 2, y)]
    p = patches.Polygon(poly, closed=True, linewidth=LINE_WIDTH,
                        edgecolor=LINE_COLOR, facecolor=FILL_COLOR)
    ax.add_patch(p)
    ax.text(x, y, text, ha="center", va="center", fontsize=FONT_SIZE,
            color=TEXT_COLOR)


def arrow(ax, x1, y1, x2, y2, label=None, label_offset=(0.12, 0.0)):
    a = patches.FancyArrowPatch((x1, y1), (x2, y2), **ARROW_KW)
    ax.add_patch(a)
    if label:
        ax.text((x1 + x2) / 2 + label_offset[0],
                (y1 + y2) / 2 + label_offset[1],
                label, fontsize=FONT_SIZE - 1, color=TEXT_COLOR)


def hline(ax, x1, x2, y):
    ax.add_line(plt.Line2D([x1, x2], [y, y], color=LINE_COLOR,
                           linewidth=LINE_WIDTH))


def vline(ax, x, y1, y2):
    ax.add_line(plt.Line2D([x, x], [y1, y2], color=LINE_COLOR,
                           linewidth=LINE_WIDTH))


def main():
    fig, ax = plt.subplots(figsize=(10, 14), dpi=200)
    ax.set_xlim(0, 12)
    ax.set_ylim(0, 22)
    ax.set_aspect("equal")
    ax.axis("off")

    cx = 6.0

    # 1. Начало
    y = 21.0
    terminator(ax, cx, y, "Начало")
    arrow(ax, cx, y - H / 2, cx, y - H / 2 - DV)

    # 2. Обнаружение неисправности
    y -= H + DV
    process(ax, cx, y, "Оператор обнаруживает\nнеисправность оборудования")
    arrow(ax, cx, y - H / 2, cx, y - H / 2 - DV)

    # 3. Создание заявки
    y -= H + DV
    io_data(ax, cx, y, "Регистрация заявки\n(идентификация оборудования)")
    arrow(ax, cx, y - H / 2, cx, y - H / 2 - DV)

    # 4. Просмотр инженером
    y -= H + DV
    process(ax, cx, y, "Инженер ОГМ просматривает заявку,\nоценивает приоритет")
    arrow(ax, cx, y - H / 2, cx, y - H / 2 - DV)

    # 5. Назначение техника
    y -= H + DV
    process(ax, cx, y, "Назначение исполнителя (техника)")
    arrow(ax, cx, y - H / 2, cx, y - H / 2 - DV)

    # 6. Техник принимает в работу
    y -= H + DV
    process(ax, cx, y, "Техник принимает заявку в работу")
    arrow(ax, cx, y - H / 2, cx, y - H / 2 - DV - 0.3)

    # 7. Решение: запчасти есть?
    y_dec = y - H / 2 - DV - 0.3 - 0.8
    decision(ax, cx, y_dec, "Запчасти\nна складе?")
    # Yes — вниз
    arrow(ax, cx, y_dec - 0.8, cx, y_dec - 0.8 - DV, label="Да")

    # No — вправо к запросу запчастей
    side_x = cx + 4.3
    hline(ax, cx + 1.8, side_x, y_dec)
    ax.text(cx + 1.95, y_dec + 0.18, "Нет", fontsize=FONT_SIZE - 1,
            color=TEXT_COLOR)

    # запрос запчастей
    process(ax, side_x, y_dec, "Формирование\nзаказа запчастей", w=2.8)
    # вниз и петля обратно
    arrow(ax, side_x, y_dec - H / 2, side_x, y_dec - H / 2 - 0.7)
    process(ax, side_x, y_dec - H / 2 - 0.7 - H / 2, "Поступление\nна склад",
            w=2.8)
    # обратно к решению "запчасти?" (петля)
    arrow(ax, side_x - 1.4, y_dec - H / 2 - 0.7 - H, side_x - 1.4,
          y_dec)

    # 8. Выполнение ремонта
    y_proc = y_dec - 0.8 - DV - H / 2
    process(ax, cx, y_proc, "Выполнение ремонтных работ")
    arrow(ax, cx, y_proc - H / 2, cx, y_proc - H / 2 - DV)

    # 9. Фиксация результатов
    y_fix = y_proc - H - DV
    io_data(ax, cx, y_fix,
            "Фиксация результатов:\nработы, запчасти, время")
    arrow(ax, cx, y_fix - H / 2, cx, y_fix - H / 2 - DV - 0.2)

    # 10. Инженер проверяет качество
    y_chk = y_fix - H / 2 - DV - 0.2 - 0.8
    decision(ax, cx, y_chk, "Качество\nприёмлемо?")
    # Нет - влево обратно к выполнению
    left_x = cx - 4.3
    hline(ax, cx - 1.8, left_x, y_chk)
    ax.text(cx - 2.15, y_chk + 0.18, "Нет", fontsize=FONT_SIZE - 1,
            color=TEXT_COLOR)
    vline(ax, left_x, y_chk, y_proc)
    arrow(ax, left_x, y_proc, cx - W / 2, y_proc)

    # Да - вниз
    arrow(ax, cx, y_chk - 0.8, cx, y_chk - 0.8 - DV, label="Да")

    # 11. Закрытие заявки
    y_close = y_chk - 0.8 - DV - H / 2
    process(ax, cx, y_close, "Закрытие заявки\n(статус «Закрыта»)")
    arrow(ax, cx, y_close - H / 2, cx, y_close - H / 2 - DV)

    # 12. Конец
    y_end = y_close - H - DV
    terminator(ax, cx, y_end, "Конец")

    plt.tight_layout()
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(OUTPUT, dpi=200, bbox_inches="tight",
                pad_inches=0.2, facecolor="white")
    print(f"Saved: {OUTPUT}")


if __name__ == "__main__":
    main()
