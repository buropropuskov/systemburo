#!/usr/bin/env python3
"""Генерация 8 PNG-диаграмм для ВКР через matplotlib."""

from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch, Rectangle, Polygon
import matplotlib.patches as mpatches

IMAGES = Path("/home/washka/project/diplom/Sergey/images")
DPI = 150

plt.rcParams["font.family"] = ["DejaVu Sans", "Liberation Sans", "sans-serif"]
plt.rcParams["font.size"] = 11

# Единая палитра диаграмм: чёрный / белый / синий
BRAND_BLUE = "#6C67FD"
BLACK = "#000000"
WHITE = "#FFFFFF"
LIGHT = "#E6E6E6"
MUTED = "#A2A2A2"


def _box(ax, x, y, w, h, text, color=WHITE, edgecolor=BLACK, fontsize=10, fontweight="normal",
         accent=False):
    fill = BRAND_BLUE if accent else color
    textcolor = WHITE if accent else BLACK
    box = FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.015", facecolor=fill,
                         edgecolor=edgecolor, linewidth=1.2)
    ax.add_patch(box)
    ax.text(x + w / 2, y + h / 2, text, ha="center", va="center", fontsize=fontsize,
            fontweight=fontweight, color=textcolor, wrap=True)


def _diamond(ax, x, y, w, h, text, color=WHITE, edgecolor=BLACK, fontsize=10):
    pts = [(x + w / 2, y + h), (x + w, y + h / 2), (x + w / 2, y), (x, y + h / 2)]
    poly = Polygon(pts, closed=True, facecolor=color, edgecolor=edgecolor, linewidth=1.2)
    ax.add_patch(poly)
    ax.text(x + w / 2, y + h / 2, text, ha="center", va="center", fontsize=fontsize, color=BLACK)


def _arrow(ax, x1, y1, x2, y2, label=None, color=BLACK, ortho=True):
    """Прямая (ортогональная) стрелка. ortho=True рисует Г-образный путь через промежуточную точку."""
    if ortho and x1 != x2 and y1 != y2:
        mid_x, mid_y = x2, y1
        ax.plot([x1, mid_x], [y1, mid_y], color=color, linewidth=1.1)
        arr = FancyArrowPatch((mid_x, mid_y), (x2, y2), arrowstyle="->",
                              mutation_scale=12, color=color, linewidth=1.1)
        ax.add_patch(arr)
    else:
        arr = FancyArrowPatch((x1, y1), (x2, y2), arrowstyle="->",
                              mutation_scale=12, color=color, linewidth=1.1)
        ax.add_patch(arr)
    if label:
        ax.text((x1 + x2) / 2, (y1 + y2) / 2, label, fontsize=9, color=color,
                bbox=dict(facecolor=WHITE, edgecolor="none", pad=2))


def _ellipse(ax, x, y, w, h, text, color=WHITE, edgecolor=BLACK, fontsize=10):
    from matplotlib.patches import Ellipse
    e = Ellipse((x + w / 2, y + h / 2), w, h, facecolor=color, edgecolor=edgecolor, linewidth=1.2)
    ax.add_patch(e)
    ax.text(x + w / 2, y + h / 2, text, ha="center", va="center", fontsize=fontsize,
            color=BLACK, wrap=True)


def _connect_ortho(ax, x1, y1, x2, y2, color=BLACK, linewidth=1.0):
    """Ортогональная ломаная: сначала вертикально до середины, потом горизонтально."""
    mid_y = (y1 + y2) / 2
    ax.plot([x1, x1], [y1, mid_y], color=color, linewidth=linewidth)
    ax.plot([x1, x2], [mid_y, mid_y], color=color, linewidth=linewidth)
    ax.plot([x2, x2], [mid_y, y2], color=color, linewidth=linewidth)


