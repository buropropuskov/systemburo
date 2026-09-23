"""Строгие монохромные диаграммы для презентации.

Палитра: navy (#1F3A5F) + тёмно-серый (#707070) + светло-серый (#D0D0D0). Белый фон.
Текст — TNR-совместимый serif, крупный, с запасом по месту.
"""

from __future__ import annotations

import pathlib
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches

ROOT = pathlib.Path(__file__).resolve().parent.parent
IMAGES = ROOT / "images"
PRESENTATION = ROOT / "presentation"

# Файлы, предназначенные для презентации, кладём в отдельную папку.
_PRESENTATION_FILES = {"pres_user_stories.png", "pres_usecase.png", "test_metrics.png"}

NAVY = "#1F3A5F"
NAVY_LIGHT = "#5A7199"
GRAY_DARK = "#4A4A4A"
GRAY = "#707070"
GRAY_LIGHT = "#D0D0D0"
ACCENT = "#8B3A3A"

plt.rcParams.update({
    "font.family": "serif",
    "font.serif": ["Liberation Serif", "Times New Roman", "DejaVu Serif"],
    "font.size": 13,
})


def _box(ax, x, y, w, h, text, *, face="white", edge=NAVY, fontcolor=NAVY,
         fontsize=14, bold=True, lineheight=0.5):
    ax.add_patch(
        mpatches.FancyBboxPatch(
            (x, y), w, h,
            boxstyle="round,pad=0.02,rounding_size=0.05",
            facecolor=face, edgecolor=edge, linewidth=1.6,
        )
    )
    if isinstance(text, str):
        ax.text(x + w / 2, y + h / 2, text, ha="center", va="center",
                fontsize=fontsize, color=fontcolor,
                fontweight="bold" if bold else "normal")
    else:
        # text — список строк.
        n = len(text)
        for i, line in enumerate(text):
            cy = y + h - (i + 0.5) * (h / n)
            ax.text(x + w / 2, cy, line, ha="center", va="center",
                    fontsize=fontsize if i == 0 else fontsize - 2,
                    color=fontcolor, fontweight="bold" if (bold and i == 0) else "normal")


def _arrow(ax, x1, y1, x2, y2):
    ax.annotate("", xy=(x2, y2), xytext=(x1, y1),
                arrowprops=dict(arrowstyle="-|>", color=GRAY_DARK, lw=1.6))


def _setup(figsize=(10.0, 6.0)):
    fig, ax = plt.subplots(figsize=figsize, dpi=150)
    ax.set_xlim(0, 16)
    ax.set_ylim(0, 10)
    ax.axis("off")
    fig.patch.set_facecolor("white")
    return fig, ax


def _save(fig, name: str) -> None:
    target_dir = PRESENTATION if name in _PRESENTATION_FILES else IMAGES
    target_dir.mkdir(parents=True, exist_ok=True)
    out = target_dir / name
    fig.savefig(out, dpi=150, bbox_inches="tight", facecolor="white")
    plt.close(fig)
    print(f"OK -> {out.relative_to(ROOT)}")


def make_tech_stack() -> None:
    fig, ax = _setup()
    layers = [
        (8.5, "Vue 3 + Pinia", "Composition API · 98 компонентов · бандл 142 КБ gzip"),
        (7.0, "Nginx", "reverse proxy · отдача SPA · проксирование /api · SSL"),
        (5.5, "Go 1.22 + Echo", "Clean Architecture · 32 REST-эндпоинта · JWT"),
        (4.0, "PostgreSQL 16", "ACID · JSONB · 8 сущностей · 12 таблиц"),
        (2.5, "Docker Compose", "multi-stage · образ Go 18 МБ · развёртывание в 1 команду"),
    ]
    for y, title, sub in layers:
        ax.add_patch(
            mpatches.FancyBboxPatch(
                (1.0, y - 0.55), 14.0, 1.15,
                boxstyle="round,pad=0.02,rounding_size=0.05",
                facecolor="white", edgecolor=NAVY, linewidth=1.8,
            )
        )
        ax.text(1.4, y + 0.15, title, fontsize=17, fontweight="bold",
                color=NAVY, va="center")
        ax.text(1.4, y - 0.28, sub, fontsize=13, color=GRAY_DARK, va="center")

    for y in (7.7, 6.2, 4.7, 3.2):
        ax.annotate("", xy=(8.0, y - 0.15), xytext=(8.0, y + 0.15),
                    arrowprops=dict(arrowstyle="-|>", color=GRAY, lw=1.4))

    ax.text(8, 9.55, "Уровни системы: фронт → прокси → API → БД → контейнеризация",
            ha="center", fontsize=14, color=GRAY_DARK, style="italic")

    _save(fig, "tech_stack.png")


