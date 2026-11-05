const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

// Получаем user_id из JWT - нужно чтобы создать notification для самого себя
function userIdFromToken(token) {
  const payload = JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString());
  return payload.user_id;
}

test.describe('Notifications API', () => {
  test('POST /notifications + GET /notifications + mark-read + delete', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const myId = userIdFromToken(token);

    const title = `E2E Notif ${Date.now()}`;
    const createRes = await request.post(`${API_BASE}/notifications`, {
      headers: headers(token),
      data: { user_id: myId, title, message: 'test message', type: 'application_status_changed',
        data: JSON.stringify({ application_id: 701, application_number: 'Private fabricated number',
          projection_ref: { v: 1, type: 'application_status_changed', application_id: 701 } }) },
    });
    expect(createRes.ok() || createRes.status() === 201).toBeTruthy();
    const created = (await createRes.json()).data;
    expect(created.id).toBeGreaterThan(0);
    expect(created.user_id).toBe(myId);
    expect(created.title).toBe('Есть обновление');
    expect(created.message).toBe('Есть обновление');
    expect(created.data).toBeNull();
    expect(created).not.toHaveProperty('projection_ref');
    expect(created).not.toHaveProperty('group_key');

    // GET /notifications возвращает мои уведомления
    const listRes = await request.get(`${API_BASE}/notifications`, { headers: headers(token) });
    expect(listRes.ok()).toBeTruthy();
    const body = await listRes.json();
    const list = body.data || body;
    const items = Array.isArray(list) ? list : (list.items || []);
    const found = items.find(n => n.id === created.id);
    expect(found).toBeTruthy();
    expect(found.title).toBe('Есть обновление');
    expect(found.message).toBe('Есть обновление');
    expect(found.data).toBeNull();
    expect(found).not.toHaveProperty('projection_ref');
    expect(items.every(n => n.user_id === myId)).toBe(true);

    // mark-read
    const readRes = await request.put(`${API_BASE}/notifications/${created.id}/read`, {
      headers: headers(token),
      data: { is_read: true },
    });
    expect(readRes.ok()).toBeTruthy();
    const read = (await readRes.json()).data;
    expect(read.id).toBe(created.id);
    expect(read.is_read).toBe(true);
    expect(read.title).toBe('Есть обновление');
    expect(read.data).toBeNull();

    // delete
    const delRes = await request.delete(`${API_BASE}/notifications/${created.id}`, { headers: headers(token) });
    expect(delRes.ok() || delRes.status() === 204).toBeTruthy();
    const remainingRes = await request.get(`${API_BASE}/notifications`, { headers: headers(token) });
    expect(remainingRes.ok()).toBeTruthy();
    const remaining = (await remainingRes.json()).data;
    expect(remaining.some(n => n.id === created.id)).toBe(false);
  });
});
