const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

test.describe('AccessDenials - manage actions API', () => {
  test('GET /access-denials/archive возвращает архив', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/access-denials/archive`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data || body;
    expect(Array.isArray(data) || Array.isArray(data.items) || 'total' in data).toBeTruthy();
  });

  test('POST /access-denials/archive с пустым фильтром не падает', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    // ArchiveOlderThan endpoint - принимает cutoff в payload или query
    const res = await request.post(`${API_BASE}/access-denials/archive`, {
      headers: headers(token),
      data: { cutoff: new Date('2020-01-01').toISOString() },
    });
    // OK или 400 при невалидном payload - всё кроме 5xx приемлемо
    expect(res.status()).toBeLessThan(500);
  });

  test('DELETE /access-denials с фильтром по несуществующему user_id', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.delete(`${API_BASE}/access-denials?user_id=999999`, { headers: headers(token) });
    // Может вернуть 200 (0 удалено) либо 204
    expect(res.status()).toBeLessThan(500);
  });
});
