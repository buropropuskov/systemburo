const { test, expect } = require('@playwright/test');
const { loginAsUser } = require('../helpers/auth');

test.describe('Admin-only Access Control', () => {
  test('non-admin redirected from /admin/users', async ({ page }) => {
    await loginAsUser(page);

    await page.goto('/admin/users');
    // permission-guard кидает на /403 (исходный путь - в query from=, поэтому
    // проверяем именно посадку на Forbidden, а не отсутствие admin/users в URL).
    await expect(page).toHaveURL(/\/403/);
  });

  test('non-admin redirected from /admin/settings', async ({ page }) => {
    await loginAsUser(page);

    await page.goto('/admin/settings');
    await expect(page).toHaveURL(/\/403/);
  });

  // TODO: после расширения seed - назначить e2e_admin роль с permission.audit.manage
  // и page.admin.users. Сейчас e2e_admin (type_id=6, не суперадмин) не имеет ключей
  // и блокируется permission-guard'ом из PR #244.
  test.skip('admin can access /admin/users (требует расширения seed)', async () => {});
});
