const { test, expect } = require('@playwright/test');
const { loginAsUser, loginAsSuperAdminUI } = require('../helpers/auth');
const { NavigationBar } = require('../pages/NavigationBar');

test.describe('Navigation & Authorization', () => {
  test('regular user cannot access admin pages', async ({ page }) => {
    await loginAsUser(page);

    await page.goto('/table-constructor');

    // permission-guard кидает на /403 (исходный путь - в query from=, поэтому
    // проверяем посадку на Forbidden, а не отсутствие table-constructor в URL).
    await expect(page).toHaveURL(/\/403/);
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
    await expect(navBar.adminEntry).toBeVisible();
  });

  test('nav menu hides admin entry for regular user', async ({ page }) => {
    await loginAsUser(page);
    await page.goto('/personal-cabinet');

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();
    await navBar.root.hover();
    // Секция "Администрирование" гейтится по isSuperAdmin (#510) - у обычного
    // пользователя её нет в DOM.
    await expect(navBar.adminEntry).toHaveCount(0);
  });

  test('admin entry opens two-column admin panel and navigates sections', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/personal-cabinet');

    const navBar = new NavigationBar(page);
    await expect(navBar.root).toBeVisible();

    await navBar.openAdmin();
    // Колонка открылась, разделы видны.
    await expect(navBar.adminColumn).toBeVisible();
    await expect(navBar.adminSection('users')).toBeVisible();
    await expect(navBar.adminSection('organizations')).toBeVisible();

    // Переход по разделу - SPA-навигация, колонка остаётся открытой, раздел активен.
    await navBar.adminSection('organizations').click();
    await expect(page).toHaveURL(/\/admin\/organizations/);
    await expect(navBar.adminColumn).toBeVisible();
    await expect(navBar.adminSection('organizations')).toHaveClass(/active/);

    // "Назад в работу" закрывает колонку, маршрут не меняется.
    await navBar.adminBack.click();
    await expect(navBar.adminColumn).toHaveCount(0);
    await expect(page).toHaveURL(/\/admin\/organizations/);

    // Esc тоже закрывает колонку.
    await navBar.openAdmin();
    await expect(navBar.adminColumn).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(navBar.adminColumn).toHaveCount(0);
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
