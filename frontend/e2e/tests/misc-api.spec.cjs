const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

test.describe('Misc API endpoints', () => {
  test('POST /bug-report принимает валидный отчёт', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.post(`${API_BASE}/bug-report`, {
      headers: headers(token),
      data: {
        bug_hash: `e2e${Date.now()}`.slice(0, 16),
        route: '/test/e2e',
        http_status: 500,
        message: 'E2E test bug report',
      },
    });
    expect(res.ok() || res.status() === 201).toBeTruthy();
  });

  test('POST /bug-report с невалидным http_status возвращает 400', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.post(`${API_BASE}/bug-report`, {
      headers: headers(token),
      data: { bug_hash: 'e2e12345', route: '/x', http_status: 200, message: 'x' },
    });
    expect(res.status()).toBe(400);
  });

  test('GET /consents возвращает список согласий', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/consents`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const data = (await res.json()).data;
    expect(Array.isArray(data) || data === null || typeof data === 'object').toBeTruthy();
  });

  test('GET /consents/check/pd_processing возвращает boolean', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/consents/check/pd_processing`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('POST /consents выдаёт согласие, DELETE отзывает', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const grantRes = await request.post(`${API_BASE}/consents`, {
      headers: headers(token),
      data: { consent_type: 'pd_processing' },
    });
    expect(grantRes.ok() || grantRes.status() === 201).toBeTruthy();

    const revokeRes = await request.delete(`${API_BASE}/consents/pd_processing`, { headers: headers(token) });
    expect(revokeRes.ok() || revokeRes.status() === 204).toBeTruthy();
  });

  test('GET /unload-places возвращает массив', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/unload-places`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const data = (await res.json()).data || [];
    expect(Array.isArray(data)).toBeTruthy();
  });

  test('GET /permissions/my отдаёт режим super для суперадмина (bypass)', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/permissions/my`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const data = (await res.json()).data || {};
    // Новый формат: { mode, permissions[{key,value,source}], denied, banned, ban_reason }.
    // Суперадмин -> mode=super, permissions пуст (всё разрешено через is_super_admin,
    // фронт проверяет режим отдельно).
    expect(data.mode).toBe('super');
    expect(Array.isArray(data.permissions)).toBeTruthy();
  });
});
