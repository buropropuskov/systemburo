#!/usr/bin/env python3
"""
Рисунок 2.1 — Диаграмма вариантов использования (UML 2.5).
Swim-lane компоновка: акторы — колонки, варианты использования — внутри колонок.
"""
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.patches as patches

OUTPUT = Path(
    "/home/washka/project/diplom/Nikita/images/2.2_use_case_diagramma.png"
)

LINE = "#1f2937"
LIGHT = "#cbd5e1"
ACTOR_BG = "#f8fafc"
UC_BG = "#ffffff"

ACTORS = [
    {
        "name": "Оператор",
        "color": "#2563eb",
        "ucs": [
            "Создать заявку\nна ремонт",
            "Просмотреть\nстатус заявки",
            "Подтвердить\nремонт",
        ],
    },
    {
        "name": "Техник",
        "color": "#0d9488",
        "ucs": [
            "Принять заявку\nв работу",
            "Зафиксировать\nрезультат",
            "Запросить\nзапчасти",
        ],
    },
    {
        "name": "Инженер",
        "color": "#a16207",
        "ucs": [
            "Вести справочник\nоборудования",
            "Назначить\nисполнителя",
            "Настроить\nграфики ТО",
            "Просматривать\nдашборд",
        ],
    },
    {
        "name": "Администратор",
        "color": "#b91c1c",
        "ucs": [
            "Управлять\nучётными записями",
        ],
    },
]

# Общие варианты использования, доступные нескольким акторам
SHARED_UCS = [
    {
        "label": "Сканировать\nQR-код",
        "actors": ["Оператор", "Техник"],
    },
]


def draw_actor(ax, x, y, label, color):
    head_r = 0.32
    ax.add_patch(patches.Circle((x, y + 1.2), head_r,
                                edgecolor=color, facecolor="white",
                                linewidth=1.6))
    ax.plot([x, x], [y + 0.85, y + 0.05], color=color, linewidth=1.6)
    ax.plot([x - 0.55, x + 0.55], [y + 0.55, y + 0.55],
            color=color, linewidth=1.6)
    ax.plot([x, x - 0.4], [y + 0.05, y - 0.55], color=color, linewidth=1.6)
    ax.plot([x, x + 0.4], [y + 0.05, y - 0.55], color=color, linewidth=1.6)
    ax.text(x, y - 0.95, label, ha="center", va="top",
            fontsize=12, fontweight="bold", color=color)


def use_case(ax, cx, cy, w, h, label):
    ell = patches.Ellipse((cx, cy), w, h, edgecolor=LINE,
                          facecolor=UC_BG, linewidth=1.2)
    ax.add_patch(ell)
    ax.text(cx, cy, label, ha="center", va="center", fontsize=11,
            color=LINE)


def draw():
    cols = len(ACTORS)
    col_w = 5.0
    fig, ax = plt.subplots(figsize=(15, 11), dpi=200)
    width = cols * col_w
    ax.set_xlim(0, width)
    ax.set_ylim(-1, 26)
    ax.set_aspect("equal")
    ax.axis("off")

    # Акторы — поверх рамки системы
    actor_y = 24.0

    # Рамка системы — ниже акторов
    sys_top = 21.0
    sys_bottom = 1.0
    sys_rect = patches.Rectangle(
        (0.3, sys_bottom), width - 0.6, sys_top - sys_bottom,
        edgecolor=LINE, facecolor="none", linewidth=1.4,
    )
    ax.add_patch(sys_rect)
    ax.text(width / 2, sys_top + 0.55,
            "Веб-приложение управления ТОиР",
            ha="center", va="center", fontsize=13, fontweight="bold",
            color=LINE)

    # Use cases в колонках
    uc_top_y = sys_top - 1.7
    uc_step = 3.0
    uc_w = 4.2
    uc_h = 1.7

    uc_positions = {}  # {(actor_idx, uc_idx): (cx, cy)}
    actor_xy = {}      # {actor_name: (cx, cy)}

    for i, actor in enumerate(ACTORS):
        col_x = i * col_w + col_w / 2
        if i > 0:
            ax.plot([i * col_w, i * col_w], [sys_bottom, sys_top],
                    color=LIGHT, linewidth=0.8, linestyle=(0, (4, 3)),
                    zorder=1)

        for j, uc in enumerate(actor["ucs"]):
            cy = uc_top_y - j * uc_step
            use_case(ax, col_x, cy, uc_w, uc_h, uc)
            uc_positions[(i, j)] = (col_x, cy)

        draw_actor(ax, col_x, actor_y - 0.5, actor["name"], actor["color"])
        actor_xy[actor["name"]] = (col_x, actor_y - 0.5)

        # Линия от актора к первому use case
        first_cy = uc_top_y
        ax.plot([col_x, col_x], [actor_y - 1.5, first_cy + uc_h / 2],
                color=actor["color"], linewidth=1.2, alpha=0.8,
                zorder=1)

    # Общие use cases (например «Сканировать QR-код») — внизу системы по центру
    shared_y = sys_bottom + 1.4
    shared_x_start = width * 0.32
    for k, shared in enumerate(SHARED_UCS):
        cx = shared_x_start + k * 5.5
        use_case(ax, cx, shared_y, 3.8, 1.5, shared["label"])
        for actor_name in shared["actors"]:
            ax_pos = actor_xy[actor_name]
            # Прямая ассоциация от актора через систему до общего uc
            ax.plot(
                [ax_pos[0], cx],
                [ax_pos[1] - 1.0, shared_y + 0.75],
                color="#94a3b8", linewidth=1.0, linestyle=(0, (3, 3)),
                alpha=0.9, zorder=0,
            )

    # «Выбрать оборудование» — общий use case с include
    select_x = width * 0.62
    use_case(ax, select_x, shared_y, 3.8, 1.5, "Выбрать\nоборудование")
    src = uc_positions[(0, 0)]
    ax.annotate(
        "",
        xy=(select_x - 1.6, shared_y + 0.45),
        xytext=(src[0] - 1.4, src[1] - uc_h / 2),
        arrowprops=dict(arrowstyle="->", color=LINE, linewidth=1.0,
                        linestyle=(0, (5, 3))),
    )
    ax.text(select_x - 2.6, (src[1] - uc_h / 2 + shared_y + 0.45) / 2,
            "«include»",
            ha="center", va="center", fontsize=10, fontstyle="italic",
            color=LINE)

    fig.tight_layout()
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(OUTPUT, dpi=200, bbox_inches="tight", pad_inches=0.3,
                facecolor="white")
    print(f"Saved: {OUTPUT}")


if __name__ == "__main__":
    draw()
