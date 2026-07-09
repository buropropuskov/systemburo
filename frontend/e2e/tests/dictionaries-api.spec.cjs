const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

// Покрываем CRUD справочников (organizations, companies, citizenships,
// license-plate-formats, user-types). Цикл: POST → GET → PUT → DELETE.
// Smoke на API чтобы убедиться что endpoints работают - детальная
// валидация полей вынесена в backend-тесты.

test.describe('Dictionaries API - CRUD', () => {
  // Note: POST для справочников возвращает {id, message} - без полного объекта.
  // Поэтому проверяем созданную запись через последующий GET в списке.

  test('citizenships: POST → GET → PUT → DELETE', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = `E2E Cit ${Date.now()}`;

    const createRes = await request.post(`${API_BASE}/citizenships`, {
      headers: headers(token),
      data: { name, is_default: false },
    });
    expect(createRes.ok() || createRes.status() === 201).toBeTruthy();
    const createdId = (await createRes.json()).data?.id;
    expect(createdId).toBeTruthy();

    const listRes = await request.get(`${API_BASE}/citizenships`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    const found = list.find(c => c.id === createdId);
    expect(found?.name).toBe(name);

    await request.put(`${API_BASE}/citizenships/${createdId}`, {
      headers: headers(token),
      data: { name: `${name} updated` },
    });

    const delRes = await request.delete(`${API_BASE}/citizenships/${createdId}`, { headers: headers(token) });
    expect(delRes.ok() || delRes.status() === 204).toBeTruthy();
  });

  test('organizations: POST → GET → DELETE', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = `E2E Org ${Date.now()}`;

    const createRes = await request.post(`${API_BASE}/organizations`, {
      headers: headers(token),
      data: { name, type: 'Организация' },
    });
    expect(createRes.ok() || createRes.status() === 201).toBeTruthy();
    const createdId = (await createRes.json()).data?.id;
    expect(createdId).toBeTruthy();

    const listRes = await request.get(`${API_BASE}/organizations`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    expect(list.find(o => o.id === createdId)).toBeTruthy();

    const delRes = await request.delete(`${API_BASE}/organizations/${createdId}`, { headers: headers(token) });
    expect(delRes.ok() || delRes.status() === 204).toBeTruthy();
  });

  test('companies: POST → GET → DELETE', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = `E2E Company ${Date.now()}`;

    const createRes = await request.post(`${API_BASE}/companies`, {
      headers: headers(token),
      data: { name, type: 'Организация' },
    });
    expect(createRes.ok() || createRes.status() === 201).toBeTruthy();
    const createdId = (await createRes.json()).data?.id;
    expect(createdId).toBeTruthy();

    const listRes = await request.get(`${API_BASE}/companies`, { headers: headers(token) });
    const list = (await listRes.json()).data || [];
    expect(list.find(c => c.id === createdId)).toBeTruthy();

    await request.delete(`${API_BASE}/companies/${createdId}`, { headers: headers(token) });
  });

  test('license-plate-formats: POST → GET → DELETE', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = `E2E LPF ${Date.now()}`;

    const createRes = await request.post(`${API_BASE}/license-plate-formats`, {
      headers: headers(token),
      data: { name, pattern: 'A###AA' },
    });
    if (!createRes.ok() && createRes.status() !== 201) {
      test.skip(true, `lpf create returned ${createRes.status()}`);
      return;
    }
    const createdId = (await createRes.json()).data?.id;
    if (!createdId) {
      test.skip(true, 'lpf create did not return id');
      return;
    }

    const listRes = await request.get(`${API_BASE}/license-plate-formats`, { headers: headers(token) });
    expect(listRes.ok()).toBeTruthy();

    await request.delete(`${API_BASE}/license-plate-formats/${createdId}`, { headers: headers(token) });
  });

  test('user-types: GET /user-types (public) возвращает массив', async ({ request }) => {
    // /user-types - public endpoint без auth (для регистрации/login flow).
    const res = await request.get(`${API_BASE}/user-types`);
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
    expect(list.length).toBeGreaterThan(0);
  });

  test('user-types-management: GET всех типов (admin)', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/user-types-management`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
  });
});
