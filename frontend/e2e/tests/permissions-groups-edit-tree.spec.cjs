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

test.describe('Admin / Permission Groups - edit keys via tree', () => {
  let createdGroupId = null;

  test.afterAll(async ({ request }) => {
    if (createdGroupId) {
      const token = await loginAsSuperAdmin(request).catch(() => null);
      if (token) await deleteGroup(request, token, createdGroupId).catch(() => {});
    }
    await cleanupE2eEntities(request).catch(() => {});
  });

  test('открыть редактирование группы и сохранить выбранные ключи', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = e2eName('edit_group');
    const created = await createGroup(request, token, { name, description: 'e2e edit tree', keys: [] });
    createdGroupId = created.id;
    expect(created.keys).toEqual([]);

    await loginAsSuperAdminUI(page);
    const groupsPage = new AdminPermissionGroupsPage(page);
    await groupsPage.goto();

    await expect(groupsPage.card(name)).toBeVisible();
    // В master-detail права редактируются из панели деталей: выбрать строку -> «Редактировать права».
    await groupsPage.clickEditTree(name);

    // Поиск + развернуть категорию + клик по конкретному testid
    await groupsPage.treeSearch.fill('cars');
    await page.waitForTimeout(300);
    await groupsPage.expandGroup('page');
    await groupsPage.treeKey('page.cars').click();

    await groupsPage.treeSave.click();
    await groupsPage.treeModal.waitFor({ state: 'hidden' });

    const groups = await listGroups(request, token);
    const updated = groups.find(g => g.id === createdGroupId);
    expect(updated.keys).toContain('page.cars');
  });
});
