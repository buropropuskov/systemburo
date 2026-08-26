/**
 * Site smoke crawler.
 *
 * Каждый route - отдельный test. Логин раз на worker через beforeAll
 * (StorageState переиспользуется на serial-тестах внутри describe).
 *
 * Алгоритм для каждого route:
 * 1. setupErrorCollectors на page
 * 2. navigate(route)
 * 3. waitForLoadState networkidle (5s soft timeout)
 * 4. собрать clickables
 * 5. кликнуть каждый safe-clickable, после клика - закрыть overlays, вернуться на route если URL изменился
 * 6. assert: 0 pageerror, 0 неотфильтрованных console.error
 *
 * Запуск:
 *   E2E_BASE_URL=https://stagingburo.washka17.ru \
 *   E2E_HTTP_USER=admin E2E_HTTP_PASSWORD=... \
 *   E2E_SUPERADMIN_USER=buropropuskov E2E_SUPERADMIN_PASSWORD=admin123 \
 *     npx playwright test e2e/tests/site-smoke-crawler.spec.cjs --reporter=list
 */

const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const {
  setupErrorCollectors,
  getClickables,
  clickSafe,
  closeOpenedOverlays,
  formatRouteReport,
} = require('../helpers/site-crawler.cjs');

const MAX_CLICKS_PER_PAGE = Number(process.env.E2E_CRAWL_MAX_CLICKS) || 50;
const REPORT_DIR = path.resolve(__dirname, '..', '..', 'site-crawl-reports');

// Список роутов взят из frontend/src/router.js. Пропущены:
// - / (redirect на /news для authenticated)
// - /submit-form (redirect на /new-application)
// - /table (redirect на /personal-cabinet)
// - /table/:tableName (нужен параметр - не unit-навигация)
// - /500, /maintenance, /403 (открываются только по факту события, прямой
//   заход редиректит - краулить нечего, они проверяются отдельной спекой)
const ROUTES = [
  { path: '/news', name: 'NewsAndReview' },
  { path: '/personal-cabinet', name: 'Account' },
  { path: '/carsview', name: 'CarsView' },
  { path: '/center', name: 'ApplicationsCenter' },
  { path: '/employeesview', name: 'EmployeeView' },
  { path: '/table-constructor', name: 'TableConstructor' },
  { path: '/number-format', name: 'NumberFormat' },
  { path: '/new-application', name: 'NewApplication' },
  { path: '/admin/feedback', name: 'FeedbackPage' },
  { path: '/admin/requests', name: 'RequestsView' },
  { path: '/admin/settings', name: 'AdminSettings' },
  { path: '/admin/users', name: 'AdminUsers' },
  { path: '/admin/permission-groups', name: 'AdminPermissionGroups' },
  { path: '/admin/roles', name: 'AdminRoles' },
  { path: '/admin/access-denials', name: 'AccessDenialsLog' },
  { path: '/admin/system-control', name: 'SystemControl' },
];

if (!fs.existsSync(REPORT_DIR)) fs.mkdirSync(REPORT_DIR, { recursive: true });

// browser.newContext() НЕ наследует use.httpCredentials/baseURL из playwright.config.
// Передаём явно из env (нужно для basic-auth на staging nginx).
function siteContextOptions(extra = {}) {
  const opts = { ...extra };
  if (process.env.E2E_BASE_URL) opts.baseURL = process.env.E2E_BASE_URL;
  if (process.env.E2E_HTTP_USER) {
    opts.httpCredentials = {
      username: process.env.E2E_HTTP_USER,
      password: process.env.E2E_HTTP_PASSWORD || '',
    };
  }
  return opts;
}

const REPORT_PATH = path.join(REPORT_DIR, `crawl-${Date.now()}.md`);
const reportSink = fs.createWriteStream(REPORT_PATH, { flags: 'a' });
reportSink.write(`# Site smoke crawl\n\nbase: ${process.env.E2E_BASE_URL || 'http://localhost:8081'}\nstarted: ${new Date().toISOString()}\n\n`);

// retries=0 чтобы не множить отчёты при flakiness и не упираться в rate-limit /login
test.describe.configure({ retries: 0 });

test.describe('Site smoke crawler', () => {
  // Логинимся один раз через storageState чтобы переиспользовать на всех routes.
  // Не используем worker fixture - serial выполнение всех routes в одном browser context.
  let authState;

  test.beforeAll(async ({ browser }) => {
    const context = await browser.newContext(siteContextOptions());
    const page = await context.newPage();
    await loginAsSuperAdminUI(page);
    authState = await context.storageState();
    await context.close();
  });

  for (const route of ROUTES) {
    test(`route ${route.path} (${route.name})`, async ({ browser }) => {
      test.setTimeout(120_000);

      const context = await browser.newContext(siteContextOptions({ storageState: authState }));
      const page = await context.newPage();
      const collectors = setupErrorCollectors(page);

      const results = {
        totalFound: 0,
        clicked: 0,
        skipped: 0,
        failures: [],
      };

      try {
        await page.goto(route.path, { waitUntil: 'domcontentloaded', timeout: 30_000 });
        await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {});
        // дать SPA время отрисоваться
        await page.waitForTimeout(500);

        const clickables = getClickables(page);
        const count = await clickables.count();
        results.totalFound = count;
        const clicksToTry = Math.min(count, MAX_CLICKS_PER_PAGE);

        for (let i = 0; i < clicksToTry; i++) {
          // Каждый раз заново получаем locator - DOM мог измениться после прошлого клика
          const locator = getClickables(page).nth(i);
          if (!(await locator.isVisible({ timeout: 200 }).catch(() => false))) continue;

          const res = await clickSafe(page, locator);
          if (res.skipped) {
            results.skipped++;
            continue;
          }
          if (!res.clicked) {
            results.failures.push({ text: '<click-failed>', reason: res.reason });
            continue;
          }
          results.clicked++;

          // если URL изменился - возвращаемся на route
          if (res.urlChanged && !page.url().endsWith(route.path)) {
            await page.goto(route.path, { waitUntil: 'domcontentloaded', timeout: 15_000 }).catch(() => {});
            await page.waitForTimeout(300);
          }
          // закрываем любую открывшуюся модалку/dropdown
          await closeOpenedOverlays(page);
        }
      } finally {
        const errors = {
          pageErrors: collectors.getPageErrors(),
          consoleErrors: collectors.getConsoleErrors(),
          consoleWarnings: collectors.getConsoleWarnings(),
        };
        reportSink.write(formatRouteReport({ route, results, errors }));
        await context.close();

        // soft asserts через expect - можно увидеть несколько проблем сразу
        expect.soft(errors.pageErrors, `pageerror на ${route.path}:\n${JSON.stringify(errors.pageErrors, null, 2)}`).toHaveLength(0);
        expect.soft(errors.consoleErrors, `console.error на ${route.path}:\n${JSON.stringify(errors.consoleErrors.slice(0, 5), null, 2)}`).toHaveLength(0);
      }
    });
  }

  test.afterAll(() => {
    reportSink.write(`\nfinished: ${new Date().toISOString()}\n`);
    reportSink.end();
    // eslint-disable-next-line no-console
    console.log(`\n[site-smoke-crawler] report: ${REPORT_PATH}`);
  });
});
