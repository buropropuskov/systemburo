const { test, expect } = require('@playwright/test');
const { registerUser, loginAsUser, setAuthTokens } = require('../helpers/auth');

test.describe('Authentication', () => {
  test('login page loads with login form', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.login__form')).toBeVisible();
    await expect(page.locator('.login__form input').first()).toBeVisible();
    await expect(page.locator('.login__form input').nth(1)).toBeVisible();
    await expect(page.locator('.login__button')).toBeVisible();
  });

  test('successful login redirects to personal cabinet', async ({ page }) => {
    const username = `e2e_login_ok_${Date.now()}`;
    await registerUser(username, 'testpass123', 1);

    await page.goto('/');
    await page.locator('.login__form input').first().fill(username);
    await page.locator('.login__form input').nth(1).fill('testpass123');
    await page.locator('.login__button').click();

    await page.waitForURL('/personal-cabinet');
    await expect(page).toHaveURL(/personal-cabinet/);
  });

  test('invalid credentials show error message', async ({ page }) => {
    await page.goto('/');
    await page.locator('.login__form input').first().fill('nonexistent_user');
    await page.locator('.login__form input').nth(1).fill('wrongpassword');
    await page.locator('.login__button').click();

    await expect(page.locator('.error-message')).toBeVisible();
  });

  test('empty fields show validation error', async ({ page }) => {
    await page.goto('/');
    await page.locator('.login__button').click();

    await expect(page.locator('.error-message')).toContainText('Необходимо заполнить все поля');
  });

  test('logout redirects to login page', async ({ page }) => {
    const username = `e2e_logout_${Date.now()}`;
    await registerUser(username, 'testpass123', 1);

    await page.goto('/');
    await page.locator('.login__form input').first().fill(username);
    await page.locator('.login__form input').nth(1).fill('testpass123');
    await page.locator('.login__button').click();
    await page.waitForURL('/personal-cabinet');

    // Find and click the logout button in the nav menu
    await page.locator('.nav-menu').getByText('Выход').click();

    await page.waitForURL('/');
    await expect(page).toHaveURL(/^https?:\/\/[^/]+\/$/);
  });

  test('unauthenticated user is redirected from protected routes', async ({ page }) => {
    await page.goto('/personal-cabinet');

    await expect(page).toHaveURL(/^https?:\/\/[^/]+\/$/);
  });

  test('authenticated user is redirected from login to personal cabinet', async ({ page }) => {
    const username = `e2e_redirect_${Date.now()}`;
    await registerUser(username, 'testpass123', 1);
    await setAuthTokens(page, username, 'testpass123');

    await page.goto('/');
    await page.waitForURL('/personal-cabinet');
    await expect(page).toHaveURL(/personal-cabinet/);
  });
});
