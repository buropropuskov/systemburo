const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { AdminUsersPage } = require('../pages/AdminUsersPage');
const { UserPermissionsModal } = require('../pages/UserPermissionsModal');

// UserPermissionsModal открывается из AdminUsers через кнопку "Роль и группы прав".
// Smoke: открыть, проверить элементы (role select, group checkboxes), закрыть.
test.describe('AdminUsers - UserPermissionsModal', () => {
  test('клик "Роль и группы прав" открывает модалку с role select', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    const usersPage = new AdminUsersPage(page);
    await usersPage.goto();

    // Выбираем любого юзера - e2e_user или первый seed-юзер
    await usersPage.selectUserByLogin('e2e_user');

    // Открыть модалку прав
    await usersPage.permissionsRoleGroupsButton.click();

    const modal = new UserPermissionsModal(page);
    await modal.waitForOpen();

    // role-select должен быть виден (combobox с ролями)
    await expect(modal.roleSelect).toBeVisible();

    // Закрыть через Отмена
    await modal.cancelButton.click();
    await modal.waitForClose();
  });
});
