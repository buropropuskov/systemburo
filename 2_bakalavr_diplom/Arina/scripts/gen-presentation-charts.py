"""Генерация диаграмм для презентации Арины.
Сохраняет PNG в /home/washka/project/diplom/Arina/presentation/
"""
from pathlib import Path

import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch, Rectangle, Polygon, Circle
from matplotlib.lines import Line2D

OUT = Path(__file__).resolve().parent.parent / "presentation"
OUT.mkdir(parents=True, exist_ok=True)

GOLD = "#F1D669"
GOLD_LIGHT = "#FAE9A8"
GOLD_DARK = "#D4B340"
NAVY = "#1F3A5F"
NAVY_LIGHT = "#5A7199"
GRAY = "#707070"
GRAY_LIGHT = "#CFCFCF"
WHITE = "#FFFFFF"
BLACK = "#000000"
RED = "#C0392B"
ORANGE = "#E67E22"
YELLOW = "#F39C12"
GREEN = "#27AE60"
BLUE = "#2980B9"


def _save(fig, name: str) -> None:
    p = OUT / name
    fig.savefig(p, dpi=170, bbox_inches="tight", facecolor="white")
    print(f"saved: {p}")


# ---------------- ПИРАМИДА ТЕСТИРОВАНИЯ ----------------
def make_pyramid():
    fig, ax = plt.subplots(figsize=(14, 8))
    ax.set_xlim(0, 20)
    ax.set_ylim(0, 12)
    ax.axis("off")

    cx = 10
    levels = [
        # (width, y, height, color, title, count, time)
        (4.0, 9.2, 1.9, "#E67E22", "E2E (Playwright)", "~25-30 сценариев", "3-5 мин"),
        (7.2, 6.8, 2.1, "#F1C40F", "Интеграционные + API\n(Go testing, Postman/Newman)", "~80 тестов", "2-3 мин"),
        (10.4, 3.9, 2.5, "#27AE60", "Модульные\n(Vitest + Go testing)", "~350 тестов", "40-60 с"),
    ]
    for w, y, h, color, title, count, t in levels:
        left = cx - w / 2
        ax.add_patch(Rectangle((left, y), w, h,
                                facecolor=color, edgecolor=NAVY, lw=2.2, alpha=0.92))
        ax.text(cx, y + h / 2 + 0.25, title, ha="center", va="center",
                fontsize=15, fontweight="bold", color="white")
        ax.text(cx, y + h / 2 - 0.5, count, ha="center", va="center",
                fontsize=13, color="white")
        # annotations справа
        ax.text(cx + w / 2 + 0.6, y + h / 2, f"время: {t}",
                ha="left", va="center", fontsize=13, color=NAVY, fontweight="bold")

    # стрелки-комментарии слева: "дороже/медленнее" ↑, "дешевле/быстрее" ↓
    ax.annotate("", xy=(2.5, 10.0), xytext=(2.5, 4.5),
                arrowprops=dict(arrowstyle="->", color=RED, lw=2.5))
    ax.text(2.2, 7.2, "дороже, медленнее,\nмедленнее запуск",
            ha="right", va="center", fontsize=12, color=RED)

    ax.annotate("", xy=(17.5, 4.5), xytext=(17.5, 10.0),
                arrowprops=dict(arrowstyle="->", color=GREEN, lw=2.5))
    ax.text(17.8, 7.2, "быстрее, дешевле,\nбольше покрытие",
            ha="left", va="center", fontsize=12, color=GREEN)

    # нижняя подпись
    ax.text(cx, 2.8, "Общее время прогона в CI ≤ 10 минут за счёт параллелизации",
            ha="center", va="center", fontsize=13, color=NAVY,
            fontweight="bold", style="italic")

    plt.tight_layout()
    _save(fig, "pres_pyramid.png")
    plt.close(fig)