def _actor(ax, x, y, label, color=BLACK):
    """Человечек для Use Case диаграмм."""
    from matplotlib.patches import Circle
    head = Circle((x, y + 0.6), 0.12, facecolor=WHITE, edgecolor=color, linewidth=1.2)
    ax.add_patch(head)
    ax.plot([x, x], [y + 0.48, y + 0.05], color=color, linewidth=1.4)
    ax.plot([x - 0.2, x + 0.2], [y + 0.35, y + 0.35], color=color, linewidth=1.4)
    ax.plot([x, x - 0.15], [y + 0.05, y - 0.2], color=color, linewidth=1.4)
    ax.plot([x, x + 0.15], [y + 0.05, y - 0.2], color=color, linewidth=1.4)
    ax.text(x, y - 0.35, label, ha="center", va="top", fontsize=10, fontweight="bold", color=BLACK)


# ============================================================
# 1. Сравнение технологий Vue / React / Svelte
# ============================================================

def gen_tech_comparison():
    fig, axes = plt.subplots(1, 4, figsize=(14, 4.5), dpi=DPI)
    fig.suptitle("Сравнение фронтенд-фреймворков: Vue, React, Svelte",
                 fontsize=14, fontweight="bold", y=1.01)

    tech = ["Vue.js 3", "React 18", "Svelte 4"]
    # Vue — синий бренд (выбран), остальные — белые с чёрным контуром
    colors = [BRAND_BLUE, WHITE, WHITE]

    metrics = [
        ("Размер бандла, КБ gzip\n(меньше лучше)", [33, 44, 12]),
        ("Производительность,\n% (больше лучше)", [92, 88, 96]),
        ("Кривая обучения,\nбалл (меньше лучше)", [4, 7, 3]),
        ("Экосистема,\nтыс. пакетов (больше)", [52, 180, 8]),
    ]

    for i, (title, values) in enumerate(metrics):
        ax = axes[i]
        bars = ax.bar(tech, values, color=colors, edgecolor=BLACK, linewidth=1.3)
        ax.set_title(title, fontsize=10, color=BLACK)
        ax.grid(axis="y", linestyle="-", linewidth=0.4, color=LIGHT)
        ax.set_axisbelow(True)
        for b, v in zip(bars, values):
            ax.text(b.get_x() + b.get_width() / 2, v + max(values) * 0.02, str(v),
                    ha="center", fontsize=9, fontweight="bold", color=BLACK)
        ax.tick_params(axis="x", labelsize=9, colors=BLACK)
        ax.tick_params(axis="y", labelsize=8, colors=BLACK)
        # рамка — только чёрная
        for spine in ax.spines.values():
            spine.set_color(BLACK)
            spine.set_linewidth(0.8)

    plt.tight_layout()
    out = IMAGES / "1.10_tech_comparison.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor="white")
    plt.close()
    return out


# ============================================================
# 2. User flow регистрации
# ============================================================

