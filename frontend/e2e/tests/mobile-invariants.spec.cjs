const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const {
  touchDrag, scrollSnapshot, horizontalOverflow, overlaps, smallTargets, OWN_SCROLL_ATTR,
} = require('../helpers/mobileInvariants');

/**
 * Мобильные инварианты: пять правил на КАЖДЫЙ экран.
 *
 * Зачем отдельный спек, когда юнит-тестов уже почти пять тысяч: они не считают
 * раскладку (jsdom), поэтому молчали и про кнопки, налезающие друг на друга, и про
 * прокрутку, которой не было. Владелец находил это пальцем за минуту.
 *
 * Новый экран добавляется строкой в SCREENS и сразу получает все проверки — правило
 * перестаёт зависеть от того, вспомнил ли автор правки про соседний список.
 */

const MOBILE = { width: 390, height: 844 };
const TOUCH_MIN = 36;

// Экран: как на него попасть + селектор карточки, внутри которой ищем наложения.
const SCREENS = [
  { name: 'Мои сотрудники', nav: 'nav-link-employees', card: '.employee-row' },
  { name: 'Мои автомобили', nav: 'nav-link-cars', card: '.car-row' },
  { name: 'Доступные мне', nav: 'nav-link-accessible-attachments', card: '[data-testid="aa-card"]' },
  { name: 'Аналитика', nav: 'nav-link-analytics', card: '.metric' },
];

test.use({ viewport: MOBILE, isMobile: true, hasTouch: true });

test.describe('Мобильные инварианты', () => {
  for (const screen of SCREENS) {
    test(`${screen.name}: страница прокручивается пальцем и ничем не перекрыта`, async ({ page, context }) => {
      const cdp = await context.newCDPSession(page);

      // Вход и переход - на широком экране: пункты меню на мобилке в drawer, а нам
      // нужен сам экран, а не проверка навигации.
      await page.setViewportSize({ width: 1280, height: 800 });
      await loginAsSuperAdminUI(page);
      await page.getByTestId(screen.nav).click({ force: true });
      await page.waitForTimeout(3000);
      await page.setViewportSize(MOBILE);
      await page.waitForTimeout(1500);

      // 1-2. Палец двигает страницу, а не внутренний блок. Проверяем фактом, а не
      // объявлениями: непереполненная область с `overflow: auto` жест не забирает.
      const scrollable = await page.evaluate(() => {
        const de = document.documentElement;
        return de.scrollHeight - de.clientHeight > 40;
      });
      if (scrollable) {
        const before = await page.evaluate(scrollSnapshot, OWN_SCROLL_ATTR);
        await touchDrag(cdp);
        const after = await page.evaluate(scrollSnapshot, OWN_SCROLL_ATTR);

        expect(after.page, 'страница не сдвинулась от тач-драга').toBeGreaterThan(before.page);

        const stolen = after.inner
          .map((el, i) => ({ ...el, was: before.inner[i] ? before.inner[i].top : 0 }))
          .filter((el) => el.top > el.was + 1);
        expect(stolen, `жест забрал внутренний блок: ${JSON.stringify(stolen)}`).toEqual([]);

        await page.evaluate(() => window.scrollTo(0, 0));
        await page.waitForTimeout(300);
      }

      // 3. Ничего не торчит за правый край.
      const over = await page.evaluate(horizontalOverflow);
      expect(over, `узлы за правым краем: ${JSON.stringify(over)}`).toEqual([]);
      const docWidth = await page.evaluate(() => ({
        doc: document.documentElement.scrollWidth, vw: window.innerWidth,
      }));
      expect(docWidth.doc).toBeLessThanOrEqual(docWidth.vw);

      // 4. Внутри карточек ничего не наложено друг на друга.
      const crossing = await page.evaluate(overlaps, screen.card);
      expect(crossing, `элементы карточки перекрывают друг друга: ${JSON.stringify(crossing)}`).toEqual([]);

      // 5. По кнопкам можно попасть пальцем.
      const small = await page.evaluate(smallTargets, TOUCH_MIN);
      expect(small, `тач-таргеты мельче ${TOUCH_MIN}: ${JSON.stringify(small)}`).toEqual([]);
    });
  }
});
