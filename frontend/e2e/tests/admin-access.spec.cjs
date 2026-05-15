const { test, expect } = require('@playwright/test');
const { registerUser, loginAsAdmin, loginAsUser } = require('../helpers/auth');

test.describe('Admin-only Access Control', () => {
  test('non-admin redirected from /admin/users', async ({ page }) => {
    const username = `e2e_noadmin_users_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/admin/users');
    await expect(page).not.toHaveURL(/admin\/users/);
  });

  test('non-admin redirected from /admin/settings', async ({ page }) => {
    const username = `e2e_noadmin_settings_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/admin/settings');
    await expect(page).not.toHaveURL(/admin\/settings/);
  });

  test('admin can access /admin/users', async ({ page }) => {
    await loginAsAdmin(page);

    await page.goto('/admin/users');
    await expect(page).toHaveURL(/admin\/users/);
  });
});
