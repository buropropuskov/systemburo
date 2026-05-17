const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { AdminPermissionGroupsPage } = require('../pages/AdminPermissionGroupsPage');
const {
  loginAsSuperAdmin,
  createGroup,
  listGroups,
  deleteGroup,
  cleanupE2eEntities,
  e2eName,
} = require('../helpers/permissions');

test.describe('Admin / Permission Groups CRUD', () => {
  test.afterAll(async ({ request }) => {
    await cleanupE2eEntities(request).catch(() => {});
  });

  test('создаёт группу прав через API, видит её в UI списка групп', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = e2eName('group');
    const created = await createGroup(request, token, {
      name,
      description: 'e2e groups-crud',
      keys: [],
    });
    expect(created).toMatchObject({ name });

    await loginAsSuperAdminUI(page);

    const groupsPage = new AdminPermissionGroupsPage(page);
    await groupsPage.goto();

    await expect(groupsPage.cards).not.toHaveCount(0);
    await expect(groupsPage.card(name)).toBeVisible();
  });

  test('создаёт группу через UI и API подтверждает её существование', async ({ page, request }) => {
    await loginAsSuperAdminUI(page);

    const groupsPage = new AdminPermissionGroupsPage(page);
    await groupsPage.goto();

    const name = e2eName('uigroup');
    await groupsPage.createGroup({ name, description: 'через UI' });

    await expect(groupsPage.card(name)).toBeVisible();

    const token = await loginAsSuperAdmin(request);
    const groups = await listGroups(request, token);
    const found = groups.find(g => g.name === name);
    expect(found).toBeTruthy();

    if (found) await deleteGroup(request, token, found.id).catch(() => {});
  });

  test('обычный юзер БЕЗ permission.audit.manage не может создать группу (403)', async ({ request }) => {
    // Регресс-тест на критическую уязвимость, найденную smoke-проверкой.
    // До фикса POST /api/permission-groups возвращал 201 для любого
    // авторизованного юзера, что позволяло манипулировать системой прав.
    const apiBase = process.env.E2E_API_BASE_URL || '/api';
    const loginRes = await request.post(`${apiBase}/login`, {
      data: { username: 'e2e_user', password: 'testpass123' },
    });
    expect(loginRes.ok()).toBeTruthy();
    const userToken = (await loginRes.json()).data.token;

    const res = await request.post(`${apiBase}/permission-groups`, {
      headers: { Authorization: `Bearer ${userToken}`, 'Content-Type': 'application/json' },
      data: { name: 'HACK Group', description: '', keys: [] },
    });
    expect(res.status()).toBe(403);
  });
});
