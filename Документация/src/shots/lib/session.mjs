/**
 * Съёмочная сессия: браузер, вход под учётной записью роли и приведение
 * страницы к постоянному виду.
 */

import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

/*
 * Playwright и браузеры к нему уже установлены для сквозных тестов в frontend.
 * Ставить их второй раз ради снимков незачем, но и лежит снимальщик не там,
 * поэтому пакет резолвится от frontend/package.json явно - обычный импорт
 * искал бы node_modules по дереву от Документация/ и не нашёл бы.
 */
const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(HERE, '..', '..', '..', '..');
const requireFromFrontend = createRequire(path.join(REPO_ROOT, 'frontend', 'package.json'));
const { chromium } = requireFromFrontend('playwright');

/**
 * Окно съёмки. Ширина подобрана так, чтобы раскладка была десктопной, а кадр
 * целого экрана после расширения на воздух оставался в пределах допустимой
 * пропорции.
 */
export const VIEWPORT = { width: 1300, height: 850 };

/** Снимаем в двойном масштабе и уменьшаем - см. capture.normalize. */
export const SCALE = 2;

/** Все туры системы. Съёмочным учётным записям они помечаются пройденными. */
const TOUR_KEYS = ['user', 'guard', 'approve', 'accept', 'admin'];

/**
 * Версия, которой помечается прохождение. Заведомо выше любой реальной: тур
 * версионируется, и точное значение протухло бы при первом же обновлении тура.
 */
const TOUR_DONE_VERSION = 1000;

/**
 * Гасит анимации и переходы. Без этого кадр ловит середину анимации открытия
 * модалки, и одна и та же съёмка даёт разный результат.
 */
const STILL_CSS = `
  *, *::before, *::after {
    animation-duration: 0s !important;
    animation-delay: 0s !important;
    transition-duration: 0s !important;
    transition-delay: 0s !important;
    caret-color: transparent !important;
  }
  html { scroll-behavior: auto !important; }
`;

/** @returns {Promise<import('playwright').Browser>} */
export async function openBrowser() {
  return chromium.launch();
}

/**
 * Создаёт отдельное окружение браузера.
 *
 * Каждой роли - своё: сеанс продлевается по признаку в хранилище браузера, и в
 * общем окружении вход второй учётной записи проходил бы поверх сеанса первой -
 * страница входа даже не показывалась бы.
 *
 * @param {import('playwright').Browser} browser
 * @returns {Promise<import('playwright').BrowserContext>}
 */
export async function newContext(browser) {
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: SCALE,
    locale: 'ru-RU',
    timezoneId: 'Europe/Moscow',
    reducedMotion: 'reduce',
  });

  // Светлая тема применяется скриптом из index.html до первого кадра, поэтому
  // ставим её в хранилище до загрузки страницы, а не после.
  await context.addInitScript(() => {
    try {
      localStorage.setItem('app-theme', 'light');
    } catch {
      /* приватный режим - тема и так светлая по умолчанию */
    }
  });

  await context.addInitScript((css) => {
    const apply = () => {
      const style = document.createElement('style');
      style.textContent = css;
      document.head.appendChild(style);
    };
    if (document.head) apply();
    else document.addEventListener('DOMContentLoaded', apply, { once: true });
  }, STILL_CSS);

  return context;
}

/**
 * Помечает все туры пройденными для учётной записи. Делается запросом к
 * методу программного интерфейса, а не правкой базы: автозапуск тура перекрыл
 * бы съёмку оверлеем, а сбрасывать его в базе - обходить ту самую логику,
 * которую документируем.
 */
