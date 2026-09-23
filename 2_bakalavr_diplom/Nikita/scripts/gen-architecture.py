#!/usr/bin/env python3
"""
Рисунок 2.2 — Архитектура системы.
Простая горизонтальная блочная диаграмма с 4 контейнерами:
Браузер -> Nginx -> Go API -> PostgreSQL.
Внутри каждого контейнера — короткий состав без раскрытия слоёв.
"""
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.patches as patches

OUTPUT = Path(
    "/home/washka/project/diplom/Nikita/images/2.3_arhitektura_sistemy.png"
)

LINE_COLOR = "#1f2937"
TEXT_COLOR = "#1f2937"
SUBTEXT_COLOR = "#4b5563"
BORDER = 1.4

CONTAINERS = [
    {
        "x": 0.5, "y": 1.5, "w": 4.0, "h": 4.0,
        "title": "Браузер",
        "subtitle": "Vue 3 SPA",
        "lines": ["Composition API", "Pinia + Vue Router", "Axios"],
        "fill": "#dbeafe",
        "border": "#1e40af",
    },
    {
        "x": 6.0, "y": 1.5, "w": 4.0, "h": 4.0,
        "title": "Nginx",
        "subtitle": "Reverse Proxy",
        "lines": ["статика SPA", "проксирование /api/*", "TLS"],
        "fill": "#dcfce7",
        "border": "#15803d",
    },
    {
        "x": 11.5, "y": 1.5, "w": 4.5, "h": 4.0,
        "title": "Go API",
        "subtitle": "Echo + GORM",
        "lines": [
            "JWT, RBAC",
            "Handlers -> Services",
            "Repositories",
        ],
        "fill": "#fef3c7",
        "border": "#a16207",
    },
    {
        "x": 17.5, "y": 1.5, "w": 4.0, "h": 4.0,
        "title": "PostgreSQL 16",
        "subtitle": "Реляционная БД",
        "lines": ["12 таблиц", "ACID-транзакции", "JSONB, индексы"],
        "fill": "#fee2e2",
        "border": "#b91c1c",
    },
]

ARROWS = [
    {
        "from": (4.5, 3.5),
        "to":   (6.0, 3.5),
        "label": "HTTPS",
        "sub":   "JSON",
    },
    {
        "from": (10.0, 3.5),
        "to":   (11.5, 3.5),
        "label": "/api/*",
        "sub":   "REST",
    },
    {
        "from": (16.0, 3.5),
        "to":   (17.5, 3.5),
        "label": "SQL",
        "sub":   "GORM",
    },
]


def draw():
    fig, ax = plt.subplots(figsize=(15, 5), dpi=200)
    ax.set_xlim(0, 22)
    ax.set_ylim(0, 7)
    ax.set_aspect("equal")
    ax.axis("off")

    for c in CONTAINERS:
        rect = patches.FancyBboxPatch(
            (c["x"], c["y"]), c["w"], c["h"],
            boxstyle="round,pad=0.05,rounding_size=0.25",
            facecolor=c["fill"], edgecolor=c["border"],
            linewidth=BORDER, zorder=2,
        )
        ax.add_patch(rect)
        cx = c["x"] + c["w"] / 2
        # title
        ax.text(cx, c["y"] + c["h"] - 0.65, c["title"],
                ha="center", va="center", fontsize=14,
                fontweight="bold", color=TEXT_COLOR, zorder=3)
        # subtitle
        ax.text(cx, c["y"] + c["h"] - 1.35, c["subtitle"],
                ha="center", va="center", fontsize=11,
                color=SUBTEXT_COLOR, fontstyle="italic", zorder=3)
        # body lines
        line_y = c["y"] + c["h"] - 2.2
        for line in c["lines"]:
            ax.text(cx, line_y, line, ha="center", va="center",
                    fontsize=11, color=TEXT_COLOR, zorder=3)
            line_y -= 0.55

    for a in ARROWS:
        arrow = patches.FancyArrowPatch(
            a["from"], a["to"],
            arrowstyle="-|>,head_length=0.35,head_width=0.22",
            color=LINE_COLOR, linewidth=1.6,
            mutation_scale=1, zorder=4,
        )
        ax.add_patch(arrow)
        mid_x = (a["from"][0] + a["to"][0]) / 2
        mid_y = a["from"][1]
        ax.text(mid_x, mid_y + 0.45, a["label"],
                ha="center", va="bottom", fontsize=11,
                fontweight="bold", color=TEXT_COLOR)
        ax.text(mid_x, mid_y - 0.55, a["sub"],
                ha="center", va="top", fontsize=10,
                color=SUBTEXT_COLOR, fontstyle="italic")

    # Подпись горизонта развёртывания снизу
    ax.text(11, 0.5, "Контейнеризация: Docker Compose (api, frontend, db)",
            ha="center", va="center", fontsize=11,
            color=SUBTEXT_COLOR, fontstyle="italic")

    fig.tight_layout()
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(OUTPUT, dpi=200, bbox_inches="tight", pad_inches=0.3,
                facecolor="white")
    print(f"Saved: {OUTPUT}")


if __name__ == "__main__":
    draw()
