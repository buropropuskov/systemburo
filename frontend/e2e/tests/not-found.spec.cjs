const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');

test.describe('Страница 404', () => {
  test('неизвестный адрес показывает 404 с промахнувшимся путём', async ({ page }) => {
    await page.goto('/kakoy-to-put');
    await expect(page.getByTestId('not-found-page')).toBeVisible();
    await expect(page.getByTestId('not-found-page')).toContainText('Страница не найдена');
    await expect(page.getByTestId('not-found-page')).toContainText('/kakoy-to-put');
  });

  test('прямой /404 открывается без пути в карточке', async ({ page }) => {
    await page.goto('/404');
    await expect(page.getByTestId('not-found-page')).toBeVisible();
    await expect(page.locator('.not-found__path')).toBeHidden();
  });

  test('гостя кнопка "На главную" ведёт на вход', async ({ page }) => {
    await page.goto('/net-takoy-stranicy');
    await page.getByTestId('not-found-home').click();
    await expect(page).toHaveURL(/\/$/);
  });

  test('авторизованного кнопка "На главную" ведёт в новости', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/net-takoy-stranicy');
    await page.getByTestId('not-found-home').click();
    await expect(page).toHaveURL(/\/news/);
  });
});
