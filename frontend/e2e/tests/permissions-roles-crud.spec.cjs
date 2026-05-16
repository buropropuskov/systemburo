const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { AdminRolesPage } = require('../pages/AdminRolesPage');
const {
  loginAsSuperAdmin,
  createRole,
  listRoles,
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

    await expect(rolesPage.cards).not.toHaveCount(0);
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
});