def make_test_metrics() -> None:
    # Увеличенный холст, левая колонка шире — длинная строка
    # "45 интеграционных (testcontainers + PostgreSQL)" помещается в основание пирамиды.
    fig = plt.figure(figsize=(14.0, 7.5), dpi=150)
    gs = fig.add_gridspec(2, 2, width_ratios=[1.25, 1], height_ratios=[1, 1],
                          hspace=0.55, wspace=0.30)

    # 1. Пирамида тестов — левая колонка на всю высоту.
    ax1 = fig.add_subplot(gs[:, 0])
    ax1.set_xlim(0, 12); ax1.set_ylim(0, 10); ax1.axis("off")
    ax1.text(6, 9.5, "Пирамида тестов", fontsize=17, fontweight="bold",
             ha="center", color=NAVY)
    ax1.text(6, 8.9, "81 тест · 100 % пройдено", fontsize=12, ha="center",
             color=GRAY_DARK, style="italic")

    # Ступени от основания (широкая) к вершине (узкая) — классическая пирамида.
    levels = [
        (0.6, 45, "45 интеграционных\n(testcontainers + PostgreSQL)"),
        (3.1, 28, "28 модульных (Go)"),
        (5.6, 8,  "8 сквозных\n(Playwright E2E)"),
    ]
    widths = {45: 11.6, 28: 8.2, 8: 5.0}
    for y, n, label in levels:
        w = widths[n]
        x = (12 - w) / 2
        ax1.add_patch(
            mpatches.FancyBboxPatch(
                (x, y), w, 2.2,
                boxstyle="round,pad=0.02,rounding_size=0.05",
                facecolor="white", edgecolor=NAVY, linewidth=1.8,
            )
        )
        ax1.text(6, y + 1.1, label, fontsize=11, fontweight="bold",
                 ha="center", va="center", color=NAVY, linespacing=1.2)

    # 2. Покрытие кода (верх-право).
    ax2 = fig.add_subplot(gs[0, 1])
    ax2.axis("off")
    ax2.text(0.5, 1.08, "Покрытие кода", fontsize=16, fontweight="bold",
             ha="center", color=NAVY, transform=ax2.transAxes)
    coverage = [("Аутентификация", 84), ("Обработка заявок", 76), ("Всего", 68)]
    for i, (label, pct) in enumerate(coverage):
        y = 0.78 - i * 0.3
        ax2.barh(y, 1.0, height=0.18, color=GRAY_LIGHT, edgecolor=GRAY)
        ax2.barh(y, pct / 100, height=0.18, color=NAVY, edgecolor=NAVY)
        ax2.text(-0.02, y, label, ha="right", va="center", fontsize=12,
                 color=GRAY_DARK)
        ax2.text(pct / 100 + 0.02, y, f"{pct} %", ha="left", va="center",
                 fontsize=12, fontweight="bold", color=NAVY)
    ax2.set_xlim(-0.40, 1.22); ax2.set_ylim(-0.05, 1.15)

    # 3. Нагрузка (низ-право).
    ax3 = fig.add_subplot(gs[1, 1])
    ax3.axis("off")
    ax3.text(0.5, 1.05, "Нагрузка · 500 одновременных подключений",
             fontsize=13, fontweight="bold", ha="center", color=NAVY,
             transform=ax3.transAxes)
    metrics = [
        ("медиана", "47 мс"),
        ("95-й перцентиль", "180 мс"),
        ("99-й перцентиль", "320 мс"),
        ("пропускная способность", "2.4k req/s"),
    ]
    for i, (label, value) in enumerate(metrics):
        row, col = divmod(i, 2)
        x = 0.04 + col * 0.49
        y = 0.52 - row * 0.45
        ax3.add_patch(
            mpatches.FancyBboxPatch(
                (x, y), 0.46, 0.38,
                boxstyle="round,pad=0.01,rounding_size=0.03",
                transform=ax3.transAxes,
                facecolor="white", edgecolor=NAVY, linewidth=1.5,
            )
        )
        ax3.text(x + 0.23, y + 0.25, value, transform=ax3.transAxes,
                 fontsize=16, fontweight="bold", color=NAVY, ha="center", va="center")
        ax3.text(x + 0.23, y + 0.08, label, transform=ax3.transAxes,
                 fontsize=10, color=GRAY_DARK, ha="center", va="center")
    ax3.set_xlim(0, 1); ax3.set_ylim(0, 1)

    _save(fig, "test_metrics.png")


