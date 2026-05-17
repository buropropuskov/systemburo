const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

test.describe('Announcements API', () => {
  const createdIds = [];

  test.afterAll(async ({ request }) => {
    const token = await loginAsSuperAdmin(request).catch(() => null);
    if (!token) return;
    for (const id of createdIds) {
      await request.delete(`${API_BASE}/announcements/${id}`, { headers: headers(token) }).catch(() => {});
    }
  });

  test('POST /announcements + GET /announcements/all + set-active', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const title = `E2E Announcement ${Date.now()}`;
    const res = await request.post(`${API_BASE}/announcements`, {
      headers: headers(token),
      data: { title, description: 'тестовое объявление', is_important: false },
    });
    expect(res.ok() || res.status() === 201).toBeTruthy();
    const created = (await res.json()).data;
    createdIds.push(created.id);
    expect(created.title).toBe(title);

    const allRes = await request.get(`${API_BASE}/announcements/all`, { headers: headers(token) });
    const all = (await allRes.json()).data || [];
    expect(all.find(a => a.id === created.id)).toBeTruthy();

    // set-active: делаем объявление активным
    const setActiveRes = await request.post(`${API_BASE}/announcements/set-active`, {
      headers: headers(token),
      data: { announcement_id: created.id },
    });
    expect(setActiveRes.ok()).toBeTruthy();

    // GET /active возвращает наше объявление
    const activeRes = await request.get(`${API_BASE}/announcements/active`, { headers: headers(token) });
    const active = (await activeRes.json()).data;
    expect(active?.id).toBe(created.id);
  });

  test('PUT /announcements/:id и DELETE', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const createRes = await request.post(`${API_BASE}/announcements`, {
      headers: headers(token),
      data: { title: `E2E Upd ${Date.now()}`, description: 'orig' },
    });
    const created = (await createRes.json()).data;

    const newTitle = `Updated ${Date.now()}`;
    const upRes = await request.put(`${API_BASE}/announcements/${created.id}`, {
      headers: headers(token),
      data: { title: newTitle },
    });
    expect(upRes.ok()).toBeTruthy();

    const delRes = await request.delete(`${API_BASE}/announcements/${created.id}`, { headers: headers(token) });
    expect(delRes.ok() || delRes.status() === 204).toBeTruthy();
  });
});