def gen_user_flow_registration():
    fig, ax = plt.subplots(figsize=(12, 8), dpi=DPI)
    ax.set_xlim(0, 12)
    ax.set_ylim(0, 10)
    ax.axis("off")
    ax.set_title("Пользовательский поток: регистрация и выбор тарифа",
                 fontsize=13, fontweight="bold")

    # старт
    _ellipse(ax, 4.5, 9, 3, 0.7, "Посещение лендинга", color="#D1FAE5", edgecolor="#065F46")
    # кнопка войти
    _box(ax, 4.5, 7.8, 3, 0.7, "Нажатие «Войти / Зарегистрироваться»")
    _arrow(ax, 6, 9, 6, 8.5)
    # форма регистрации
    _box(ax, 4.5, 6.6, 3, 0.7, "Ввод email и пароля")
    _arrow(ax, 6, 7.8, 6, 7.3)
    # письмо
    _box(ax, 4.5, 5.4, 3, 0.7, "Отправка письма подтверждения")
    _arrow(ax, 6, 6.6, 6, 6.1)
    # проверка
    _diamond(ax, 4.5, 4, 3, 1, "Email подтверждён?")
    _arrow(ax, 6, 5.4, 6, 5)
    # ждём
    _box(ax, 0.5, 4.2, 3, 0.7, "Повторная отправка письма", color="#FEF2F2", edgecolor="#991B1B")
    _arrow(ax, 4.5, 4.5, 3.5, 4.5, label="нет")
    _arrow(ax, 2, 4.2, 2, 5.4, color="#991B1B")
    _arrow(ax, 2, 5.4, 4.5, 5.75, color="#991B1B")

    # выбор тарифа
    _box(ax, 4.5, 2.7, 3, 0.7, "Выбор тарифа (тест / базовый / про)")
    _arrow(ax, 6, 4, 6, 3.4, label="да")
    # развилка платный?
    _diamond(ax, 4.5, 1.3, 3, 1, "Тариф платный?")
    _arrow(ax, 6, 2.7, 6, 2.3)
    # биллинг
    _box(ax, 8.5, 1.5, 3, 0.7, "Переход в биллинг ЮKassa", color="#FEF3C7", edgecolor="#92400E")
    _arrow(ax, 7.5, 1.8, 8.5, 1.85, label="да")
    _box(ax, 8.5, 0.5, 3, 0.7, "Оплата и webhook", color="#FEF3C7", edgecolor="#92400E")
    _arrow(ax, 10, 1.5, 10, 1.2)
    # активация
    _ellipse(ax, 4.5, 0.1, 3, 0.7, "Рабочий стол сервиса", color="#DBEAFE", edgecolor="#1D4E89")
    _arrow(ax, 6, 1.3, 6, 0.8, label="нет")
    _arrow(ax, 10, 0.5, 7.5, 0.4)

    out = IMAGES / "2.9_user_flow_registration.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor="white")
    plt.close()
    return out


# ============================================================
# 3. User flow анализа документа
# ============================================================

def gen_user_flow_analysis():
    fig, ax = plt.subplots(figsize=(12, 9), dpi=DPI)
    ax.set_xlim(0, 12)
    ax.set_ylim(0, 11)
    ax.axis("off")
    ax.set_title("Пользовательский поток: загрузка и анализ документа",
                 fontsize=13, fontweight="bold")

    _ellipse(ax, 4.5, 10, 3, 0.7, "Рабочий стол сервиса", color="#D1FAE5", edgecolor="#065F46")
    _box(ax, 4.5, 8.8, 3, 0.7, "Drag&Drop или выбор файла")
    _arrow(ax, 6, 10, 6, 9.5)
    _diamond(ax, 4.5, 7.3, 3, 1, "Формат корректен?")
    _arrow(ax, 6, 8.8, 6, 8.3)
    _box(ax, 0.5, 7.5, 3, 0.7, "Сообщение об ошибке", color="#FEF2F2", edgecolor="#991B1B")
    _arrow(ax, 4.5, 7.8, 3.5, 7.85, label="нет")
    _arrow(ax, 2, 7.5, 2, 9.15, color="#991B1B")
    _arrow(ax, 2, 9.15, 4.5, 9.15, color="#991B1B")

    _box(ax, 4.5, 6, 3, 0.7, "Загрузка файла на сервер")
    _arrow(ax, 6, 7.3, 6, 6.7, label="да")
    _box(ax, 4.5, 4.8, 3, 0.7, "Запрос к DeepSeek API", color="#E0E7FF", edgecolor="#3730A3")
    _arrow(ax, 6, 6, 6, 5.5)
    _box(ax, 4.5, 3.6, 3, 0.7, "Прогресс анализа (polling)")
    _arrow(ax, 6, 4.8, 6, 4.3)
    _box(ax, 4.5, 2.3, 3, 0.8, "Отображение рисков\nна документе")
    _arrow(ax, 6, 3.6, 6, 3.1)
    _box(ax, 4.5, 1, 3, 0.8, "Панель замечаний\nсправа от документа")
    _arrow(ax, 6, 2.3, 6, 1.8)
    _ellipse(ax, 4.5, 0.1, 3, 0.7, "Работа с результатами", color="#DBEAFE", edgecolor="#1D4E89")
    _arrow(ax, 6, 1, 6, 0.8)

    out = IMAGES / "2.10_user_flow_analysis.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor="white")
    plt.close()
    return out


