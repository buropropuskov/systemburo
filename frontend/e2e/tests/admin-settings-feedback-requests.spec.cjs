const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');

test.describe('Admin / Settings, Feedback, Requests-log - smoke', () => {
  test('страница /admin/settings открывается у супер-админа', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/admin/settings');
    await expect(page).toHaveURL(/admin\/settings/);
    await expect(page.getByRole('heading', { name: 'Настройки системы' })).toBeVisible();
  });

  test('страница /admin/feedback открывается у супер-админа', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/admin/feedback');
    await expect(page).toHaveURL(/admin\/feedback/);
    await expect(page.getByRole('heading', { name: 'Обратная связь' })).toBeVisible();
  });

  test('страница /admin/requests (логи) открывается у супер-админа', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/admin/requests');
    await expect(page).toHaveURL(/admin\/requests/);
    await expect(page.getByRole('heading', { name: 'Мониторинг запросов' })).toBeVisible();
  });
});
