const { test, expect } = require('@playwright/test');
const { LoginPage } = require('../pages/LoginPage');
const { loginAsSuperAdminUI } = require('../helpers/auth');

/**
 * Доводит юзера до /500 так же, как это происходит в проде: api-клиент ловит
 * 5xx и сам уводит на страницу инцидента. Ломаем загрузку новостей - fulfill
 * тут не мок данных (их правило проекта требует брать из HAR), а имитация
 * сбоя, как route.abort() для сети.
 */
async function triggerServerError(page) {
  await loginAsSuperAdminUI(page);
  await page.route('**/api/news**', (route) => route.fulfill({ status: 500, body: '' }));
  await page.reload();
  await expect(page).toHaveURL(/\/500/);
  await page.unroute('**/api/news**');
}

const bugContext = (page) => page.evaluate(() => sessionStorage.getItem('last_bug_ctx'));

test.describe('Password recovery + страницы-ошибки', () => {
  test('клик "Забыли пароль?" открывает модалку восстановления', async ({ page }) => {
    await new LoginPage(page).goto();
    await page.getByRole('link', { name: 'Забыли пароль?' }).click();
    await expect(page.getByRole('heading', { name: 'Восстановление доступа' })).toBeVisible();
    await expect(page.getByText(/Если вы забыли логин или пароль/)).toBeVisible();
  });

  test('ошибка сервера уводит на /500 и запоминает контекст инцидента', async ({ page }) => {
    await triggerServerError(page);
    expect(await bugContext(page)).toContain('/news');
    expect(await bugContext(page)).toContain('500');
  });

  test('уход со страницы закрывает инцидент - вернуться на /500 нельзя', async ({ page }) => {
    await triggerServerError(page);

    await page.goBack();
    await expect(page).not.toHaveURL(/\/500/);
    expect(await bugContext(page)).toBeNull();

    await page.goto('/500');
    await expect(page).not.toHaveURL(/\/500/);
  });

  test('кнопка "На главную" уводит с /500 и закрывает инцидент', async ({ page }) => {
    await triggerServerError(page);

    // Упали как раз на новостях - повторять там нечего, кнопка была бы дублем.
    await expect(page.getByTestId('error-500-retry')).toBeHidden();
    await page.getByTestId('error-500-home').click();
    await expect(page).toHaveURL(/\/news/);
    expect(await bugContext(page)).toBeNull();
  });

  test('повтор возвращает на страницу, где упал запрос', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.route('**/api/unique-cars**', (route) => route.fulfill({ status: 500, body: '' }));
    await page.goto('/carsview');
    await expect(page).toHaveURL(/\/500/);
    await page.unroute('**/api/unique-cars**');

    await page.getByTestId('error-500-retry').click();
    await expect(page).toHaveURL(/\/carsview/);
  });

  test('прямой заход на /500 без ошибки не показывает страницу инцидента', async ({ page }) => {
    await page.goto('/500');
    await expect(page).not.toHaveURL(/\/500/);
    await expect(page.getByTestId('error-500-page')).toBeHidden();
  });

  test('прямой заход на /maintenance без техработ не показывает заглушку', async ({ page }) => {
    await page.goto('/maintenance');
    await expect(page).not.toHaveURL(/\/maintenance/);
  });

  test('прямой заход на /403 без отказа в правах не показывает Forbidden', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/403');
    await expect(page).not.toHaveURL(/\/403/);
  });
});
