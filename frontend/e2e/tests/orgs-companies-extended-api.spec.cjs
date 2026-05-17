const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

// Smoke на extended GET endpoints organizations/companies.
// Эти ручки возвращают данные с join-ами (users, tables, unload-places).
// Покрываем чтобы убедиться что они работают и возвращают массивы/объекты.

test.describe('Organizations / Companies - extended GET API', () => {
  test('GET /organizations/with-users возвращает массив', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/organizations/with-users`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
  });

  test('GET /organizations/with-users-extended возвращает массив', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/organizations/with-users-extended`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
  });

  test('GET /companies/with-users возвращает массив', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/companies/with-users`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
  });

  test('GET /companies/with-users-extended возвращает массив', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/companies/with-users-extended`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
  });

  test('GET /get-organization возвращает текущую организацию (или 404 если её нет)', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/get-organization`, { headers: headers(token) });
    // У superadmin может не быть привязки - 404/200 оба допустимы.
    expect([200, 404]).toContain(res.status());
  });

  test('GET /organizations/:id/users работает для существующей организации', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const listRes = await request.get(`${API_BASE}/organizations`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    if (!list.length) {
      test.skip(true, 'no organizations to query');
      return;
    }
    const id = list[0].id;
    const res = await request.get(`${API_BASE}/organizations/${id}/users`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(Array.isArray(body.data || [])).toBeTruthy();
  });

  test('GET /organizations/:id/tables работает', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const listRes = await request.get(`${API_BASE}/organizations`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    if (!list.length) {
      test.skip(true, 'no organizations to query');
      return;
    }
    const id = list[0].id;
    const res = await request.get(`${API_BASE}/organizations/${id}/tables`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('GET /organizations/:id/unload-places работает', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const listRes = await request.get(`${API_BASE}/organizations`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    if (!list.length) {
      test.skip(true, 'no organizations to query');
      return;
    }
    const id = list[0].id;
    const res = await request.get(`${API_BASE}/organizations/${id}/unload-places`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('GET /companies/:id/users работает', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const listRes = await request.get(`${API_BASE}/companies`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    if (!list.length) {
      test.skip(true, 'no companies to query');
      return;
    }
    const id = list[0].id;
    const res = await request.get(`${API_BASE}/companies/${id}/users`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('GET /companies/:id/tables работает', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const listRes = await request.get(`${API_BASE}/companies`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    if (!list.length) {
      test.skip(true, 'no companies to query');
      return;
    }
    const id = list[0].id;
    const res = await request.get(`${API_BASE}/companies/${id}/tables`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });

  test('GET /unload-places возвращает массив мест разгрузки', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/unload-places`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
  });

  test('GET /unload-places/:id/time-slots работает для существующего места', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const listRes = await request.get(`${API_BASE}/unload-places`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    if (!list.length) {
      test.skip(true, 'no unload-places to query');
      return;
    }
    const id = list[0].id;
    const res = await request.get(`${API_BASE}/unload-places/${id}/time-slots`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
  });
});
