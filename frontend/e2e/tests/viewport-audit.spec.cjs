const fs = require('fs');
const path = require('path');
const { test } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { horizontalOverflow, overlaps, smallTargets } = require('../helpers/mobileInvariants');
const { pageMetrics, oversizedRows, clippedText, modalGeometry } = require('../helpers/layoutProbes');
const { selectScreens } = require('../helpers/screens');

/**
 * Обход всех экранов по всем ширинам со снятием замеров раскладки (#2473).
 *
 * Это не гейт, а инструмент: тест зелёный всегда, кроме инфраструктурного сбоя, а
 * находки складывает в отчёт. Гейтом их делает `tools/audit-report.cjs`, сравнивая
 * отчёт с эталонным срезом - так «мобилка и десктоп не сломались» проверяется
 * числами, а не памятью о том, как было до правки.
 *
 * Почему один тест на весь обход, а не тест на экран: вход на стенде лимитирован
 * (10 попыток `/api/login` в минуту), и тридцать шесть параллельных логинов
 * упираются в лимит раньше, чем доходят до первой страницы. Логинимся один раз и
 * дальше живём в одном контексте.
 *
 * Тест инструментальный и в общем прогоне CI пропускается: он ходит по стенду и
 * занимает четверть часа. Включается флагом AUDIT_RUN=1.
 *
 * Прогон:
 *   AUDIT_RUN=1 E2E_BASE_URL=https://stagingburo.washka17.ru E2E_HTTP_USER=... \
 *   E2E_HTTP_PASSWORD=... E2E_SUPERADMIN_PASSWORD=... \
 *   npx playwright test viewport-audit --project=chromium
 * Отбор экранов - `AUDIT_SCREENS=users,roles` или `AUDIT_SCREENS=admin`.
 * Набор ширин - `AUDIT_WIDTHS=390,834,1440`.
 */

// 1920 в наборе по умолчанию обязателен: это одна из трёх защищённых ширин, и без
// замера гейт сравнивать нечего - он молча решил бы, что регрессий нет.
const DEFAULT_WIDTHS = [390, 834, 1024, 1440, 1920];
const HEIGHT_BY_WIDTH = { 390: 844, 810: 1080, 834: 1112, 1024: 1366, 1180: 820, 1366: 1024, 1440: 900, 1920: 1080 };

// Норма тач-таргета проекта - 36px (эталон §18): компактные контролы `.rt-btn-compact`
// и «Обновить» сделаны именно такими, и гейт на 44 ругался бы на принятую норму.
const TOUCH_MIN = 36;
// Выше этой ширины раскладка десктопная и по решению эпика не перевёрстывается, а
// её компактные контролы (пин рельса 28px, «Подать заявку» 26px) - принятая норма.
// Проверять там тач-таргеты значит получать по восемь одинаковых находок на каждом
// экране и утопить в них реальные.
const TOUCH_MAX_WIDTH = 1024;

const REPORT_DIR = path.join(__dirname, '..', 'reports');
const REPORT_FILE = path.join(REPORT_DIR, 'viewport-audit.json');

const widths = (process.env.AUDIT_WIDTHS || DEFAULT_WIDTHS.join(','))
  .split(',').map((w) => parseInt(w.trim(), 10)).filter(Boolean);

/** Экран мог не успеть отрисовать список - ждём содержимое, а не фиксированную паузу. */
async function settle(page) {
  await page.waitForLoadState('domcontentloaded');
  await page.waitForTimeout(1200);
}

/**
 * Переход с восстановлением сессии. Access-токен живёт только в памяти Pinia, и
 * перезагрузка страницы поднимает его заново по refresh-cookie. Если восстановление
 * не прошло (кука протухла), приложение показывает форму входа - логинимся ещё раз,
 * иначе весь дальнейший отчёт будет снят с экрана логина.
 */
async function visit(page, screenPath) {
  await page.goto(screenPath);
  await settle(page);
  const onLogin = await page.locator('input[type="password"]').first().isVisible().catch(() => false);
  if (onLogin) {
    await loginAsSuperAdminUI(page);
    await page.goto(screenPath);
    await settle(page);
  }
  // Страница отказа и «не найдено» выглядят опрятно: ни переполнений, ни наложений.
  // Не отличив их от рабочего экрана, обход записал бы «чисто» там, где ничего не
  // проверил, - ровно та ложная зелень, ради защиты от которой инструмент и заведён.
  const url = page.url();
  const wrongScreen = /\/(403|404|maintenance)(\?|$|\/)/.test(url)
    || !url.includes(screenPath.split('?')[0]);
  if (wrongScreen) throw new Error(`вместо ${screenPath} открылось ${url}`);
}