# ============================================================
# 4. User flow работы с результатами
# ============================================================

def gen_user_flow_results():
    fig, ax = plt.subplots(figsize=(13, 8), dpi=DPI)
    ax.set_xlim(0, 13)
    ax.set_ylim(0, 9)
    ax.axis("off")
    ax.set_title("Пользовательский поток: работа с результатами и ИИ-помощником",
                 fontsize=13, fontweight="bold")

    _ellipse(ax, 5, 8, 3, 0.7, "Открыт документ с рисками", color="#D1FAE5", edgecolor="#065F46")
    _box(ax, 5, 6.8, 3, 0.7, "Клик на подсвеченный риск")
    _arrow(ax, 6.5, 8, 6.5, 7.5)
    _box(ax, 5, 5.6, 3, 0.7, "Отображение пояснения")
    _arrow(ax, 6.5, 6.8, 6.5, 6.3)

    _diamond(ax, 5, 4.2, 3, 1, "Нужен уточняющий вопрос?")
    _arrow(ax, 6.5, 5.6, 6.5, 5.2)

    # ветка с ИИ
    _box(ax, 9, 4.3, 3.5, 0.7, "Задать вопрос ИИ-помощнику", color="#E0E7FF", edgecolor="#3730A3")
    _arrow(ax, 8, 4.7, 9, 4.65, label="да")
    _box(ax, 9, 3.3, 3.5, 0.7, "Ответ DeepSeek", color="#E0E7FF", edgecolor="#3730A3")
    _arrow(ax, 10.75, 4.3, 10.75, 4)

    _box(ax, 5, 2.8, 3, 0.7, "Заметка + изменение статуса")
    _arrow(ax, 6.5, 4.2, 6.5, 3.5, label="нет")
    _arrow(ax, 10.75, 3.3, 8, 3.1)

    _diamond(ax, 5, 1.2, 3, 1, "Все риски просмотрены?")
    _arrow(ax, 6.5, 2.8, 6.5, 2.2)
    # цикл назад к риску
    _arrow(ax, 5, 1.7, 1, 5, label="нет", color="#991B1B")
    _arrow(ax, 1, 5, 1, 7.15, color="#991B1B")
    _arrow(ax, 1, 7.15, 5, 7.15, color="#991B1B")

    _ellipse(ax, 5, 0, 3, 0.8, "Экспорт PDF-отчёта", color="#DBEAFE", edgecolor="#1D4E89")
    _arrow(ax, 6.5, 1.2, 6.5, 0.8, label="да")

    out = IMAGES / "2.11_user_flow_results.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor="white")
    plt.close()
    return out


# ============================================================
# 5. Swimlane биллинга
# ============================================================

