const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { AdminPermissionGroupsPage } = require('../pages/AdminPermissionGroupsPage');
const {
  loginAsSuperAdmin,
  createGroup,
  listGroups,
  cleanupE2eEntities,
  e2eName,
} = require('../helpers/permissions');

test.describe('AdminPermissionGroups - UI delete + tree open', () => {
  test.afterAll(async ({ request }) => {
    await cleanupE2eEntities(request).catch(() => {});
  });

  test('Клик Редактировать открывает редактор прав группы с включённым правом', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = e2eName('ui_tree');
    await createGroup(request, token, { name, keys: ['page.cars'] });

    await loginAsSuperAdminUI(page);
    const groupsPage = new AdminPermissionGroupsPage(page);
    await groupsPage.goto();

    await groupsPage.clickEditTree(name);
    await expect(groupsPage.treeModal).toBeVisible();
    // page.cars в группе -> его тумблер включён (aria-pressed=true).
    const carsToggle = groupsPage.treeKey('page.cars');
    await expect(carsToggle).toBeVisible();
    await expect(carsToggle).toHaveAttribute('aria-pressed', 'true');

    await groupsPage.treeCancel.click();
    await expect(groupsPage.treeModal).toBeHidden();
  });

  test('Delete группы через UI убирает карточку и удаляет в API', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = e2eName('ui_del_grp');
    const created = await createGroup(request, token, { name, keys: [] });

    await loginAsSuperAdminUI(page);
    const groupsPage = new AdminPermissionGroupsPage(page);
    await groupsPage.goto();

    await groupsPage.clickDelete(name);
    // Удаление подтверждаем в ConfirmationModal (эталонная модалка, не нативный confirm).
    await page.getByTestId('confirmation-confirm').click();

    await expect(groupsPage.card(name)).toHaveCount(0, { timeout: 5000 });

    const all = await listGroups(request, token);
    expect(all.find(g => g.id === created.id)).toBeFalsy();
  });
});
