const { test, expect } = require('@playwright/test');
const { LoginPage } = require('../pages/LoginPage');
const { AdminAccessDenialsPage } = require('../pages/AdminAccessDenialsPage');
const { SUPER_ADMIN } = require('../helpers/permissions');

test.describe('Admin / Access Denials Log', () => {
  test('страница журнала отказов открывается и рендерит контейнер', async ({ page }) => {
    await new LoginPage(page).goto();
    await new LoginPage(page).login(SUPER_ADMIN.username, SUPER_ADMIN.password);

    const denialsPage = new AdminAccessDenialsPage(page);
    await denialsPage.goto();

    await expect(denialsPage.title).toBeVisible();
    const rowCount = await denialsPage.rows.count();
    expect(rowCount).toBeGreaterThanOrEqual(0);
  });
});
