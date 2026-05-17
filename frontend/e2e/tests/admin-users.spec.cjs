const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { AdminUsersPage } = require('../pages/AdminUsersPage');

// Целевой юзер для не-destructive операций - изолированный seed-аккаунт.
// На local CI seed создаёт e2e_user (type_id=1).
// На staging вручную создан testuser_f5 для smoke-проверок.
const TARGET_USER_LOGIN = process.env.E2E_TARGET_USER || 'e2e_user';

test.describe('Admin / Users', () => {
  test('страница списка пользователей открывается у супер-админа', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    const usersPage = new AdminUsersPage(page);
    await usersPage.goto();

    await expect(usersPage.title).toBeVisible();
    const count = await usersPage.rows.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test('поиск фильтрует пользователей по логину', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    const usersPage = new AdminUsersPage(page);
    await usersPage.goto();

    const totalBefore = await usersPage.rows.count();
    await usersPage.search(TARGET_USER_LOGIN);

    const filteredCount = await usersPage.rows.count();
    expect(filteredCount).toBeGreaterThanOrEqual(1);
    expect(filteredCount).toBeLessThanOrEqual(totalBefore);
    await expect(usersPage.rowByLogin(TARGET_USER_LOGIN)).toBeVisible();
  });

  test('бессмысленный поиск показывает empty state', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    const usersPage = new AdminUsersPage(page);
    await usersPage.goto();

    await usersPage.search('___no_such_user___xyz');
    await expect(usersPage.emptyMessage).toBeVisible({ timeout: 5000 });
  });

  test('выбор пользователя показывает detail-панель с табами', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    const usersPage = new AdminUsersPage(page);
    await usersPage.goto();

    await usersPage.selectUserByLogin(TARGET_USER_LOGIN);

    await expect(usersPage.tabInfo).toBeVisible();
    await expect(usersPage.permissionsRoleGroupsButton).toBeVisible();
    await expect(usersPage.resetPasswordButton).toBeVisible();
  });
});
