const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

test.describe('News API - CRUD', () => {
  const createdIds = [];

  test.afterAll(async ({ request }) => {
    const token = await loginAsSuperAdmin(request).catch(() => null);
    if (!token) return;
    for (const id of createdIds) {
      await request.delete(`${API_BASE}/news/${id}`, { headers: headers(token) }).catch(() => {});
    }
  });

  test('POST /news создаёт новость, GET /news/all возвращает её', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const title = `E2E News ${Date.now()}`;
    const res = await request.post(`${API_BASE}/news`, {
      headers: headers(token),
      data: { title, description: 'e2e desc', is_active: true },
    });
    expect(res.ok() || res.status() === 201).toBeTruthy();
    const created = (await res.json()).data;
    createdIds.push(created.id);
    expect(created.title).toBe(title);

    const listRes = await request.get(`${API_BASE}/news/all`, { headers: headers(token) });
    expect(listRes.ok()).toBeTruthy();
    const all = (await listRes.json()).data || [];
    expect(all.find(n => n.id === created.id)).toBeTruthy();
  });

  test('PUT /news/:id обновляет title', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const createRes = await request.post(`${API_BASE}/news`, {
      headers: headers(token),
      data: { title: 'Initial', description: 'd', is_active: true },
    });
    const created = (await createRes.json()).data;
    createdIds.push(created.id);

    const newTitle = `Updated ${Date.now()}`;
    const upRes = await request.put(`${API_BASE}/news/${created.id}`, {
      headers: headers(token),
      data: { title: newTitle },
    });
    expect(upRes.ok()).toBeTruthy();

    const listRes = await request.get(`${API_BASE}/news/all`, { headers: headers(token) });
    const updated = ((await listRes.json()).data || []).find(n => n.id === created.id);
    expect(updated.title).toBe(newTitle);
  });

  test('GET /news (active) возвращает массив активных', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.get(`${API_BASE}/news`, { headers: headers(token) });
    expect(res.ok()).toBeTruthy();
    const list = (await res.json()).data || [];
    expect(Array.isArray(list)).toBeTruthy();
    // активные = is_active true (если есть)
    for (const n of list) {
      expect(n.is_active === undefined || n.is_active === true).toBeTruthy();
    }
  });

  test('DELETE /news/:id убирает новость', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const createRes = await request.post(`${API_BASE}/news`, {
      headers: headers(token),
      data: { title: 'To delete', is_active: true },
    });
    const created = (await createRes.json()).data;

    const delRes = await request.delete(`${API_BASE}/news/${created.id}`, { headers: headers(token) });
    expect(delRes.ok() || delRes.status() === 204).toBeTruthy();

    const listRes = await request.get(`${API_BASE}/news/all`, { headers: headers(token) });
    expect(((await listRes.json()).data || []).find(n => n.id === created.id)).toBeFalsy();
  });
});
