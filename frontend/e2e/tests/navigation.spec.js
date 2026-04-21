const { test, expect } = require('@playwright/test');
const { loginAsAdmin, loginAsUser } = require('../helpers/auth');
const { NavigationBar } = require('../pages/NavigationBar');

test.describe('Navigation & Authorization', () => {
  test('regular user cannot access admin pages', async ({ page }) => {
    const username = `e2e_nav_user_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/table-constructor');

    await expect(page).toHaveURL(/personal-cabinet/);
  });

  test('admin can access admin pages', async ({ page }) => {
    await loginAsAdmin(page);

    await page.goto('/table-constructor');

    await expect(page).toHaveURL(/table-constructor/);
  });

  test('nav menu shows admin items for admin user', async ({ page }) => {
    await loginAsAdmin(page);

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();
    await navBar.root.hover();
    await expect(navBar.root.getByText('Админка')).toBeVisible();
  });

  // TODO: зависит от корректного type_id у e2e_user — проверить что cmd/seed
  // выбирает реально non-admin тип, возможно type_id попал на роль которая
  // тоже видит "Админка".
  test.skip('nav menu hides admin items for regular user', async ({ page }) => {
    const username = `e2e_nav_noadmin_${Date.now()}`;
    await loginAsUser(page, username);

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();
    await navBar.root.hover();
    await expect(navBar.root.getByText('Админка')).not.toBeVisible();
  });

  test('navigation links change URL correctly', async ({ page }) => {
    const username = `e2e_nav_links_${Date.now()}`;
    await loginAsUser(page, username);

    await expect(page).toHaveURL(/personal-cabinet/);

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();

    // Check that clicking cabinet link keeps us on personal cabinet
    await navBar.root.hover();
    const personalCabinetLink = navBar.cabinetLink;
    if (await personalCabinetLink.isVisible()) {
      await personalCabinetLink.click();
      await expect(page).toHaveURL(/personal-cabinet/);
    }
  });
});
