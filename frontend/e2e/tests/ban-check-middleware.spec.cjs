const { test, expect } = require('@playwright/test');
const {
  loginAsSuperAdmin,
  banUser,
  unbanUser,
} = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

const BAN_TARGET_USERNAME = process.env.E2E_BAN_TARGET_USERNAME;

/**
 * Регресс на issue #271: глобальный ban-check middleware должен немедленно
 * возвращать 403 для забаненного пользователя даже с валидным access-токеном.
 *
 * До фикса JWTAuth просто декодировал токен и пускал юзера на любую protected
 * ручку до exp - бан не работал для API-only клиентов.
 *
 * Требует E2E_BAN_TARGET_* как и permissions-ban-flow, без них тест skip.
 */
test.describe('Ban check middleware', () => {
  test.skip(
    !BAN_TARGET_USERNAME,
    'E2E_BAN_TARGET_USERNAME не задан - тест пропущен',
  );

  test('забаненный юзер с валидным access-токеном получает 403 на protected ручке', async ({
    request,
  }) => {
    const targetId = Number(process.env.E2E_BAN_TARGET_ID);
    const targetPassword = process.env.E2E_BAN_TARGET_PASSWORD;
    expect(targetId).toBeGreaterThan(0);
    expect(targetPassword).toBeTruthy();
    expect(targetId).not.toBe(1);

    // 1. Login юзером, получаем access-токен.
    const loginRes = await request.post(`${API_BASE}/login`, {
      data: { username: BAN_TARGET_USERNAME, password: targetPassword },
    });
    expect(loginRes.ok()).toBeTruthy();
    const userToken = (await loginRes.json()).data.token;
    expect(userToken).toBeTruthy();

    // 2. До бана - access работает.
    const beforeBan = await request.get(`${API_BASE}/users/me`, {
      headers: { Authorization: `Bearer ${userToken}` },
    });
    expect(beforeBan.status()).toBe(200);

    // 3. Баним.
    const adminToken = await loginAsSuperAdmin(request);
    await banUser(request, adminToken, targetId);

    try {
      // 4. С тем же access (без refresh) - 403. Окно кэша 30s, поэтому
      // в worst-case первый запрос ещё пройдёт. Делаем несколько попыток
      // с интервалом чтобы дождаться обновления кэша.
      const start = Date.now();
      let lastStatus = 200;
      while (Date.now() - start < 35_000) {
        const res = await request.get(`${API_BASE}/users/me`, {
          headers: { Authorization: `Bearer ${userToken}` },
        });
        lastStatus = res.status();
        if (lastStatus === 403) break;
        await new Promise(r => setTimeout(r, 2000));
      }
      expect(lastStatus).toBe(403);
    } finally {
      // 5. Разбан - очищаем кэш, восстанавливаем доступ.
      await unbanUser(request, adminToken, targetId).catch(() => {});
    }
  });
});
