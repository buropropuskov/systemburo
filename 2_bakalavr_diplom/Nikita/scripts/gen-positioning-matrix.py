#!/usr/bin/env python3
"""
Рисунок 1.1 — Совокупная стоимость владения CMMS-системами за 5 лет
для предприятия среднего масштаба (200-500 единиц оборудования, 10 пользователей).
Простая горизонтальная гистограмма.
"""
from pathlib import Path

import matplotlib.pyplot as plt

OUTPUT = Path(
    "/home/washka/project/diplom/Nikita/images/1.1_pozicionirovanie_cmms.png"
)

# Системы и их совокупная стоимость владения за 5 лет, млн руб.
# Включает лицензию, внедрение, поддержку и сопровождение.
SYSTEMS = [
    ("SAP Plant Maintenance", 38.0,  "#94a3b8"),
    ("IBM Maximo",            22.0,  "#94a3b8"),
    ("1С:ТОИР",               4.5,   "#94a3b8"),
    ("UpKeep (SaaS, 5 лет)",  3.6,   "#94a3b8"),
    ("Разрабатываемое решение", 1.2, "#1e40af"),
]


def draw():
    fig, ax = plt.subplots(figsize=(11, 5.5), dpi=200)
    names = [s[0] for s in SYSTEMS]
    values = [s[1] for s in SYSTEMS]
    colors = [s[2] for s in SYSTEMS]

    bars = ax.barh(names, values, color=colors, height=0.6,
                   edgecolor="white", linewidth=1.0)

    # Подпись значения справа от каждого столбца
    for bar, val in zip(bars, values):
        ax.text(val + 0.6, bar.get_y() + bar.get_height() / 2,
                f"{val:.1f} млн ₽",
                va="center", ha="left", fontsize=12,
                color="#1f2937", fontweight="bold")

    ax.set_xlim(0, max(values) * 1.18)
    ax.invert_yaxis()
    ax.tick_params(axis="y", labelsize=12)
    ax.tick_params(axis="x", labelsize=10, colors="#6b7280")
    ax.set_xlabel("Совокупная стоимость владения за 5 лет, млн руб.",
                  fontsize=12, color="#1f2937", labelpad=10)

    for spine in ("top", "right", "left"):
        ax.spines[spine].set_visible(False)
    ax.spines["bottom"].set_color("#cbd5e1")
    ax.xaxis.grid(True, color="#e5e7eb", linewidth=0.8)
    ax.set_axisbelow(True)

    fig.tight_layout()
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(OUTPUT, dpi=200, bbox_inches="tight", pad_inches=0.3,
                facecolor="white")
    print(f"Saved: {OUTPUT}")


if __name__ == "__main__":
    draw()
