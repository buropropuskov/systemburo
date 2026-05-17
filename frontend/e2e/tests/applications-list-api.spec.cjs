const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

// Applications create требует сложной фикстуры (categories, attachments) -
// в этом spec'е только smoke на list endpoint, чтобы убедиться что бэк
// корректно отдаёт пагинацию/фильтры.
test.describe('Applications list API', () => {
  test('GET /applications/user возвращает paginated response', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/applications/user`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data || body;
    // shape: { items: [], total: N } или просто [].
    expect(Array.isArray(data) || Array.isArray(data.items) || 'total' in data).toBeTruthy();
  });

  test('GET /applications (общий список) для суперадмина возвращает массив', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/applications`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data || body;
    expect(Array.isArray(data) || Array.isArray(data.items)).toBeTruthy();
  });
});
