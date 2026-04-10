const { test, expect } = require('@playwright/test');
const { registerUser, setAuthTokens } = require('../helpers/auth');
const { LoginPage } = require('../pages/LoginPage');
const { NavigationBar } = require('../pages/NavigationBar');

test.describe('Authentication', () => {
  test('login page loads with login form', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();

    await expect(loginPage.form).toBeVisible();
    await expect(loginPage.usernameInput).toBeVisible();
    await expect(loginPage.passwordInput).toBeVisible();
    await expect(loginPage.submitButton).toBeVisible();
  });

  test('successful login redirects to personal cabinet', async ({ page }) => {
    const username = `e2e_login_ok_${Date.now()}`;
    await registerUser(username, 'testpass123', 1);

    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(username, 'testpass123');

    await page.waitForURL('/personal-cabinet');
    await expect(page).toHaveURL(/personal-cabinet/);
  });

  test('invalid credentials show error message', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login('nonexistent_user', 'wrongpassword');

    await expect(loginPage.errorMessage).toBeVisible();
  });

  test('empty fields show validation error', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.submitButton.click();

    // Submit with empty fields triggers FormField per-field validation (role="alert")
    const fieldError = page.getByRole('alert').first();
    await expect(fieldError).toBeVisible();
  });

  test('logout redirects to login page', async ({ page }) => {
    const username = `e2e_logout_${Date.now()}`;
    await registerUser(username, 'testpass123', 1);

    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(username, 'testpass123');
    await page.waitForURL('/personal-cabinet');

    const navBar = new NavigationBar(page);
    await navBar.logout();

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
