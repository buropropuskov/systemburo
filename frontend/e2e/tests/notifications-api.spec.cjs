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
      data: { user_id: myId, title, message: 'test message', type: 'info' },
    });
    expect(createRes.ok() || createRes.status() === 201).toBeTruthy();
    const created = (await createRes.json()).data;
    expect(created.title).toBe(title);

    // GET /notifications возвращает мои уведомления
    const listRes = await request.get(`${API_BASE}/notifications`, { headers: headers(token) });
    expect(listRes.ok()).toBeTruthy();
    const body = await listRes.json();
    const list = body.data || body;
    const items = Array.isArray(list) ? list : (list.items || []);
    const found = items.find(n => n.id === created.id);
    expect(found).toBeTruthy();

    // mark-read
    const readRes = await request.put(`${API_BASE}/notifications/${created.id}/read`, {
      headers: headers(token),
      data: { is_read: true },
    });
    expect(readRes.ok()).toBeTruthy();

    // delete
    const delRes = await request.delete(`${API_BASE}/notifications/${created.id}`, { headers: headers(token) });
    expect(delRes.ok() || delRes.status() === 204).toBeTruthy();
  });
});
