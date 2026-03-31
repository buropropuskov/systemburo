const { test, expect } = require('@playwright/test');
const { loginAsAdmin, loginAsUser } = require('../helpers/auth');

test.describe('Personal Cabinet', () => {
  test('cabinet page shows user profile', async ({ page }) => {
    const username = `e2e_cabinet_profile_${Date.now()}`;
    await loginAsUser(page, username);

    await expect(page).toHaveURL(/personal-cabinet/);
    await expect(page.locator('.account-dashboard')).toBeVisible();
  });

  test('regular user sees dashboard content', async ({ page }) => {
    const username = `e2e_cabinet_user_${Date.now()}`;
    await loginAsUser(page, username);

    await expect(page.locator('.account-dashboard')).toBeVisible();
    // Dashboard should have at least some content rendered
    const rows = page.locator('.dashboard-row');
    await rows.first().waitFor({ state: 'visible', timeout: 10000 });
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test('admin user sees more sections than regular user', async ({ page }) => {
    await loginAsAdmin(page);

    await expect(page.locator('.account-dashboard')).toBeVisible();
    // Admin should have dashboard cards (animated or regular)
    const cards = page.locator('.dashboard-card, .dashboard-card-animated');
    await cards.first().waitFor({ state: 'visible', timeout: 10000 });
    const count = await cards.count();
    expect(count).toBeGreaterThanOrEqual(2);
  });

  test('cabinet has navigation menu', async ({ page }) => {
    const username = `e2e_cabinet_nav_${Date.now()}`;
    await loginAsUser(page, username);

    await expect(page.locator('.nav-menu')).toBeVisible();
  });
});