# ---------------- CI/CD ПАЙПЛАЙН ----------------
def make_cicd():
    fig, ax = plt.subplots(figsize=(15, 7.5))
    ax.set_xlim(0, 30)
    ax.set_ylim(0, 12)
    ax.axis("off")

    def draw_stage(x, y, w, h, title, sub, color=GOLD):
        ax.add_patch(FancyBboxPatch((x, y), w, h,
                                      boxstyle="round,pad=0.05,rounding_size=0.15",
                                      facecolor=color, edgecolor=NAVY, lw=1.8))
        ax.text(x + w / 2, y + h / 2 + 0.3, title, ha="center", va="center",
                fontsize=13, fontweight="bold", color=NAVY)
        ax.text(x + w / 2, y + h / 2 - 0.45, sub, ha="center", va="center",
                fontsize=11, color=NAVY)

    def arrow(x1, y1, x2, y2):
        ax.annotate("", xy=(x2, y2), xytext=(x1, y1),
                    arrowprops=dict(arrowstyle="->", color=NAVY, lw=2))

    # левая колонка: бэкенд
    ax.text(1.5, 11.0, "Бэкенд (Go)", fontsize=14, fontweight="bold", color=NAVY)
    ax.text(1.5, 10.4, "git push → bekkend-repo", fontsize=10, color=GRAY, style="italic")

    bx = 0.5
    by = 8.4
    draw_stage(bx, by, 5.3, 1.4, "1. golangci-lint", "23 с")
    draw_stage(bx, by - 2.0, 5.3, 1.4, "2. go test -race", "модульные + интегр., 2-3 мин")
    draw_stage(bx, by - 4.0, 5.3, 1.4, "3. Newman (Postman)", "34 API-сценария, 1-2 мин")
    draw_stage(bx, by - 6.0, 5.3, 1.4, "4. Docker build & push", "GitHub Container Registry", color=GOLD_LIGHT)
    arrow(bx + 2.65, by, bx + 2.65, by - 0.6)
    arrow(bx + 2.65, by - 2.0, bx + 2.65, by - 2.6)
    arrow(bx + 2.65, by - 4.0, bx + 2.65, by - 4.6)

    # правая колонка: фронтенд
    ax.text(8.5, 11.0, "Фронтенд (Vue.js)", fontsize=14, fontweight="bold", color=NAVY)
    ax.text(8.5, 10.4, "git push → frontend-repo", fontsize=10, color=GRAY, style="italic")

    fx = 7.5
    draw_stage(fx, by, 5.3, 1.4, "1. ESLint", "14 с")
    draw_stage(fx, by - 2.0, 5.3, 1.4, "2. Vitest (coverage)", "85 юнитов, ~5 с")
    draw_stage(fx, by - 4.0, 5.3, 1.4, "3. Playwright (headless)", "E2E-сценарии, ~1:15")
    draw_stage(fx, by - 6.0, 5.3, 1.4, "4. Build & deploy", "staging.dockee.ru", color=GOLD_LIGHT)
    arrow(fx + 2.65, by, fx + 2.65, by - 0.6)
    arrow(fx + 2.65, by - 2.0, fx + 2.65, by - 2.6)
    arrow(fx + 2.65, by - 4.0, fx + 2.65, by - 4.6)

    # итоги справа
    ax.add_patch(FancyBboxPatch((15.0, 1.8), 13.8, 8.8,
                                  boxstyle="round,pad=0.05,rounding_size=0.2",
                                  facecolor=GOLD_LIGHT, edgecolor=NAVY, lw=2))
    ax.text(21.9, 10.0, "Итого по пайплайну", ha="center", va="center",
            fontsize=15, fontweight="bold", color=NAVY)

    rows = [
        ("Бэкенд, полный прогон", "7 мин 42 с"),
        ("Фронтенд, полный прогон", "4 мин 18 с"),
        ("Стабильность за 30 дней", "81,6% (71 из 87)"),
        ("Покрытие бэкенда", "74,1%"),
        ("Покрытие фронтенда", "72,3%"),
        ("Всего тестов", "246"),
        ("Fail fast на lint-этапе", "экономит 6 минут при ошибке"),
    ]
    y0 = 9.1
    for k, v in rows:
        ax.text(15.8, y0, "•", fontsize=14, color=NAVY, fontweight="bold")
        ax.text(16.4, y0, k, fontsize=12, color=NAVY, va="center")
        ax.text(28.0, y0, v, fontsize=12, fontweight="bold", color=NAVY,
                va="center", ha="right")
        y0 -= 0.95

    # триггер
    ax.text(14.5, 0.9,
            "Результат: push → 4-8 минут → зелёный прогон → уведомление в Telegram",
            ha="center", fontsize=12, color=NAVY, fontweight="bold", style="italic")

    plt.tight_layout()
    _save(fig, "pres_cicd.png")
    plt.close(fig)


