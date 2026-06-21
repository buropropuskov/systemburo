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

  test('Клик Редактировать открывает PermissionTreeModal с правами группы', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const name = e2eName('ui_tree');
    await createGroup(request, token, { name, keys: ['page.cars'] });

    await loginAsSuperAdminUI(page);
    const groupsPage = new AdminPermissionGroupsPage(page);
    await groupsPage.goto();

    await groupsPage.clickEditTree(name);
    await expect(groupsPage.treeModal).toBeVisible();
    // page.cars должен быть выбран (он в группе)
    const carsKey = groupsPage.treeKey('page.cars');
    await groupsPage.expandGroup('page');
    await expect(carsKey).toBeVisible();
    const checkbox = carsKey.locator('input[type="checkbox"]');
    await expect(checkbox).toBeChecked();

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
    // Нативный confirm заменён на кастомную модалку ConfirmDialog - подтверждаем в ней.
    await page.getByTestId('confirm-ok').click();

    await expect(groupsPage.card(name)).toHaveCount(0, { timeout: 5000 });

    const all = await listGroups(request, token);
    expect(all.find(g => g.id === created.id)).toBeFalsy();
  });
});
