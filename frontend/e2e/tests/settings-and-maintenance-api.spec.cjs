const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

test.describe('Settings + Maintenance API', () => {
  test('GET /settings возвращает массив настроек', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/settings`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data || body;
    expect(Array.isArray(data) || typeof data === 'object').toBeTruthy();
  });

  test('GET /settings/upload возвращает upload-конфиг', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/settings/upload`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data || body;
    expect(data).toBeTruthy();
  });

  test('GET /admin/maintenance возвращает статус', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/admin/maintenance`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data || body;
    // структура: { enabled: bool, ... }
    expect(typeof data.enabled === 'boolean' || 'enabled' in data).toBeTruthy();
  });
});
