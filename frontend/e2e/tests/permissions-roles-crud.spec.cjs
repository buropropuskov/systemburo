const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { AdminRolesPage } = require('../pages/AdminRolesPage');
const {
  loginAsSuperAdmin,
  createRole,
  listRoles,
  updateRole,
  deleteRole,
  cleanupE2eEntities,
  e2eName,
} = require('../helpers/permissions');

test.describe('Admin / Roles CRUD', () => {
  test.afterAll(async ({ request }) => {
    await cleanupE2eEntities(request).catch(() => {});
  });

  test('создаёт роль через API, видит её в UI списка ролей', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const code = e2eName('role');
    const created = await createRole(request, token, {
      name: `E2E Role ${code}`,
      code,
      description: 'e2e roles-crud smoke',
    });
    expect(created).toMatchObject({ code, name: `E2E Role ${code}` });

    await loginAsSuperAdminUI(page);

    const rolesPage = new AdminRolesPage(page);
    await rolesPage.goto();

    // Конкретная карточка с кодом должна быть видна.
    // not.toHaveCount(0) на rolesPage.cards убрали - параллельные cleanup-и
    // в других тестах могут временно опустошить список.
    await expect(rolesPage.card(code)).toBeVisible();
  });

  test('создаёт роль через UI и видит её в списке (полный круг)', async ({ page, request }) => {
    await loginAsSuperAdminUI(page);

    const rolesPage = new AdminRolesPage(page);
    await rolesPage.goto();

    const code = e2eName('uirole');
    await rolesPage.createRole({
      name: `E2E UI Role ${code}`,
      code,
      description: 'через UI',
    });

    await expect(rolesPage.card(code)).toBeVisible();

    const token = await loginAsSuperAdmin(request);
    const roles = await listRoles(request, token);
    const found = roles.find(r => r.code === code);
    expect(found).toBeTruthy();
    expect(found.name).toBe(`E2E UI Role ${code}`);

    if (found) await deleteRole(request, token, found.id).catch(() => {});
  });

  test('PUT /roles/:id обновляет name и description', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const code = e2eName('edit_role');
    const created = await createRole(request, token, {
      name: 'Initial Name',
      code,
      description: 'Initial desc',
    });

    await updateRole(request, token, created.id, {
      name: 'Updated Name',
      description: 'Updated desc',
    });

    const all = await listRoles(request, token);
    const updated = all.find(r => r.id === created.id);
    expect(updated.name).toBe('Updated Name');
    expect(updated.description).toBe('Updated desc');

    await deleteRole(request, token, created.id).catch(() => {});
  });

  test('DELETE /roles/:id убирает роль из списка', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const code = e2eName('del_role');
    const created = await createRole(request, token, { name: 'To Delete', code });

    await deleteRole(request, token, created.id);

    const all = await listRoles(request, token);
    expect(all.find(r => r.id === created.id)).toBeFalsy();
  });

  test('обычный юзер БЕЗ permission.audit.manage не может создать роль (403)', async ({ request }) => {
    // Регресс-тест на критическую уязвимость, найденную smoke-проверкой.
    // До фикса POST /api/roles возвращал 201 для любого авторизованного юзера.
    const apiBase = process.env.E2E_API_BASE_URL || '/api';
    const loginRes = await request.post(`${apiBase}/login`, {
      data: { username: 'e2e_user', password: 'testpass123' },
    });
    expect(loginRes.ok()).toBeTruthy();
    const loginBody = await loginRes.json();
    const userToken = loginBody.data.token;

    const res = await request.post(`${apiBase}/roles`, {
      headers: { Authorization: `Bearer ${userToken}`, 'Content-Type': 'application/json' },
      data: { code: 'hack_attempt', name: 'HACK', description: '' },
    });
    expect(res.status()).toBe(403);
  });
});
