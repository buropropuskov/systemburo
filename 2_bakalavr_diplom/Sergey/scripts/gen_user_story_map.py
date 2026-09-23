#!/usr/bin/env python3
"""Генерация user story map через matplotlib (таблица с цветовыми дорожками)."""
from pathlib import Path
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import Rectangle

IMAGES = Path("/home/washka/project/diplom/Sergey/images")
plt.rcParams["font.family"] = ["DejaVu Sans", "Liberation Sans", "sans-serif"]

# колонки — этапы пути пользователя
BACKBONE = [
    "Знакомство",
    "Регистрация",
    "Загрузка\nдокумента",
    "Работа\nс результатами",
    "Управление\nфайлами",
    "Поддержка",
]

# строки — релизы (сверху вниз: MVP, Релиз 2, Релиз 3)
BRAND_BLUE = "#6C67FD"
BLACK = "#000000"
WHITE = "#FFFFFF"
LIGHT = "#E6E6E6"
MUTED = "#A2A2A2"

RELEASES = [
    {
        "name": "Релиз 1 — MVP",
        "color": BRAND_BLUE,
        "edge":  BLACK,
        "text":  WHITE,
        "cells": [
            "Посмотреть\nлендинг",
            "Регистрация\nemail + пароль",
            "Загрузить\nPDF / DOCX",
            "Увидеть\nподсветку рисков",
            "Список\nдокументов",
            "Email-\nподдержка",
        ],
    },
    {
        "name": "Релиз 2 — расширение",
        "color": LIGHT,
        "edge":  BLACK,
        "text":  BLACK,
        "cells": [
            "Тарифы\nи цены",
            "Подтверждение\nemail",
            "Drag-n-drop\nзагрузка",
            "Чат с ИИ-\nпомощником",
            "Поиск и\nсортировка",
            "База знаний\nв интерфейсе",
        ],
    },
    {
        "name": "Релиз 3 — развитие",
        "color": WHITE,
        "edge":  BLACK,
        "text":  BLACK,
        "cells": [
            "Блок отзывов",
            "Выбор\nтарифа",
            "Ограничение\nразмера\n(50 МБ)",
            "Экспорт\nотчёта PDF",
            "Папки\nи корзина",
            "Личный\nкабинет",
        ],
    },
]


def draw():
    fig, ax = plt.subplots(figsize=(14, 8), dpi=150)
    n_cols = len(BACKBONE)
    n_rows = len(RELEASES)
    col_w = 2.0
    row_h = 1.5
    y_top = (n_rows + 1) * row_h + 1.5

    ax.set_xlim(-0.5, n_cols * col_w + 0.5)
    ax.set_ylim(0, y_top)
    ax.axis("off")

    ax.text(n_cols * col_w / 2, y_top - 0.3, "Карта пользовательских историй сервиса Dockee",
            ha="center", fontsize=14, fontweight="bold", color=BLACK)

    # backbone (верхняя строка с этапами) — белый фон, чёрный контур
    y = y_top - 1.8
    for i, stage in enumerate(BACKBONE):
        rect = Rectangle((i * col_w + 0.1, y), col_w - 0.2, 1.2,
                         facecolor=WHITE, edgecolor=BLACK, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(i * col_w + col_w / 2, y + 0.6, stage, ha="center", va="center",
                fontsize=10, fontweight="bold", color=BLACK)

    ax.text(-0.3, y + 0.6, "Путь\nпользователя", ha="right", va="center",
            fontsize=9, style="italic", color=MUTED)

    for ri, release in enumerate(RELEASES):
        y = y_top - 1.8 - (ri + 1) * (row_h + 0.1)
        ax.text(-0.3, y + row_h / 2, release["name"], ha="right", va="center",
                fontsize=10, fontweight="bold", color=BLACK)
        for ci, text in enumerate(release["cells"]):
            rect = Rectangle((ci * col_w + 0.15, y + 0.1), col_w - 0.3, row_h - 0.2,
                             facecolor=release["color"], edgecolor=release["edge"],
                             linewidth=1.1)
            ax.add_patch(rect)
            ax.text(ci * col_w + col_w / 2, y + row_h / 2, text, ha="center", va="center",
                    fontsize=9, color=release["text"])

    legend_y = 0.3
    ax.text(n_cols * col_w / 2, legend_y,
            "Чем выше в колонке — тем более приоритетная история; первая строка — необходимый минимум для выпуска MVP",
            ha="center", fontsize=9, style="italic", color=MUTED)

    plt.tight_layout()
    out = IMAGES / "2.3.5_user_story_map.png"
    plt.savefig(out, dpi=150, bbox_inches="tight", facecolor="white")
    plt.close()
    print(f"  OK {out.name}  ({out.stat().st_size // 1024} KB)")


if __name__ == "__main__":
    draw()