async function markToursDone(apiBase, token) {
  for (const tour of TOUR_KEYS) {
    const response = await fetch(`${apiBase}/onboarding/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ tour, version: TOUR_DONE_VERSION, finished: true }),
    });
    if (!response.ok) {
      throw new Error(`не удалось отметить тур ${tour}: ${response.status}`);
    }
  }
}

/**
 * Подтверждает согласие на обработку персональных данных.
 *
 * Запрос согласия на стенде включён намеренно - окно надо снять для
 * руководства, - но всем прочим кадрам оно мешает: пока согласие не дано,
 * оверлей перекрывает страницу. Кадр самого окна снимается учётной записью,
 * которая через это ещё не проходила.
 */
async function acceptConsent(apiBase, token) {
  const gate = await fetch(`${apiBase}/consents/gate`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!gate.ok) {
    // Молча выйти нельзя: не спросив состояние, мы не знаем, требуется ли
    // согласие, и сбой всплыл бы позже невнятной ошибкой в чужом кадре.
    throw new Error(`не удалось узнать состояние согласия: ${gate.status}`);
  }
  const body = await gate.json();
  if (!(body.data ?? body)?.required) return;

  const response = await fetch(`${apiBase}/consents/accept`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({}),
  });
  if (!response.ok) {
    throw new Error(`не удалось подтвердить согласие: ${response.status}`);
  }
}

/** Вход по программному интерфейсу - только чтобы получить маркер доступа. */
export async function apiLogin(apiBase, username, password) {
  const response = await fetch(`${apiBase}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!response.ok) {
    throw new Error(`вход ${username} не удался: ${response.status}`);
  }
  const body = await response.json();
  return body.data?.token ?? body.token;
}

/**
 * Открывает страницу под учётной записью роли: помечает туры пройденными,
 * входит через форму (маркер доступа живёт в памяти приложения, поэтому вход
 * запросом браузеру не поможет) и фиксирует часы.
 *
 * @param {import('playwright').BrowserContext} context
 * @param {{username: string, password: string}} account
 * @param {{baseUrl: string, apiBase: string, clockAt: Date|null}} options
 */
export async function signIn(context, account, { baseUrl, apiBase, clockAt, keepConsent }) {
  const token = await apiLogin(apiBase, account.username, account.password);
  /*
   * Пока согласие не дано, шлюз отвечает 403 на все защищённые методы, включая
   * отметку туров. Учётной записи для кадра окна согласия туры и не нужны: тур
   * не запускается, пока согласие не подтверждено.
   */
  if (!keepConsent) {
    await acceptConsent(apiBase, token);
    await markToursDone(apiBase, token);
  }

  const page = await context.newPage();

  /*
   * Часы фиксируются подменой Date, но не остановкой таймеров: остановленные
   * таймеры вешают опрос уведомлений и подгрузку списков, и страница просто не
   * догружается. Момент берётся заведомо в прошлом относительно выдачи маркера
   * доступа, иначе приложение сочтёт его просроченным и выкинет на вход.
   */
  if (clockAt) {
    await page.clock.setFixedTime(clockAt);
  }

  await page.goto(`${baseUrl}/`);
  await page.getByTestId('login-input-username').fill(account.username);
  await page.getByTestId('login-input-password').fill(account.password);
  await page.getByTestId('login-button-submit').click();
  await page.waitForURL(/\/(news|personal-cabinet)(\?|$|\/)/, { timeout: 20000 });

  return page;
}

/**
 * Убирает с экрана то, что налезает на кадр и живёт своей жизнью: всплывающие
 * уведомления и раскрытые выпадающие списки.
 */
export async function calmPage(page) {
  await page.evaluate(() => {
    document
      .querySelectorAll('.toast, .notification-toast, [data-testid="deletion-toast"]')
      .forEach((node) => node.remove());
  });
  await page.keyboard.press('Escape');
  await settleAnimations(page);
}

/**
 * Доигрывает анимации и переходы до конца.
 *
 * Именно доигрывает, а не выключает: списки и карточки появляются анимацией с
 * начальным `opacity: 0`, и снятая анимация оставляла их в этом начальном
 * состоянии - кадры выходили пустыми, хотя данные загружены (счётчик внизу
 * списка показывал их число). Снимок сам дожимает анимации, поэтому до обводки
 * страницу надо привести в то же конечное состояние, в каком её увидит камера.
 */
export async function settleAnimations(page) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const pending = await page.evaluate(() => {
      const running = document.getAnimations();
      for (const animation of running) {
        try {
          // Бесконечную не дожать - её снимок отменяет, нам довольно паузы.
          if (animation.effect?.getComputedTiming?.().iterations === Infinity) animation.pause();
          else animation.finish();
        } catch {
          /* анимация уже снята или не поддаётся - пропускаем */
        }
      }
      return running.length;
    });
    if (pending === 0) return;
    await page.waitForTimeout(120);
  }
}

