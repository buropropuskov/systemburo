/**
 * Утилиты для site-smoke-crawler.spec.cjs.
 * Перехват console/pageerror, фильтрация safe-to-click элементов, helper'ы для безопасной навигации.
 */

// Кнопки/линки которые НЕЛЬЗЯ кликать в смок-обходе:
// - destructive: удаляют, сбрасывают, блокируют, выходят
// - submit: создают/сохраняют сущности (мусор в staging)
// - confirm в модалках
const DESTRUCTIVE_REGEX = /(удалить|сбросить|заблокировать|забанить|очистить|выйти|выход|сохранить|создать|подтвердить|применить|архивировать|разбанить|разблокировать|снять блок|delete|remove|reset|ban|logout|save|create|confirm|apply|archive)/i;

// href которые ведут вовне сайта или меняют контекст
const DANGEROUS_HREF = /^(mailto:|tel:|javascript:|https?:\/\/(?!stagingburo\.washka17\.ru))/i;

// Известные ложные console.error - не считаем регрессией
const EXPECTED_ERRORS_WHITELIST = [
  'ResizeObserver loop limit exceeded',
  'ResizeObserver loop completed with undelivered notifications',
  'Failed to load resource: net::ERR_BLOCKED_BY_CLIENT', // adblock и подобное
  // 401 на /api/refresh-token нормален - SPA пробует refresh при загрузке,
  // если cookie уже актуальна - всё хорошо; если протухла - 401 и редирект на login
  '/api/refresh-token',
  // 401/403 на других /api/* возникают как race при logout-flow
  'status of 401 ()',
  'Missing or invalid authorization header',
];

// Регексы для путей которые часто дают 404 на картинки/превью (но не для критичных API/JS)
const EXPECTED_404_PATHS = [
  /\/uploads\//, // отсутствующие аватары/превью
  /\/static\/.+\.(png|jpg|jpeg|svg|webp)/,
  /\.(png|jpg|jpeg|svg|webp|gif|ico)$/i,
];

function isWhitelisted(message, urlContext = '') {
  if (!message) return false;
  const combined = `${message} ${urlContext}`;
  if (EXPECTED_ERRORS_WHITELIST.some((w) => combined.includes(w))) return true;
  // 404 на картинки - не баг
  if (combined.includes('status of 404') && EXPECTED_404_PATHS.some((r) => r.test(combined))) return true;
  return false;
}

/**
 * Привязывает слушатели к page. Возвращает геттеры для собранных ошибок.
 * Слушатели снимаются автоматически при закрытии page.
 */
function setupErrorCollectors(page) {
  const pageErrors = [];
  const consoleErrors = [];
  const consoleWarnings = [];

  page.on('pageerror', (err) => {
    pageErrors.push({
      message: err.message,
      stack: err.stack,
      at: new Date().toISOString(),
    });
  });

  page.on('console', (msg) => {
    const text = msg.text();
    const location = msg.location();
    if (isWhitelisted(text, location?.url || '')) return;
    const entry = { text, location, at: new Date().toISOString() };
    if (msg.type() === 'error') consoleErrors.push(entry);
    else if (msg.type() === 'warning') consoleWarnings.push(entry);
  });

  return {
    getPageErrors: () => [...pageErrors],
    getConsoleErrors: () => [...consoleErrors],
    getConsoleWarnings: () => [...consoleWarnings],
    snapshot: () => ({
      pageErrors: pageErrors.length,
      consoleErrors: consoleErrors.length,
      consoleWarnings: consoleWarnings.length,
    }),
  };
}

/**
 * Собирает все clickable элементы на странице с фильтрами безопасности.
 * Не дёргает .all() сразу - возвращает Locator чтобы можно было лениво итерировать.
 */
function getClickables(page) {
  // button, a[href], [role="button"], [role="tab"], [role="link"]
  return page.locator(
    'button:visible:not([disabled]), ' +
    'a[href]:visible, ' +
    '[role="button"]:visible:not([aria-disabled="true"]), ' +
    '[role="tab"]:visible, ' +
    '[role="link"]:visible',
  );
}

