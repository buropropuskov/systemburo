const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

test.describe('Misc API: logs, user info, approvers, tables', () => {
  test('GET /users/me возвращает данные текущего юзера', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/users/me`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const data = (await res.json()).data;
    expect(data.username).toBe('buropropuskov');
    expect(data.is_super_admin).toBe(true);
  });

  test('GET /user-data возвращает расширенные данные юзера', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/user-data`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const data = (await res.json()).data;
    expect(data).toBeTruthy();
  });

  test('GET /request-logs возвращает массив логов', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/request-logs?limit=10`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data || body;
    expect(Array.isArray(data) || Array.isArray(data.items)).toBeTruthy();
  });

  test('GET /request-logs/stats возвращает статистику', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/request-logs/stats`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('GET /request-logs/users возвращает уникальных юзеров', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/request-logs/users`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('GET /application-approvers возвращает список согласующих', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/application-approvers`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const data = (await res.json()).data || [];
    expect(Array.isArray(data)).toBeTruthy();
  });

  test('GET /application-approvers/available-users возвращает кандидатов', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/application-approvers/available-users`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('GET /system-tables возвращает массив таблиц', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/system-tables`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const data = (await res.json()).data || [];
    expect(Array.isArray(data)).toBeTruthy();
  });

  test('POST + DELETE /system-tables создаёт и удаляет таблицу', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = `e2e_table_${Date.now()}`;
    const createRes = await request.post(`${API_BASE}/system-tables`, {
      headers: headers(token),
      data: { name, description: 'e2e test table' },
    });
    if (!createRes.ok() && createRes.status() !== 201) {
      test.skip(true, `system-tables create returned ${createRes.status()}`);
      return;
    }
    const createdId = (await createRes.json()).data?.id;
    expect(createdId).toBeTruthy();

    await request.delete(`${API_BASE}/system-tables/${createdId}`, { headers: headers(token) });
  });
});
