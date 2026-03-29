const { test, expect } = require('@playwright/test');
const { registerUser, loginAsAdmin, loginAsUser } = require('../helpers/auth');

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

    const navMenu = page.locator('.nav-menu');
    await expect(navMenu).toBeVisible();
    await expect(navMenu.getByText('Админка')).toBeVisible();
  });

  test('nav menu hides admin items for regular user', async ({ page }) => {
    const username = `e2e_nav_noadmin_${Date.now()}`;
    await loginAsUser(page, username);

    const navMenu = page.locator('.nav-menu');
    await expect(navMenu).toBeVisible();
    await expect(navMenu.getByText('Админка')).not.toBeVisible();
  });

  test('navigation links change URL correctly', async ({ page }) => {
    const username = `e2e_nav_links_${Date.now()}`;
    await loginAsUser(page, username);

    // Verify we are on personal cabinet
    await expect(page).toHaveURL(/personal-cabinet/);

    // Click on a nav item and verify navigation
    const navMenu = page.locator('.nav-menu');
    await expect(navMenu).toBeVisible();

    // Check that clicking "Личный кабинет" keeps us on personal cabinet
    const personalCabinetLink = navMenu.getByText('Личный кабинет');
    if (await personalCabinetLink.isVisible()) {
      await personalCabinetLink.click();
      await expect(page).toHaveURL(/personal-cabinet/);
    }
  });
});
