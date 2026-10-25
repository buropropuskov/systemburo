const { test, expect } = require('@playwright/test');
const { tabletFixtures, CLOCK } = require('../helpers/tablet-fixtures.cjs');

for (const width of [390, 768, 1440]) {
  test(`notification details keep the list underneath at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    await tabletFixtures(page);
    await page.route(url => url.pathname.startsWith('/api/notifications'), route => route.fulfill({ json: {
      success: true,
      data: [{ id: 99001, title: 'Учебное уведомление', message: 'Учебный текст для проверки вложенного окна.', created_at: CLOCK, is_read: true, category: 'system' }],
      meta: { total: 1, unread_count: 0 },
    } }));
    await page.goto('/news');
    await page.getByRole('button', { name: 'Уведомления', exact: true }).click();
    const panel = page.getByTestId('ob-notifications-panel');
    await expect(panel).toBeVisible();
    await expect(panel).toHaveCSS('opacity', '1');
    await page.locator('.notification-item').first().click();
    const detail = page.getByTestId('notif-detail-dialog');
    await expect(detail).toBeVisible();
    await expect(detail).toHaveCSS('opacity', '1');
    await expect(panel).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(detail).toBeHidden();
    await expect(panel).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(panel).toBeHidden();
  });
}