/**
 * Проверяет является ли клик опасным (destructive/submit/navigation outside).
 */
async function isSafeToClick(locator) {
  try {
    const text = (await locator.innerText({ timeout: 500 }).catch(() => ''))?.trim() || '';
    if (DESTRUCTIVE_REGEX.test(text)) {
      return { safe: false, reason: `destructive text: "${text.slice(0, 50)}"` };
    }
    const href = await locator.getAttribute('href').catch(() => null);
    if (href && DANGEROUS_HREF.test(href)) {
      return { safe: false, reason: `dangerous href: ${href.slice(0, 80)}` };
    }
    return { safe: true, text };
  } catch (e) {
    return { safe: false, reason: `cant-inspect: ${e.message}` };
  }
}

/**
 * Закрывает открытые overlays/модалки/dropdowns через Escape + клик в пустое место.
 * Best-effort, не падает если ничего не открыто.
 */
async function closeOpenedOverlays(page) {
  try {
    await page.keyboard.press('Escape');
    await page.waitForTimeout(150);
    // Иногда модалка не закрывается на Escape - клик по body вне модалки
    const overlay = page.locator('.base-modal-overlay, .modal-overlay, .form-modal, .rename-modal, .permission-tree-modal').first();
    if (await overlay.isVisible({ timeout: 200 }).catch(() => false)) {
      await page.keyboard.press('Escape');
      await page.waitForTimeout(150);
    }
  } catch {
    // ignore
  }
}

/**
 * Кликает по элементу с safety-проверками. Возвращает результат:
 * { clicked, skipped, reason, urlBefore, urlAfter, urlChanged }
 */
async function clickSafe(page, locator) {
  const check = await isSafeToClick(locator);
  if (!check.safe) {
    return { clicked: false, skipped: true, reason: check.reason };
  }
  const urlBefore = page.url();
  try {
    await locator.click({ timeout: 2000, force: false });
    await page.waitForTimeout(300); // ждём реакции UI
  } catch (e) {
    return { clicked: false, skipped: false, reason: `click-failed: ${e.message.slice(0, 100)}` };
  }
  const urlAfter = page.url();
  return {
    clicked: true,
    skipped: false,
    text: check.text,
    urlBefore,
    urlAfter,
    urlChanged: urlBefore !== urlAfter,
  };
}

/**
 * Формирует markdown-фрагмент отчёта для одного route.
 */
function formatRouteReport({ route, results, errors }) {
  const lines = [];
  lines.push(`## ${route.path} (${route.name})\n`);
  lines.push(`- clickables найдено: ${results.totalFound}`);
  lines.push(`- кликов выполнено: ${results.clicked}`);
  lines.push(`- skip по safety: ${results.skipped}`);
  lines.push(`- click failures: ${results.failures.length}`);
  if (errors.pageErrors.length) {
    lines.push(`\n### pageerror (${errors.pageErrors.length})`);
    errors.pageErrors.forEach((e) => lines.push(`- \`${e.message}\``));
  }
  if (errors.consoleErrors.length) {
    lines.push(`\n### console.error (${errors.consoleErrors.length})`);
    errors.consoleErrors.slice(0, 20).forEach((e) => lines.push(`- \`${e.text.slice(0, 200)}\``));
  }
  if (results.failures.length) {
    lines.push(`\n### click failures`);
    results.failures.slice(0, 20).forEach((f) => lines.push(`- "${f.text}": ${f.reason}`));
  }
  return lines.join('\n') + '\n';
}

module.exports = {
  DESTRUCTIVE_REGEX,
  DANGEROUS_HREF,
  EXPECTED_ERRORS_WHITELIST,
  isWhitelisted,
  setupErrorCollectors,
  getClickables,
  isSafeToClick,
  closeOpenedOverlays,
  clickSafe,
  formatRouteReport,
};
