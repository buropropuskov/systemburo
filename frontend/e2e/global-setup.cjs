const { chromium } = require('@playwright/test');

/**
 * Прогрев vite dev-сервера перед прогоном E2E.
 *
 * В CI фронт поднимается как vite dev-сервер. Health-check курлит только index.html
 * (отвечает мгновенно) и НЕ триггерит dep-оптимизацию vite - она запускается на первом
 * запросе модуля и блокирует его до конца пребандла. Когда параллельные воркеры разом
 * делают первые навигации к холодному vite, они упираются в это одновременно, и самые
 * ранние навигации шарда проигрывают 5s-таймауту toHaveURL (флак /500, /maintenance:
 * приложение не успевает смонтироваться, URL остаётся на корне).
 *
 * Один заход браузером по публичным маршрутам проходит главный граф модулей и чанки
 * этих роутов, снимая холодную стоимость до старта тестов. Best-effort: ошибки прогрева
 * не валят прогон (если сервер реально лежит - тесты упадут сами и покажут это явно).
 */
module.exports = async () => {
  const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8081';
  const routes = ['/', '/500', '/maintenance'];

  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({ baseURL });
    for (const route of routes) {
      try {
        await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 90000 });
        // Дождаться монтирования SPA (#app получил содержимое) - значит главный граф
        // и чанк роута уже скомпилированы vite-ом.
        await page.waitForSelector('#app > *', { timeout: 60000 });
      } catch (err) {
        console.warn(`[e2e warmup] ${route}: ${err.message}`);
      }
    }
  } finally {
    await browser.close();
  }
};