def make_cmms_positioning() -> None:
    fig, ax = _setup(figsize=(10.0, 6.0))
    ax.set_xlim(0, 10); ax.set_ylim(0, 10)

    # Оси-квадранты.
    ax.axhline(5, color=GRAY, lw=1.2, ls="--")
    ax.axvline(5, color=GRAY, lw=1.2, ls="--")
    ax.plot([0.3, 0.3, 9.7], [9.7, 0.3, 0.3], color=GRAY_DARK, lw=1.5)
    ax.annotate("", xy=(9.9, 0.3), xytext=(9.7, 0.3),
                arrowprops=dict(arrowstyle="-|>", color=GRAY_DARK, lw=1.5))
    ax.annotate("", xy=(0.3, 9.9), xytext=(0.3, 9.7),
                arrowprops=dict(arrowstyle="-|>", color=GRAY_DARK, lw=1.5))

    ax.text(9.7, -0.1, "стоимость владения", fontsize=13, color=GRAY_DARK,
            ha="right", va="top")
    ax.text(-0.15, 9.7, "локальность / контроль", fontsize=13, color=GRAY_DARK,
            ha="right", va="top", rotation=90)

    # Точки систем.
    items = [
        (8.2, 7.5, "SAP PM", "10–15 млн ₽"),
        (9.0, 8.2, "IBM Maximo", "$30k/год"),
        (6.5, 7.0, "1С:ТОИР", "от 300 тыс. ₽"),
        (3.0, 2.5, "UpKeep SaaS", "$45/мес/польз."),
        (2.5, 8.5, "Наше решение", "open source"),
    ]
    for x, y, label, price in items:
        ax.plot(x, y, "o", color=NAVY, markersize=16,
                markeredgecolor=NAVY, markeredgewidth=2, markerfacecolor="white")
        ax.text(x, y - 0.65, label, ha="center", va="top", fontsize=14,
                fontweight="bold", color=NAVY)
        ax.text(x, y - 1.1, price, ha="center", va="top", fontsize=11,
                color=GRAY_DARK, style="italic")

    ax.text(5, 10.1, "Позиционирование CMMS-систем: ниша среднего бизнеса свободна",
            fontsize=14, fontweight="bold", ha="center", color=NAVY)

    _save(fig, "pres_cmms_positioning.png")