# ---------------- РАСПРЕДЕЛЕНИЕ ДЕФЕКТОВ ----------------
def make_defects():
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 6.5),
                                     gridspec_kw={"width_ratios": [1, 1.3]})

    # --- pie: по критичности ---
    sizes = [3, 6, 5, 3]
    labels = ["Критические (3)", "Высокие (6)", "Средние (5)", "Низкие (3)"]
    colors = [RED, ORANGE, YELLOW, GREEN]
    wedges, texts, autotexts = ax1.pie(
        sizes, labels=labels, autopct="%1.0f%%", colors=colors,
        textprops={"fontsize": 12, "color": NAVY},
        startangle=90, wedgeprops=dict(width=0.55, edgecolor="white", linewidth=2))
    for at in autotexts:
        at.set_color("white")
        at.set_fontweight("bold")
        at.set_fontsize(12)
    ax1.set_title("Найдено 17 дефектов", fontsize=14, fontweight="bold", color=NAVY, pad=16)

    # --- bar: по уровню тестирования ---
    levels = ["Модульн.", "Интеграц.", "E2E", "Безопас.", "Юзабилити", "Совместим."]
    counts = [4, 1, 3, 2, 3, 4]
    ax2.barh(levels, counts, color=[GOLD, GOLD_DARK, NAVY_LIGHT, RED, BLUE, GREEN],
             edgecolor=NAVY, linewidth=1.2)
    for i, c in enumerate(counts):
        ax2.text(c + 0.12, i, str(c), va="center", fontsize=12,
                 fontweight="bold", color=NAVY)
    ax2.set_title("Дефекты по уровню тестирования", fontsize=14,
                  fontweight="bold", color=NAVY, pad=16)
    ax2.set_xlim(0, max(counts) + 1.2)
    ax2.tick_params(axis="both", labelsize=12, colors=NAVY)
    ax2.spines["top"].set_visible(False)
    ax2.spines["right"].set_visible(False)
    ax2.invert_yaxis()

    plt.tight_layout()
    _save(fig, "pres_defects.png")
    plt.close(fig)


# ---------------- АРХИТЕКТУРА DOCKEE ----------------
def make_architecture():
    fig, ax = plt.subplots(figsize=(14, 7.5))
    ax.set_xlim(0, 24)
    ax.set_ylim(0, 14)
    ax.axis("off")

    def box(x, y, w, h, title, sub, color=GOLD):
        ax.add_patch(FancyBboxPatch((x, y), w, h,
                                      boxstyle="round,pad=0.05,rounding_size=0.2",
                                      facecolor=color, edgecolor=NAVY, lw=1.8))
        ax.text(x + w / 2, y + h / 2 + 0.45, title, ha="center", va="center",
                fontsize=13, fontweight="bold", color=NAVY)
        if sub:
            ax.text(x + w / 2, y + h / 2 - 0.55, sub, ha="center", va="center",
                    fontsize=11, color=NAVY)

    def arrow(x1, y1, x2, y2, label=""):
        ax.annotate("", xy=(x2, y2), xytext=(x1, y1),
                    arrowprops=dict(arrowstyle="->", color=NAVY, lw=1.8))
        if label:
            ax.text((x1 + x2) / 2, (y1 + y2) / 2 + 0.35, label,
                    ha="center", fontsize=10, style="italic", color=GRAY)

    # клиент
    box(0.5, 10.5, 5.5, 2.4, "Браузер", "Vue.js 3 + Pinia\nvue-pdf, mammoth.js", GOLD_LIGHT)
    # сервер — модульный монолит
    ax.add_patch(FancyBboxPatch((7.5, 3.6), 10.0, 9.3,
                                  boxstyle="round,pad=0.08,rounding_size=0.25",
                                  facecolor="white", edgecolor=NAVY, lw=2.2,
                                  linestyle="--"))
    ax.text(12.5, 12.5, "Сервер Go + Gin (модульный монолит)",
            ha="center", fontsize=13, fontweight="bold", color=NAVY)

    modules = [
        (8.0, 10.2, "Модуль\nаутентификации", "JWT + refresh"),
        (12.8, 10.2, "Модуль документов", "загрузка + статус"),
        (8.0, 7.5, "Модуль AI", "DeepSeek, NER"),
        (12.8, 7.5, "Модуль отчётов", "PDF, DOCX"),
    ]
    for x, y, t, s in modules:
        box(x, y, 4.3, 2.0, t, s, GOLD)

    # middleware
    box(8.0, 4.2, 9.1, 1.6, "Middleware",
        "auth • rate-limit • логи • CORS", GOLD_DARK)

    # Внешние системы
    box(19.0, 10.5, 4.5, 2.4, "PostgreSQL", "пользователи,\nдокументы, риски", GOLD_LIGHT)
    box(19.0, 7.2, 4.5, 2.4, "S3-хранилище", "файлы документов", GOLD_LIGHT)
    box(19.0, 3.9, 4.5, 2.4, "DeepSeek API", "AI-анализ\n(вне контура)", GOLD_LIGHT)

    # стрелки
    arrow(6.0, 11.7, 7.9, 11.7, "HTTPS + JWT")
    arrow(6.0, 11.2, 7.9, 8.3, "WebSocket")

    arrow(17.5, 11.4, 19.0, 11.4)
    arrow(17.5, 10.5, 19.0, 8.2)
    arrow(17.5, 7.8, 19.0, 4.9)

    # примечание
    ax.text(12.0, 2.5,
            "Все модули — в одном процессе, слабо связаны через интерфейсы. Тестируются независимо.",
            ha="center", fontsize=12, color=NAVY, style="italic")

    plt.tight_layout()
    _save(fig, "pres_architecture.png")
    plt.close(fig)


