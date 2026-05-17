const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { NotificationsDropdown } = require('../pages/NotificationsDropdown');

test.describe('Notifications dropdown', () => {
  test('иконка уведомлений видна в шапке', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    await expect(dropdown.bell).toBeVisible();
  });

  test('клик по иконке открывает панель уведомлений', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    await dropdown.open();

    await expect(dropdown.title).toHaveText(/Уведомления/);
    // panel должна показать либо empty state, либо items - оба валидны
    const itemsCount = await dropdown.items.count();
    if (itemsCount === 0) {
      await expect(dropdown.emptyText).toBeVisible();
    } else {
      expect(itemsCount).toBeGreaterThan(0);
    }
  });

  test('повторный клик закрывает панель', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    await dropdown.open();
    await expect(dropdown.panel).toBeVisible();

    await dropdown.close();
    await expect(dropdown.panel).toBeHidden();
  });
});
