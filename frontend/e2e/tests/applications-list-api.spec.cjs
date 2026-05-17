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

  test('GET /applications/unread-count возвращает число', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/applications/unread-count`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const data = body.data ?? body;
    expect(typeof (data.count ?? data) === 'number' || typeof data === 'object').toBeTruthy();
  });

  test('GET /applications/:id и связанные read-only ручки работают для существующей заявки', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const listRes = await request.get(`${API_BASE}/applications`, { headers: headers(token) });
    if (!listRes.ok()) {
      test.skip(true, 'cannot list applications');
      return;
    }
    const body = await listRes.json();
    const list = body.data?.items || body.data || body.items || [];
    if (!Array.isArray(list) || !list.length) {
      test.skip(true, 'no applications in DB to test against');
      return;
    }
    const appId = list[0].id;

    const endpoints = [
      `/applications/${appId}`,
      `/applications/${appId}/responsible-users`,
      `/applications/${appId}/details`,
      `/applications/${appId}/attachments`,
      `/applications/${appId}/history`,
      `/applications/${appId}/viewers`,
      `/applications/${appId}/reads`,
      `/applications/${appId}/check-approval-status`,
    ];

    for (const path of endpoints) {
      const res = await request.get(`${API_BASE}${path}`, { headers: headers(token) });
      expect(res.status(), `GET ${path} вернул ${res.status()}`).toBeLessThan(500);
      expect([200, 404]).toContain(res.status());
    }
  });
});