def gen_activity_billing_swimlane():
    fig, ax = plt.subplots(figsize=(14, 8), dpi=DPI)
    ax.set_xlim(0, 14)
    ax.set_ylim(0, 10)
    ax.axis("off")
    ax.set_title("Диаграмма активности (swimlane): обработка платежа за подписку",
                 fontsize=13, fontweight="bold")

    lanes = ["Пользователь", "Фронтенд", "API-сервис", "Биллинг", "ЮKassa"]
    n = len(lanes)
    lw = 14 / n
    # дорожки
    colors = ["#EEF2FF", "#F0F9FF", "#ECFDF5", "#FEF3C7", "#FEE2E2"]
    for i, (label, color) in enumerate(zip(lanes, colors)):
        x = i * lw
        rect = Rectangle((x, 0), lw, 9, facecolor=color, edgecolor="#6B7280", linewidth=1)
        ax.add_patch(rect)
        ax.text(x + lw / 2, 9.4, label, ha="center", va="center", fontsize=11, fontweight="bold")

    # шаги (lane_index, y, text)
    steps = [
        (0, 8.3, "Выбор тарифа\nв ЛК"),
        (1, 8.3, "POST /billing/\norder"),
        (2, 8.3, "Создание\nзаказа в БД"),
        (3, 8.3, "Регистрация\nплатежа"),
        (4, 8.3, "Возврат\nplatform_id"),
        (3, 7.0, "Сохранение\nplatform_id"),
        (2, 7.0, "Возврат URL\nоплаты"),
        (1, 7.0, "Redirect на\nstrana.yookassa"),
        (0, 7.0, "Форма оплаты\nна ЮKassa"),
        (4, 5.7, "Обработка\nплатежа"),
        (4, 4.4, "Webhook\nна API"),
        (3, 4.4, "Проверка\nподписи"),
        (3, 3.1, "Обновление\nстатуса в БД"),
        (3, 1.8, "Продление\nподписки"),
        (1, 1.8, "SSE/полл:\nстатус активен"),
        (0, 1.8, "Видит\nактивную\nподписку"),
    ]
    boxes = []
    for i, y, txt in steps:
        x = i * lw + 0.25
        w = lw - 0.5
        _box(ax, x, y, w, 1.0, txt, fontsize=9)
        boxes.append((x + w / 2, y + 0.5))

    # стрелки между шагами (по порядку)
    for a, b in zip(boxes, boxes[1:]):
        _arrow(ax, a[0], a[1] - 0.5, b[0], b[1] + 0.5)

    out = IMAGES / "2.12_activity_billing_swimlane.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor="white")
    plt.close()
    return out


# ============================================================
# 6. Use Case: администрирование и биллинг
# ============================================================

def gen_usecase_admin():
    fig, ax = plt.subplots(figsize=(13, 8), dpi=DPI)
    ax.set_xlim(0, 13)
    ax.set_ylim(0, 9)
    ax.axis("off")
    ax.set_title("Диаграмма прецедентов: администрирование и биллинг",
                 fontsize=13, fontweight="bold")

    # Система (границы)
    ax.add_patch(Rectangle((3.5, 0.5), 6, 8, facecolor="#F9FAFB", edgecolor="#6B7280",
                           linewidth=1.5, linestyle="--"))
    ax.text(6.5, 8.2, "Система «Dockee: Администрирование»",
            ha="center", fontsize=11, fontweight="bold", style="italic")

    # Акторы слева
    _actor(ax, 1.5, 7, "Администратор", color="#1D4E89")
    _actor(ax, 1.5, 4, "Менеджер\nподдержки", color="#065F46")

    # Актор справа (внешняя система)
    _actor(ax, 11.5, 4, "Биллинг\n(ЮKassa)", color="#92400E")

    # Use cases (эллипсы)
    use_cases = [
        (4, 7.3, 2, 0.8, "Управление\nпользователями"),
        (6.5, 7.3, 2, 0.8, "Блокировка\nпользователя"),
        (4, 6.0, 2, 0.8, "Настройка\nтарифов"),
        (6.5, 6.0, 2, 0.8, "Модерация\nжалоб"),
        (4, 4.7, 2, 0.8, "Просмотр\nметрик сервиса"),
        (6.5, 4.7, 2, 0.8, "Подтвердить\nблокировку"),
        (4, 3.4, 2.5, 0.8, "Активация\nenterprise-тарифа"),
        (7, 3.4, 2, 0.8, "Проверить\nоплату"),
        (4, 2.1, 2, 0.8, "Обработка\nплатежа"),
        (6.5, 2.1, 2, 0.8, "Формирование\nсчёта"),
        (4, 0.9, 2, 0.8, "Автопродление\nподписки"),
        (6.5, 0.9, 2, 0.8, "Начисление\nреферального\nбонуса"),
    ]
    for x, y, w, h, t in use_cases:
        _ellipse(ax, x, y, w, h, t, fontsize=9)

    # связи актор-прецедент
    # админ → верхние 5
    for ux, uy in [(4, 7.7), (6.5, 7.7), (4, 6.4), (6.5, 6.4), (4, 5.1)]:
        ax.plot([1.7, ux], [7.3, uy], color="#1D4E89", linewidth=1)
    # менеджер → активация, проверка
    ax.plot([1.7, 4], [4.3, 3.8], color="#065F46", linewidth=1)

    # include: блокировка → подтвердить
    _arrow(ax, 7.5, 7.3, 7.5, 5.1, label="«include»", color="#6B7280")
    # include: активация → проверить оплату
    _arrow(ax, 6.5, 3.8, 7, 3.8, label="«include»", color="#6B7280")

    # биллинг ←→ обработка платежа / формирование / автопродление / реф
    for ux, uy in [(6, 2.5), (8.5, 2.5), (6, 1.3), (8.5, 1.3)]:
        ax.plot([11.3, ux], [4.2, uy], color="#92400E", linewidth=1)

    out = IMAGES / "2.13_usecase_admin.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor="white")
    plt.close()
    return out