# ---------------- РЕЗУЛЬТАТЫ (дашборд) ----------------
def make_results():
    fig = plt.figure(figsize=(14, 7.5))
    gs = fig.add_gridspec(2, 3, hspace=0.45, wspace=0.35)

    # карточки-цифры
    titles = [
        ("246", "тестов в проекте", GOLD),
        ("73%", "покрытие кода", GOLD_DARK),
        ("17", "дефектов найдено\n(3 критических устранены)", GOLD),
    ]
    for i, (big, sub, col) in enumerate(titles):
        ax = fig.add_subplot(gs[0, i])
        ax.add_patch(FancyBboxPatch((0.03, 0.05), 0.94, 0.9,
                                      boxstyle="round,pad=0.02,rounding_size=0.02",
                                      facecolor=col, edgecolor=NAVY, lw=1.8,
                                      transform=ax.transAxes))
        ax.text(0.5, 0.62, big, ha="center", va="center", fontsize=44,
                fontweight="bold", color=NAVY, transform=ax.transAxes)
        ax.text(0.5, 0.25, sub, ha="center", va="center", fontsize=13,
                color=NAVY, transform=ax.transAxes)
        ax.axis("off")

    # снизу — bar по эндпоинтам
    ax4 = fig.add_subplot(gs[1, :])
    eps = ["GET /profile", "GET /documents", "auth/login",
           "POST /upload (1 МБ)", "ai/chat"]
    p50 = [6, 12, 34, 186, 1847]
    p95 = [14, 29, 67, 412, 4213]
    import numpy as np
    x = np.arange(len(eps))
    width = 0.35
    b1 = ax4.bar(x - width / 2, p50, width, label="медиана, мс",
                 color=GOLD, edgecolor=NAVY)
    b2 = ax4.bar(x + width / 2, p95, width, label="p95, мс",
                 color=NAVY_LIGHT, edgecolor=NAVY)
    for rect, v in zip(b1, p50):
        ax4.text(rect.get_x() + rect.get_width() / 2, rect.get_height() + 30,
                 f"{v}", ha="center", fontsize=11, color=NAVY, fontweight="bold")
    for rect, v in zip(b2, p95):
        ax4.text(rect.get_x() + rect.get_width() / 2, rect.get_height() + 30,
                 f"{v}", ha="center", fontsize=11, color=NAVY, fontweight="bold")
    ax4.axhline(200, color=RED, linestyle="--", lw=2, alpha=0.7)
    ax4.text(4.3, 260, "требование: p95 ≤ 200 мс", color=RED, fontsize=11,
             fontweight="bold", ha="right")
    ax4.set_xticks(x)
    ax4.set_xticklabels(eps, fontsize=11, color=NAVY)
    ax4.set_ylabel("время, мс", fontsize=12, color=NAVY)
    ax4.set_title("Нагрузочное тестирование, 100 параллельных пользователей",
                  fontsize=13, fontweight="bold", color=NAVY, pad=10)
    ax4.legend(fontsize=11, loc="upper left")
    ax4.set_yscale("log")
    ax4.grid(True, axis="y", alpha=0.3)
    ax4.spines["top"].set_visible(False)
    ax4.spines["right"].set_visible(False)

    plt.tight_layout()
    _save(fig, "pres_results.png")
    plt.close(fig)


if __name__ == "__main__":
    make_pyramid()
    make_cicd()
    make_defects()
    make_architecture()
    make_results()
    print("\n✅ Все диаграммы сгенерированы в", OUT)
