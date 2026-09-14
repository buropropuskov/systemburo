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

/*
 * Известные исключения на телефоне: размеры контролов там менять не велено, они
 * согласованы с владельцем. Кнопка «Журнал» в «Моих сотрудниках» - 25px, и такой она
 * была до эпика #2473; прежний способ навигации (клик по меню на широком экране)
 * просто не заставал её в замере. Список точечный: новый мелкий контрол гейт поймает.
 */
const KNOWN_SMALL = /\blog-button\b/;

// Экран: как на него попасть + селектор карточки, внутри которой ищем наложения.
const SCREENS = [
  { name: 'Мои сотрудники', path: '/employeesview', card: '.employee-row' },
  { name: 'Мои автомобили', path: '/carsview', card: '.car-row' },
  { name: 'Доступные мне', path: '/accessible-attachments', card: '[data-testid="aa-card"]' },
  { name: 'Аналитика', path: '/analytics', card: '.metric' },
];

test.use({ viewport: MOBILE, isMobile: true, hasTouch: true });

test.describe('Мобильные инварианты', () => {
  for (const screen of SCREENS) {
    test(`${screen.name}: страница прокручивается пальцем и ничем не перекрыта`, async ({ page, context }) => {
      const cdp = await context.newCDPSession(page);

      // Переходим по адресу, а не кликом по меню. Раньше для клика окно временно
      // расширяли до 1280, где был рельс с пунктами; теперь на тач-устройстве меню
      // всегда drawer - независимо от ширины, потому что планшет в альбомной
      // ориентации тоже без мыши (#2473). Проверяем сам экран, а не навигацию.
      await loginAsSuperAdminUI(page);
      await page.goto(screen.path);
      await page.waitForTimeout(3000);

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
      const small = (await page.evaluate(smallTargets, TOUCH_MIN))
        .filter((f) => !KNOWN_SMALL.test(f.cls || ''));
      expect(small, `тач-таргеты мельче ${TOUCH_MIN}: ${JSON.stringify(small)}`).toEqual([]);
    });
  }
});