def make_usecase() -> None:
    # Более широкий холст и увеличенные прямоугольники — текст «Управлять
    # пользователями» (22 символа) помещается с запасом.
    fig, ax = plt.subplots(figsize=(13.0, 7.0), dpi=150)
    ax.set_xlim(0, 20); ax.set_ylim(0, 11)
    ax.axis("off")
    fig.patch.set_facecolor("white")

    actors = [
        (1.6, 8.9, "Оператор"),
        (1.6, 6.7, "Техник"),
        (1.6, 4.5, "Инженер"),
        (1.6, 2.3, "Администратор"),
    ]
    for x, y, name in actors:
        ax.add_patch(mpatches.Circle((x, y + 0.7), 0.38, facecolor="white",
                                     edgecolor=NAVY, lw=2))
        ax.plot([x, x], [y + 0.32, y - 0.15], color=NAVY, lw=2)
        ax.plot([x - 0.35, x + 0.35], [y + 0.15, y + 0.15], color=NAVY, lw=2)
        ax.plot([x - 0.2, x], [y - 0.55, y - 0.15], color=NAVY, lw=2)
        ax.plot([x + 0.2, x], [y - 0.55, y - 0.15], color=NAVY, lw=2)
        ax.text(x, y - 1.0, name, ha="center", fontsize=13,
                fontweight="bold", color=NAVY)

    # Ширина use case-боксов увеличена с 4.6 до 6.4; столбцы сдвинуты вправо.
    uc_w = 6.4
    col1_cx = 7.2
    col2_cx = 14.2
    usecases = [
        (col1_cx, 8.9, "Создать заявку на ремонт"),
        (col2_cx, 8.9, "Отследить статус заявки"),
        (col1_cx, 6.7, "Принять заявку в работу"),
        (col2_cx, 6.7, "Закрыть наряд с отчётом"),
        (col1_cx, 4.5, "Планировать график ТО"),
        (col2_cx, 4.5, "Смотреть отчётность"),
        (col1_cx, 2.3, "Управлять пользователями"),
        (col2_cx, 2.3, "Редактировать справочники"),
    ]
    for x, y, name in usecases:
        ax.add_patch(
            mpatches.FancyBboxPatch(
                (x - uc_w / 2, y - 0.55), uc_w, 1.1,
                boxstyle="round,pad=0.02,rounding_size=0.55",
                facecolor="white", edgecolor=NAVY, linewidth=1.8,
            )
        )
        ax.text(x, y, name, ha="center", va="center", fontsize=13,
                color=NAVY, fontweight="bold")

    # Горизонтальные прямые линии, нигде не пересекающие рамки:
    #  - актор → левый use case (от правой руки к левой точке капсулы);
    #  - левый use case → правый use case (между их торцевыми точками).
    # Все сегменты идут строго по одной Y, соответствующей уровню актора.
    pairs = [(0, 0, 1), (1, 2, 3), (2, 4, 5), (3, 6, 7)]
    for a, u_left, u_right in pairs:
        y_arm = actors[a][1] + 0.15
        y_uc = usecases[u_left][1]
        ax.plot([actors[a][0] + 0.35, usecases[u_left][0] - uc_w / 2],
                [y_arm, y_uc], color=GRAY, lw=1.3)
        ax.plot([usecases[u_left][0] + uc_w / 2, usecases[u_right][0] - uc_w / 2],
                [y_uc, y_uc], color=GRAY, lw=1.3)

    ax.text(10, 10.3, "Варианты использования: 4 роли, 8 ключевых сценариев",
            ha="center", fontsize=15, fontweight="bold", color=NAVY)

    _save(fig, "pres_usecase.png")


