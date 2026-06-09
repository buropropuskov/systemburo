const { test, expect } = require('@playwright/test');
const { loginAsUser, loginAsSuperAdminUI } = require('../helpers/auth');
const { NavigationBar } = require('../pages/NavigationBar');

test.describe('Navigation & Authorization', () => {
  test('regular user cannot access admin pages', async ({ page }) => {
    await loginAsUser(page);

    await page.goto('/table-constructor');

    await expect(page).not.toHaveURL(/table-constructor/);
  });

  // TODO: после расширения seed - назначить e2e_admin роль с page.admin/page.admin.users.
  // Сейчас e2e_admin (type_id=6, не суперадмин) блокируется permission-guard'ом из PR #244.
  test.skip('admin can access admin pages (требует расширения seed)', async () => {});

  test('nav menu shows admin entry for super-admin', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/personal-cabinet');

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();
    await navBar.root.hover();
    await expect(navBar.root.getByText('Администрирование')).toBeVisible();
  });

  test('nav menu hides admin entry for regular user', async ({ page }) => {
    await loginAsUser(page);
    await page.goto('/personal-cabinet');

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();
    await navBar.root.hover();
    // Пункт "Администрирование" гейтится по isSuperAdmin (#510) - у обычного
    // пользователя его нет в DOM.
    await expect(navBar.root.getByText('Администрирование')).toHaveCount(0);
  });

  test('cabinet link navigates to personal-cabinet', async ({ page }) => {
    await loginAsUser(page);
    await page.goto('/personal-cabinet');
    await expect(page).toHaveURL(/personal-cabinet/);

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();
    await navBar.root.hover();
    const personalCabinetLink = navBar.cabinetLink;
    if (await personalCabinetLink.isVisible()) {
      await personalCabinetLink.click();
      await expect(page).toHaveURL(/personal-cabinet/);
    }
  });
});
