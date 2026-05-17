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

    // Найти группу и нажать "Редактировать" - откроется PermissionTreeModal
    const card = groupsPage.card(name);
    await expect(card).toBeVisible();
    await card.getByRole('button', { name: 'Редактировать' }).click();
    await groupsPage.treeModal.waitFor({ state: 'visible' });

    // Поиск + выбор page.cars
    await groupsPage.treeSearch.fill('cars');
    await page.waitForTimeout(300);
    // раскрыть категорию "Страницы 0/1" и выбрать page.cars
    const pagesSection = page.getByRole('button', { name: /Страницы 0\/1 Выбрать все/ });
    if (await pagesSection.isVisible({ timeout: 1000 }).catch(() => false)) {
      await pagesSection.click();
    }
    // Tree-item имеет structure: <label class="tree-item"><input type="checkbox"><span>page.cars</span></label>
    // Кликаем по label который содержит текст page.cars - сработает на checkbox.
    const pageCarsLabel = page.locator('label.tree-item', { hasText: 'page.cars' }).first();
    await pageCarsLabel.click();

    await groupsPage.treeSave.click();
    await groupsPage.treeModal.waitFor({ state: 'hidden' });

    // Проверить через API что keys сохранились
    const groups = await listGroups(request, token);
    const updated = groups.find(g => g.id === createdGroupId);
    expect(updated.keys).toContain('page.cars');
  });
});
