#!/usr/bin/env python3
"""
Use Case диаграмма (UML 2.5) для веб-приложения управления ТОиР.
Горизонтальная компоновка: акторы слева/справа, варианты использования в центре.
"""
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.patches as patches

OUTPUT = Path(
    "/home/washka/project/diplom/Nikita/images/2.2_use_case_diagramma.png"
)

LINE = "#000000"
FILL = "#FFFFFF"
ACTOR_FILL = "#FFFFFF"
LW = 1.4
FS = 10


def draw_actor(ax, x, y, label):
    head_r = 0.18
    ax.add_patch(patches.Circle((x, y + 0.7), head_r, edgecolor=LINE,
                                facecolor=ACTOR_FILL, linewidth=LW))
    ax.add_line(plt.Line2D([x, x], [y + 0.5, y - 0.1], color=LINE,
                           linewidth=LW))
    ax.add_line(plt.Line2D([x - 0.4, x + 0.4], [y + 0.35, y + 0.35],
                           color=LINE, linewidth=LW))
    ax.add_line(plt.Line2D([x, x - 0.3], [y - 0.1, y - 0.6], color=LINE,
                           linewidth=LW))
    ax.add_line(plt.Line2D([x, x + 0.3], [y - 0.1, y - 0.6], color=LINE,
                           linewidth=LW))
    ax.text(x, y - 0.85, label, ha="center", va="top",
            fontsize=FS + 1, fontweight="bold")


def use_case(ax, x, y, label, w=2.7, h=0.95):
    ell = patches.Ellipse((x, y), w, h, edgecolor=LINE, facecolor=FILL,
                          linewidth=LW)
    ax.add_patch(ell)
    ax.text(x, y, label, ha="center", va="center", fontsize=FS, wrap=True)
    return (x, y, w, h)


def assoc(ax, p1, p2):
    ax.add_line(plt.Line2D([p1[0], p2[0]], [p1[1], p2[1]],
                           color=LINE, linewidth=LW))


def include(ax, p1, p2, label="«include»"):
    ax.annotate("", xy=(p2[0], p2[1]), xytext=(p1[0], p1[1]),
                arrowprops=dict(arrowstyle="->", color=LINE, linewidth=LW,
                                linestyle=(0, (5, 3))))
    mx, my = (p1[0] + p2[0]) / 2, (p1[1] + p2[1]) / 2
    ax.text(mx, my + 0.15, label, ha="center", va="bottom",
            fontsize=FS - 1, style="italic")


def extend(ax, p1, p2, label="«extend»"):
    ax.annotate("", xy=(p2[0], p2[1]), xytext=(p1[0], p1[1]),
                arrowprops=dict(arrowstyle="->", color=LINE, linewidth=LW,
                                linestyle=(0, (5, 3))))
    mx, my = (p1[0] + p2[0]) / 2, (p1[1] + p2[1]) / 2
    ax.text(mx, my + 0.15, label, ha="center", va="bottom",
            fontsize=FS - 1, style="italic")