def make_user_stories() -> None:
    # Увеличенный холст, широкие колонки: каждая текстовая ячейка уместится целиком.
    fig, ax = plt.subplots(figsize=(14.0, 7.5), dpi=150)
    ax.set_xlim(0, 20); ax.set_ylim(0, 11)
    ax.axis("off")
    fig.patch.set_facecolor("white")

    stages = ["Аутентификация", "Оборудование", "Заявки", "Планирование ТО", "Аналитика"]
    col_w = 3.3
    col_gap = 0.15
    left_margin = 2.3

    for i, stage in enumerate(stages):
        x = left_margin + i * (col_w + col_gap)
        ax.add_patch(
            mpatches.FancyBboxPatch(
                (x, 9.0), col_w, 1.0,
                boxstyle="round,pad=0.02,rounding_size=0.05",
                facecolor=NAVY, edgecolor=NAVY,
            )
        )
        ax.text(x + col_w / 2, 9.5, stage, ha="center", va="center",
                fontsize=13, color="white", fontweight="bold")

    ax.text(left_margin - 0.3, 9.5, "этап", ha="right", va="center",
            fontsize=11, color=GRAY, style="italic")

    levels = [
        ("MVP", 6.9, [
            "Вход по логину\nи паролю",
            "Просмотр списка",
            "Создание заявки",
            "График ТО",
            "Дашборд KPI",
        ]),
        ("расширение", 4.6, [
            "Сброс пароля",
            "Редактирование\nкарточки",
            "Назначение\nисполнителя",
            "Автоматические\nнаряды",
            "Отчёт за период",
        ]),
        ("развитие", 2.3, [
            "Управление ролями",
            "История\nобслуживания",
            "Уведомления\nо сроках",
            "Напоминания\nпо ППР",
            "Экспорт в Excel",
        ]),
    ]
    for level_name, y, items in levels:
        ax.text(left_margin - 0.3, y + 0.75, level_name, ha="right", va="center",
                fontsize=12, color=NAVY, fontweight="bold")
        for i, item in enumerate(items):
            x = left_margin + i * (col_w + col_gap)
            ax.add_patch(
                mpatches.FancyBboxPatch(
                    (x, y), col_w, 1.5,
                    boxstyle="round,pad=0.02,rounding_size=0.05",
                    facecolor="white", edgecolor=NAVY, linewidth=1.5,
                )
            )
            ax.text(x + col_w / 2, y + 0.75, item, ha="center", va="center",
                    fontsize=11, color=NAVY, fontweight="bold",
                    linespacing=1.15)

    ax.text(10, 10.6, "User Story Map: 5 этапов × 3 уровня приоритета = 15 историй",
            ha="center", fontsize=15, fontweight="bold", color=NAVY)

    _save(fig, "pres_user_stories.png")


def make_architecture() -> None:
    fig, ax = _setup(figsize=(10.0, 6.0))

    boxes = [
        (0.3, 4.5, 3.3, 2.8, "Браузер", "Vue 3 SPA", "142 КБ gzip", "98 компонентов"),
        (4.0, 4.5, 3.3, 2.8, "Nginx", "reverse proxy", "SSL termination", "static + /api"),
        (7.7, 4.5, 3.3, 2.8, "Go API", "Echo · Clean Arch", "32 REST-эндпоинта", "JWT + RBAC"),
        (11.4, 4.5, 4.3, 2.8, "PostgreSQL 16", "ACID · JSONB", "8 сущностей · 12 таблиц", "индексы + FK"),
    ]
    for x, y, w, h, t1, t2, t3, t4 in boxes:
        ax.add_patch(
            mpatches.FancyBboxPatch(
                (x, y), w, h,
                boxstyle="round,pad=0.02,rounding_size=0.05",
                facecolor="white", edgecolor=NAVY, linewidth=2,
            )
        )
        ax.text(x + w / 2, y + h - 0.5, t1, ha="center", va="center",
                fontsize=16, fontweight="bold", color=NAVY)
        ax.text(x + w / 2, y + h - 1.15, t2, ha="center", va="center",
                fontsize=12, color=GRAY_DARK, style="italic")
        ax.text(x + w / 2, y + 0.95, t3, ha="center", va="center",
                fontsize=11, color=GRAY_DARK)
        ax.text(x + w / 2, y + 0.4, t4, ha="center", va="center",
                fontsize=11, color=GRAY_DARK)

    for i in range(3):
        x1 = boxes[i][0] + boxes[i][2]
        x2 = boxes[i + 1][0]
        y = 5.9
        ax.annotate("", xy=(x2 - 0.05, y), xytext=(x1 + 0.05, y),
                    arrowprops=dict(arrowstyle="-|>", color=GRAY_DARK, lw=2))

    ax.add_patch(
        mpatches.FancyBboxPatch(
            (0.3, 1.2), 15.4, 1.8,
            boxstyle="round,pad=0.02,rounding_size=0.05",
            facecolor=NAVY, edgecolor=NAVY,
        )
    )
    ax.text(8.0, 2.55, "Docker Compose", ha="center", va="center",
            fontsize=15, fontweight="bold", color="white")
    ax.text(8.0, 1.85, "multi-stage build  ·  Go 18 МБ  ·  Nginx+Vue 25 МБ  ·  развёртывание одной командой",
            ha="center", va="center", fontsize=11, color="white")

    ax.text(8.0, 9.4, "REST / JSON  ·  JWT в httpOnly-cookie  ·  CORS, CSRF, CSP",
            ha="center", fontsize=11, color=GRAY_DARK, style="italic")
    ax.text(8.0, 10.0, "Архитектура: клиент → прокси → API → БД",
            ha="center", fontsize=14, fontweight="bold", color=NAVY)

    _save(fig, "pres_architecture.png")


