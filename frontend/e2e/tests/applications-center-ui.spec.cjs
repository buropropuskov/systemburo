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
