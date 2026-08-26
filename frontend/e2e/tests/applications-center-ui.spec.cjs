const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');

test.describe('ApplicationsCenter UI', () => {
  test('страница /center открывается и показывает заголовок + поиск', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/center');
    await expect(page).toHaveURL(/\/center/);

    await expect(page.getByRole('heading', { name: 'Центр заявок' })).toBeVisible();
    await expect(page.getByTestId('center-input-search')).toBeVisible();
  });

  test('поиск в /center реактивирует фильтр (smoke)', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/center');

    const search = page.getByTestId('center-input-search');
    await search.fill('non-existent-application-xyz');
    await page.waitForTimeout(500);

    // Empty state или 0 rows
    const emptyText = page.getByText(/Заявок нет|Не найдено/);
    const isEmptyVisible = await emptyText.isVisible({ timeout: 2000 }).catch(() => false);
    if (!isEmptyVisible) {
      // если есть rows - они должны быть пустые
      const rows = page.locator('table tbody tr');
      const count = await rows.count();
      expect(count).toBeLessThanOrEqual(1); // header или 1 информационная row
    }
  });
});

test.describe('ApplicationsCenter - мобильный ряд шапки', () => {
  // Замок против переполнения ряда (#1832): кнопка выгрузки приезжала с подписью и
  // растровой иконкой в натуральные 30px, ряд разъезжался на 428px при ширине 370,
  // «Фильтр» уходил за край, а страница получала горизонтальную прокрутку. Юнит этого
  // не видит - в jsdom нет раскладки, поэтому проверка живёт здесь.
  test('на 390px контролы шапки влезают в ряд и страница не едет по горизонтали', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await loginAsSuperAdminUI(page);
    await page.goto('/center');
    await expect(page.getByTestId('center-button-filter')).toBeVisible();

    const row = page.locator('.header-row2');
    const overflow = await row.evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(overflow, 'ряд шапки не должен переполняться').toBeLessThanOrEqual(0);

    const pageOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(pageOverflow, 'страница не должна прокручиваться по горизонтали').toBeLessThanOrEqual(0);

    // Все контролы ряда одной высоты: дропдаун архива, «Обновить», выгрузка, «Фильтр».
    const heights = await row.evaluate((el) => {
      const kids = [...el.querySelectorAll('.base-dropdown__button, .refresh-btn, .filter-btn')];
      return kids.map((k) => Math.round(k.getBoundingClientRect().height));
    });
    expect(heights.length).toBeGreaterThanOrEqual(3);
    expect(new Set(heights).size, `высоты разъехались: ${heights.join(', ')}`).toBe(1);

    // Подпись выгрузки на мобилке скрыта визуально, но остаётся для чтения с экрана.
    const exportBtn = page.getByTestId('center-button-export');
    if (await exportBtn.count()) {
      const textBox = await exportBtn.locator('.export-btn__text').boundingBox();
      expect(textBox.width, 'подпись должна быть свёрнута в иконку').toBeLessThan(5);
      await expect(exportBtn).toHaveAttribute('aria-label', /Выгрузить реестр|Готовим/);
    }
  });
});