def main():
    fig, ax = plt.subplots(figsize=(16, 10), dpi=200)
    ax.set_xlim(0, 22)
    ax.set_ylim(0, 14)
    ax.set_aspect("equal")
    ax.axis("off")

    # System boundary
    boundary = patches.Rectangle((4.5, 0.6), 13, 12.8, linewidth=LW,
                                 edgecolor=LINE, facecolor="none")
    ax.add_patch(boundary)
    ax.text(11, 13.0, "Веб-приложение управления ТОиР",
            ha="center", va="center", fontsize=FS + 2, fontweight="bold")

    # Actors left
    draw_actor(ax, 1.8, 11.0, "Оператор")
    draw_actor(ax, 1.8, 4.0, "Техник")

    # Actors right
    draw_actor(ax, 20.2, 11.5, "Инженер")
    draw_actor(ax, 20.2, 3.5, "Администратор")

    # ---- Use cases (3 columns x 4 rows) ----
    col1, col2, col3 = 7.5, 11.0, 14.5

    # Operator zone (top-left)
    uc_create = use_case(ax, col1, 11.5, "Создать заявку\nна ремонт")
    uc_select = use_case(ax, col2, 11.5, "Выбрать\nоборудование")
    uc_status = use_case(ax, col1, 9.5, "Просмотреть статус\nсвоих заявок")
    uc_confirm = use_case(ax, col2, 9.5, "Подтвердить\nзавершение ремонта")
    uc_qr = use_case(ax, col3, 11.5, "Сканировать\nQR-код")

    # Technician zone (bottom-left)
    uc_tasks = use_case(ax, col1, 4.5, "Принять задание\nв работу")
    uc_fix = use_case(ax, col1, 2.7, "Зафиксировать\nрезультат работ")
    uc_parts = use_case(ax, col2, 2.7, "Запросить\nзапчасти")

    # Engineer zone (top-right and middle)
    uc_eq = use_case(ax, col3, 9.5, "Вести справочник\nоборудования")
    uc_assign = use_case(ax, col2, 7.0, "Назначить\nисполнителя")
    uc_sched = use_case(ax, col3, 7.0, "Настроить\nграфики ТО")
    uc_dash = use_case(ax, col1, 7.0, "Просматривать\nдашборд")
    uc_notify = use_case(ax, col3, 5.0, "Уведомление\nо приближении ТО")

    # Admin zone (bottom-right)
    uc_users = use_case(ax, col3, 2.7, "Управлять учётными\nзаписями")

    # Operator associations
    op = (1.8, 11.0)
    assoc(ax, op, (uc_create[0] - uc_create[2] / 2, uc_create[1]))
    assoc(ax, op, (uc_status[0] - uc_status[2] / 2, uc_status[1]))
    assoc(ax, op, (uc_confirm[0] - uc_confirm[2] / 2, uc_confirm[1]))
    assoc(ax, op, (uc_qr[0] - uc_qr[2] / 2 - 1.5, uc_qr[1]))

    # Technician associations
    tech = (1.8, 4.0)
    assoc(ax, tech, (uc_tasks[0] - uc_tasks[2] / 2, uc_tasks[1]))
    assoc(ax, tech, (uc_fix[0] - uc_fix[2] / 2, uc_fix[1]))
    assoc(ax, tech, (uc_parts[0] - uc_parts[2] / 2, uc_parts[1]))
    assoc(ax, tech, (uc_qr[0] - uc_qr[2] / 2 - 1.5, uc_qr[1] - 1.0))

    # Engineer associations
    eng = (20.2, 11.5)
    assoc(ax, eng, (uc_eq[0] + uc_eq[2] / 2, uc_eq[1]))
    assoc(ax, eng, (uc_sched[0] + uc_sched[2] / 2, uc_sched[1]))
    assoc(ax, eng, (uc_assign[0] + uc_assign[2] / 2, uc_assign[1]))
    assoc(ax, eng, (uc_dash[0] + uc_dash[2] / 2, uc_dash[1]))

    # Admin associations
    admin = (20.2, 3.5)
    assoc(ax, admin, (uc_users[0] + uc_users[2] / 2, uc_users[1]))

    # Include: Создать заявку -> Выбрать оборудование
    include(ax,
            (uc_create[0] + uc_create[2] / 2, uc_create[1]),
            (uc_select[0] - uc_select[2] / 2, uc_select[1]))

    # Extend: Уведомление -> Настроить график ТО
    extend(ax,
           (uc_notify[0], uc_notify[1] + uc_notify[3] / 2),
           (uc_sched[0], uc_sched[1] - uc_sched[3] / 2))

    plt.tight_layout()
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(OUTPUT, dpi=200, bbox_inches="tight", pad_inches=0.2,
                facecolor="white")
    print(f"Saved: {OUTPUT}")


if __name__ == "__main__":
    main()
