const { test, expect } = require('@playwright/test');
const { LoginPage } = require('../pages/LoginPage');
const {
  loginAsSuperAdmin,
  banUser,
  unbanUser,
} = require('../helpers/permissions');

const BAN_TARGET_USERNAME = process.env.E2E_BAN_TARGET_USERNAME;

/**
 * Ban-flow тест требует выделенного тестового аккаунта (login + ID),
 * которого можно безопасно банить-разбанивать. Аккаунт создаётся вручную
 * на staging один раз (e.g. e2e_ban_target), credentials передаются через
 * E2E_BAN_TARGET_USERNAME + E2E_BAN_TARGET_ID + E2E_BAN_TARGET_PASSWORD.
 *
 * Если переменные не заданы, тест skip - чтобы не банить реальные аккаунты.
 */
test.describe('Admin / Ban flow', () => {
  test.skip(
    !BAN_TARGET_USERNAME,
    'E2E_BAN_TARGET_USERNAME не задан - тест пропущен (см. комментарий в spec)',
  );

  test('забаненный юзер не может залогиниться, разбан восстанавливает доступ', async ({
    page,
    request,
  }) => {
    const targetId = Number(process.env.E2E_BAN_TARGET_ID);
    const targetPassword = process.env.E2E_BAN_TARGET_PASSWORD;
    expect(targetId).toBeGreaterThan(0);
    expect(targetPassword).toBeTruthy();
    expect(targetId).not.toBe(1);

    const token = await loginAsSuperAdmin(request);

    await banUser(request, token, targetId);

    try {
      const loginPage = new LoginPage(page);
      await loginPage.goto();
      await loginPage.login(BAN_TARGET_USERNAME, targetPassword);

      // Забаненный видит плашку блокировки поверх своего read-only личного
      // кабинета (роутер уводит в /personal-cabinet, BanOverlay блокирует действия).
      await expect(page.locator('body')).toContainText(/заблокирован|ban/i, { timeout: 5000 });
      await expect(page).toHaveURL(/personal-cabinet/);
    } finally {
      await unbanUser(request, token, targetId).catch(() => {});
    }

    const loginPage2 = new LoginPage(page);
    await loginPage2.goto();
    await loginPage2.login(BAN_TARGET_USERNAME, targetPassword);
    await expect(page).toHaveURL(/personal-cabinet|news/);
  });

  test('superadmin неуязвим: попытка ban id=1 отвергается бэком', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    await expect(banUser(request, token, 1)).rejects.toThrow();
  });
});
