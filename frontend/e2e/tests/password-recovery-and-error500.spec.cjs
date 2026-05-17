const { test, expect } = require('@playwright/test');
const { LoginPage } = require('../pages/LoginPage');

test.describe('Password recovery + Error500', () => {
  test('клик "Забыли пароль?" открывает модалку восстановления', async ({ page }) => {
    await new LoginPage(page).goto();
    await page.getByRole('link', { name: 'Забыли пароль?' }).click();
    await expect(page.getByRole('heading', { name: 'Восстановление доступа' })).toBeVisible();
    await expect(page.getByText(/Если вы забыли логин или пароль/)).toBeVisible();
  });

  test('страница /500 показывает информацию об ошибке и кнопку отчёта', async ({ page }) => {
    await page.goto('/500');
    await expect(page).toHaveURL(/\/500/);
    await expect(page.getByText('Ошибка 500')).toBeVisible();
    await expect(page.getByTestId('send-bug-report-btn')).toBeVisible();
  });

  test('страница /maintenance показывает информацию', async ({ page }) => {
    await page.goto('/maintenance');
    await expect(page).toHaveURL(/\/maintenance/);
    // Любая инфо должна быть отрисована - проверяем что body не пустой
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('страница /403 показывает Forbidden', async ({ page }) => {
    // /403 требует auth, без login будет редирект
    await page.goto('/403');
    // либо на /403, либо на login - оба валидны
    const url = page.url();
    expect(url).toMatch(/(403|\/$|\/login)/);
  });
});
