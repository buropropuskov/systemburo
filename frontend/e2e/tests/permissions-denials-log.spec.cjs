const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { AdminAccessDenialsPage } = require('../pages/AdminAccessDenialsPage');

test.describe('Admin / Access Denials Log', () => {
  test('страница журнала отказов открывается и рендерит контейнер', async ({ page }) => {
    await loginAsSuperAdminUI(page);

    const denialsPage = new AdminAccessDenialsPage(page);
    await denialsPage.goto();

    await expect(denialsPage.title).toBeVisible();
    const rowCount = await denialsPage.rows.count();
    expect(rowCount).toBeGreaterThanOrEqual(0);
  });
});