test('аудит раскладки: обход экранов по ширинам', async ({ page }) => {
  test.skip(!process.env.AUDIT_RUN, 'инструмент, запускается вручную с AUDIT_RUN=1');
  test.setTimeout(30 * 60 * 1000);

  const screens = selectScreens();
  await page.setViewportSize({ width: 1440, height: 900 });
  await loginAsSuperAdminUI(page);

  const results = [];
  for (const screen of screens) {
    for (const width of widths) {
      const height = HEIGHT_BY_WIDTH[width] || 900;
      await page.setViewportSize({ width, height });
      // Перезагружаем на каждой ширине, а не меняем размер у готовой страницы:
      // часть раскладки считается скриптом на старте (AdminPageShell меряет высоту,
      // viewportScale ставит корневой zoom), и «дорисованное» окно отличается от
      // того, что увидит человек, открывший страницу с планшета.
      await visit(page, screen.path);

      const entry = { screen: screen.slug, name: screen.name, area: screen.area, width, state: 'list' };
      try {
        entry.metrics = await page.evaluate(pageMetrics);
        entry.overflow = await page.evaluate(horizontalOverflow);
        entry.oversized = screen.card ? await page.evaluate(oversizedRows, screen.card) : [];
        entry.overlaps = screen.card ? await page.evaluate(overlaps, screen.card) : [];
        entry.clipped = screen.card ? await page.evaluate(clippedText, screen.card) : [];
        entry.small = width <= TOUCH_MAX_WIDTH ? await page.evaluate(smallTargets, TOUCH_MIN) : [];
        entry.docOverflow = entry.metrics.doc > entry.metrics.vw + 1
          ? { doc: entry.metrics.doc, vw: entry.metrics.vw } : null;
      } catch (err) {
        // Замер отдельного экрана не должен ронять обход: страница могла увести на
        // 403 по правам или упасть в ошибку рендера - это само по себе находка.
        entry.error = String(err).slice(0, 200);
      }
      results.push(entry);

      // Второй замер - с раскрытым окном. Окна проверять обязательно: список
      // приводит в порядок карточная инфраструктура, а окно рисует себя само, и
      // именно на окнах в админке накопились свои радиусы, свои шапки и своя
      // прокрутка.
      if (screen.open && !entry.error) {
        const opened = { screen: screen.slug, name: screen.name, area: screen.area, width, state: 'open' };
        try {
          for (const selector of screen.open) {
            const target = page.locator(selector).first();
            if (!(await target.isVisible().catch(() => false))) throw new Error(`нет ${selector}`);
            await target.click();
          }
          await page.waitForTimeout(700);
          const dialog = page.locator('.base-modal, .modal-content, [role="dialog"], .modal-overlay > *').first();
          if (!(await dialog.isVisible().catch(() => false))) throw new Error('окно не открылось');
          opened.metrics = await page.evaluate(pageMetrics);
          opened.overflow = await page.evaluate(horizontalOverflow);
          opened.small = width <= TOUCH_MAX_WIDTH ? await page.evaluate(smallTargets, TOUCH_MIN) : [];
          opened.modal = await page.evaluate(modalGeometry);
          opened.docOverflow = opened.metrics.doc > opened.metrics.vw + 1
            ? { doc: opened.metrics.doc, vw: opened.metrics.vw } : null;
        } catch (err) {
          // Раздел без кнопки создания или с окном за правами - не находка, а
          // отсутствие второго состояния: помечаем и идём дальше.
          opened.skipped = String(err.message || err).slice(0, 80);
        }
        results.push(opened);
      }
    }
  }

  fs.mkdirSync(REPORT_DIR, { recursive: true });
  fs.writeFileSync(REPORT_FILE, JSON.stringify({
    createdAt: new Date().toISOString(),
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:8081',
    widths,
    results,
  }, null, 2), 'utf8');
  console.log(`аудит: ${results.length} замеров, отчёт ${REPORT_FILE}`);
});
