const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const {
  createEmployee,
  deleteEmployee,
  cleanupE2eEmployees,
} = require('../helpers/employees');
const { loginAsSuperAdmin } = require('../helpers/permissions');

test.describe('Employees CRUD - create via API, verify in UI', () => {
  const createdIds = [];

  test.afterAll(async ({ request }) => {
    const token = await loginAsSuperAdmin(request).catch(() => null);
    if (token) {
      for (const id of createdIds) await deleteEmployee(request, token, id).catch(() => {});
    }
    await cleanupE2eEmployees(request).catch(() => {});
  });

  test('создание через API + отображение в /employeesview', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const lastName = `E2E_Test_${Date.now()}`;
    const employee = await createEmployee(request, token, {
      last_name: lastName,
      first_name: 'Tester',
      middle_name: 'E2E',
    });
    createdIds.push(employee.id);

    await loginAsSuperAdminUI(page);
    await page.goto('/employeesview');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});

    // Поиск по фамилии в таблице
    const cell = page.locator('table').getByText(lastName).first();
    await expect(cell).toBeVisible({ timeout: 10000 });
  });

  test('удаление через API убирает сотрудника из UI', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const lastName = `E2E_Del_${Date.now()}`;
    const employee = await createEmployee(request, token, {
      last_name: lastName,
      first_name: 'Delete',
      middle_name: 'Me',
    });

    await loginAsSuperAdminUI(page);
    await page.goto('/employeesview');
    await expect(page.locator('table').getByText(lastName).first()).toBeVisible({ timeout: 10000 });

    await deleteEmployee(request, token, employee.id);

    await page.reload();
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    await expect(page.locator('table').getByText(lastName)).toHaveCount(0, { timeout: 10000 });
  });
});