# ============================================================
# 7. Site map
# ============================================================

def gen_sitemap():
    fig, ax = plt.subplots(figsize=(15, 10), dpi=DPI)
    ax.set_xlim(0, 15)
    ax.set_ylim(0, 11)
    ax.axis("off")
    ax.set_title("Структура приложения (site map) сервиса Dockee",
                 fontsize=13, fontweight="bold", color=BLACK)

    # корень — синий акцент
    _box(ax, 6, 9.8, 3, 0.7, "Dockee (корневой домен)",
         fontsize=11, fontweight="bold", accent=True)

    # 3 раздела верхнего уровня — белые с чёрными контурами
    sections = [
        (1, 8.3, 3.5, 0.7, "Публичная часть"),
        (5.5, 8.3, 3.5, 0.7, "Сервис (после входа)"),
        (10, 8.3, 3.5, 0.7, "Админ-панель"),
    ]
    for x, y, w, h, t in sections:
        _box(ax, x, y, w, h, t, fontsize=10, fontweight="bold")
        # прямая ортогональная линия от корня
        _connect_ortho(ax, 7.5, 9.8, x + w / 2, y + h)

    # публичная часть
    public = [
        (0.5, 7.1, "Лендинг /"),
        (0.5, 6.3, "Тарифы /pricing"),
        (0.5, 5.5, "FAQ"),
        (0.5, 4.7, "Контакты"),
        (3, 7.1, "Вход /login"),
        (3, 6.3, "Регистрация"),
        (3, 5.5, "Восстановление\nпароля"),
    ]
    for x, y, t in public:
        _box(ax, x, y, 2.1, 0.6, t, fontsize=9)
        _connect_ortho(ax, 2.75, 8.3, x + 1.05, y + 0.6)

    service = [
        (5, 7.1, "Рабочий стол /"),
        (5, 6.3, "Документы /docs"),
        (5, 5.5, "Аналитика"),
        (5, 4.7, "Личный кабинет"),
        (7.5, 7.1, "Загрузка\nи анализ"),
        (7.5, 6.3, "Список, поиск,\nсортировка"),
        (7.5, 5.5, "Графики,\nфильтры"),
        (7.5, 4.7, "Подписка,\nплатежи"),
    ]
    for x, y, t in service:
        _box(ax, x, y, 2.1, 0.6, t, fontsize=9)
        _connect_ortho(ax, 7.25, 8.3, x + 1.05, y + 0.6)

    admin = [
        (9.5, 7.1, "Пользователи"),
        (9.5, 6.3, "Тарифы"),
        (9.5, 5.5, "Жалобы"),
        (12, 7.1, "Метрики"),
        (12, 6.3, "Биллинг\n(логи)"),
        (12, 5.5, "Рассылки"),
    ]
    for x, y, t in admin:
        _box(ax, x, y, 2.1, 0.6, t, fontsize=9)
        _connect_ortho(ax, 11.75, 8.3, x + 1.05, y + 0.6)

    ax.text(7.5, 1.5,
            "Маршруты с префиксом / — публичные; /docs, /analytics, /account — после авторизации; "
            "/admin/* — только роль Admin или Manager.",
            ha="center", fontsize=9, style="italic", color=MUTED)

    out = IMAGES / "2.14_sitemap_full.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor="white")
    plt.close()
    return out