def make_er() -> None:
    fig, ax = _setup(figsize=(10.0, 6.0))

    def entity(x, y, w, h, title, fields):
        ax.add_patch(
            mpatches.Rectangle((x, y + h - 0.65), w, 0.65,
                               facecolor=NAVY, edgecolor=NAVY)
        )
        ax.add_patch(
            mpatches.Rectangle((x, y), w, h,
                               facecolor="white", edgecolor=NAVY, lw=1.8)
        )
        ax.text(x + w / 2, y + h - 0.33, title, ha="center", va="center",
                fontsize=13, color="white", fontweight="bold")
        for i, field in enumerate(fields):
            ax.text(x + 0.2, y + h - 0.95 - i * 0.4, field,
                    ha="left", va="center", fontsize=11, color=GRAY_DARK)

    entity(0.3, 6.6, 4.8, 3.1, "equipment", [
        "id · inventory_number", "type, manufacturer, model",
        "location, status", "specs (JSONB)",
    ])
    entity(5.6, 6.6, 4.8, 3.1, "work_order", [
        "id · equipment_id (FK)", "title, description",
        "priority, status", "assignee_id (FK) · deadline",
    ])
    entity(10.9, 6.6, 4.8, 3.1, "maintenance_history", [
        "id · equipment_id (FK)", "work_order_id (FK)",
        "performed_at · performed_by",
        "type · duration_hours · notes",
    ])

    entity(0.3, 2.0, 4.8, 3.1, "user", [
        "id · email (UK)", "full_name · password_hash",
        "role_id (FK)", "is_active · last_login_at",
    ])
    entity(5.6, 2.0, 4.8, 3.1, "role", [
        "id · name (admin, инженер,", "  техник, оператор)",
        "permissions[] (16 ключей)", "description",
    ])
    entity(10.9, 2.0, 4.8, 3.1, "maintenance_plan", [
        "id · equipment_id (FK)", "type (регламент / ТО)",
        "interval_days", "next_date · is_active",
    ])

    ax.annotate("", xy=(5.6, 8.1), xytext=(5.1, 8.1),
                arrowprops=dict(arrowstyle="->", color=GRAY_DARK, lw=1.5))
    ax.annotate("", xy=(10.9, 8.1), xytext=(10.4, 8.1),
                arrowprops=dict(arrowstyle="->", color=GRAY_DARK, lw=1.5))
    ax.annotate("", xy=(5.6, 3.5), xytext=(5.1, 3.5),
                arrowprops=dict(arrowstyle="->", color=GRAY_DARK, lw=1.5))
    ax.annotate("", xy=(2.7, 6.6), xytext=(11.3, 5.1),
                arrowprops=dict(arrowstyle="->", color=GRAY_DARK, lw=1.2))
    ax.annotate("", xy=(7.0, 6.6), xytext=(2.7, 5.1),
                arrowprops=dict(arrowstyle="->", color=GRAY_DARK, lw=1.2))

    ax.text(8, 10.0, "Модель данных: 6 центральных сущностей, ещё 2 таблицы-связки",
            ha="center", fontsize=14, fontweight="bold", color=NAVY)
    ax.text(8, 0.9, "всего 8 сущностей · 12 таблиц · foreign keys с каскадным поведением",
            ha="center", fontsize=11, color=GRAY_DARK, style="italic")

    _save(fig, "pres_er.png")


if __name__ == "__main__":
    make_tech_stack()
    make_test_metrics()
    make_cmms_positioning()
    make_usecase()
    make_user_stories()
    make_architecture()
    make_er()
