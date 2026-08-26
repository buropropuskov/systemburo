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

test.describe('AdminRoles - UI edit/delete карточек', () => {
  test.afterAll(async ({ request }) => {
    await cleanupE2eEntities(request).catch(() => {});
  });

  test('Edit роль через UI: name + description сохраняются в API', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const code = e2eName('ui_edit');
    const created = await createRole(request, token, {
      name: 'Initial UI Name',
      code,
      description: 'initial',
    });

    await loginAsSuperAdminUI(page);
    const rolesPage = new AdminRolesPage(page);
    await rolesPage.goto();

    // В master-detail имя/описание правятся inline в панели деталей.
    await rolesPage.editMeta(code, { name: 'Updated UI Name', description: 'updated desc' });

    // Проверяем через API
    const all = await listRoles(request, token);
    const updated = all.find(r => r.id === created.id);
    expect(updated.name).toBe('Updated UI Name');
    expect(updated.description).toBe('updated desc');

    await deleteRole(request, token, created.id).catch(() => {});
  });

  test('Delete роль через UI убирает карточку и удаляет в API', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const code = e2eName('ui_del');
    const created = await createRole(request, token, { name: 'To delete via UI', code });

    await loginAsSuperAdminUI(page);
    const rolesPage = new AdminRolesPage(page);
    await rolesPage.goto();

    await rolesPage.clickDelete(code);
    // Удаление подтверждаем в ConfirmationModal (эталонная модалка, не нативный confirm).
    await page.getByTestId('confirmation-confirm').click();

    // Строка пропала
    await expect(rolesPage.card(code)).toHaveCount(0, { timeout: 5000 });

    // API подтверждает
    const all = await listRoles(request, token);
    expect(all.find(r => r.id === created.id)).toBeFalsy();
  });
});
