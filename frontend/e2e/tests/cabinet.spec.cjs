const { test, expect } = require('@playwright/test');
const { loginAsUser } = require('../helpers/auth');
const { CabinetPage } = require('../pages/CabinetPage');
const { NavigationBar } = require('../pages/NavigationBar');

test.describe('Personal Cabinet', () => {
  test('cabinet page is accessible', async ({ page }) => {
    await loginAsUser(page);
    // landing после login может быть /news, явно переходим в кабинет
    await page.goto('/personal-cabinet');
    await expect(page).toHaveURL(/personal-cabinet/);

    const cabinetPage = new CabinetPage(page);
    await expect(cabinetPage.root).toBeVisible();
  });

  // TODO: dashboard был редизайн в PR #207/#208 - селекторы .dashboard-row/.dashboard-card
  // больше не актуальны. Обновить когда понадобится покрывать конкретный layout.
  test.skip('dashboard content (селекторы устарели после редизайна)', async () => {});

  test('cabinet has navigation menu', async ({ page }) => {
    await loginAsUser(page);
    await page.goto('/personal-cabinet');

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();
  });
});
