const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

// CreateFeedbackRequest требует message >= 10 символов.
// Smoke-цикл: создать обращение → видеть его в /feedback/my и в админском /feedback/all.
test.describe('Feedback - create + list', () => {
  test('POST /feedback создаёт обращение, GET /feedback/my возвращает его', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const message = `E2E feedback test ${Date.now()} — проверка smoke-флоу`;

    const createRes = await request.post(`${API_BASE}/feedback`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: { message },
    });
    expect(createRes.ok() || createRes.status() === 201).toBeTruthy();

    const myRes = await request.get(`${API_BASE}/feedback/my`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(myRes.ok()).toBeTruthy();
    const body = await myRes.json();
    const items = body.data || body;
    const found = (Array.isArray(items) ? items : []).find(f => f.message === message);
    expect(found).toBeTruthy();
  });

  test('POST /feedback с коротким message возвращает 400 (валидация min:10)', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const res = await request.post(`${API_BASE}/feedback`, {
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      data: { message: 'short' },
    });
    expect(res.status()).toBe(400);
  });
});