# ============================================================
# 8. Сравнение цветовых палитр
# ============================================================

def gen_color_palettes():
    """
    Только одна палитра (выбранная) показывается цветной.
    Альтернативы представлены как таблица hex-кодов без заливки,
    чтобы весь рисунок оставался в ч/б/бренд-синий.
    """
    chosen = ["#FFFFFF", "#000000", "#6C67FD", "#A2A2A2", "#E6E6E6"]
    chosen_note = "Фон, текст, бренд-акцент, вторичный текст, разделители. Контраст основного текста ≥ 12:1."
    alternatives = [
        ("Б — Юридическая зелёная",
         ["#1A3D2F", "#2D7A5F", "#4CAF50", "#FAFAF8", "#5A5A5A"],
         "Ассоциация «одобрено», экологичность. Контраст ≈ 5.8:1."),
        ("В — Нейтральная тёмная",
         ["#111827", "#374151", "#9CA3AF", "#F3F4F6", "#EF4444"],
         "Тёмный технологичный стиль, красный акцент. Контраст ≥ 11:1."),
    ]

    fig, axes = plt.subplots(3, 1, figsize=(12, 7), dpi=DPI)
    fig.suptitle("Сравнение вариантов цветовой палитры интерфейса Dockee",
                 fontsize=14, fontweight="bold", color=BLACK)

    # А — выбранная, показываем реальными цветами
    ax = axes[0]
    ax.set_xlim(0, 12); ax.set_ylim(0, 2); ax.axis("off")
    ax.text(0, 1.7, "А — Сдержанная с синим акцентом (выбрана)",
            fontsize=12, fontweight="bold", color=BLACK)
    ax.text(0, 0.15, chosen_note, fontsize=10, style="italic", color=BLACK)
    for i, c in enumerate(chosen):
        rect = Rectangle((i * 2.4 + 0.2, 0.5), 2.0, 1.0,
                         facecolor=c, edgecolor=BLACK, linewidth=1.0)
        ax.add_patch(rect)
        text_color = WHITE if c.upper() in ("#000000", "#6C67FD") else BLACK
        ax.text(i * 2.4 + 1.2, 1.0, c.upper(), ha="center", va="center",
                fontsize=10, color=text_color, fontweight="bold")

    # Б, В — только hex-коды в нейтральных плашках
    for ax, (name, colors, note) in zip(axes[1:], alternatives):
        ax.set_xlim(0, 12); ax.set_ylim(0, 2); ax.axis("off")
        ax.text(0, 1.7, name, fontsize=12, fontweight="bold", color=BLACK)
        ax.text(0, 0.15, note, fontsize=10, style="italic", color=BLACK)
        for i, c in enumerate(colors):
            rect = Rectangle((i * 2.4 + 0.2, 0.5), 2.0, 1.0,
                             facecolor=WHITE, edgecolor=BLACK, linewidth=1.0)
            ax.add_patch(rect)
            ax.text(i * 2.4 + 1.2, 1.0, c.upper(), ha="center", va="center",
                    fontsize=10, color=BLACK, fontweight="bold")

    plt.tight_layout(rect=[0, 0, 1, 0.95])
    out = IMAGES / "2.15_color_palettes.png"
    plt.savefig(out, dpi=DPI, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    return out


# ============================================================
# Main
# ============================================================

if __name__ == "__main__":
    IMAGES.mkdir(parents=True, exist_ok=True)
    # gen_user_flow_*, gen_activity_billing_swimlane, gen_usecase_admin
    # намеренно исключены — их выход перезаписывают gen_user_flows.py и gen_plantuml.py
    generators = [
        gen_tech_comparison,
        gen_sitemap,
        gen_color_palettes,
    ]
    for g in generators:
        try:
            path = g()
            size_kb = path.stat().st_size // 1024
            print(f"  OK {path.name}  ({size_kb} KB)")
        except Exception as e:
            print(f"  FAIL {g.__name__}: {e}")
            import traceback
            traceback.print_exc()
