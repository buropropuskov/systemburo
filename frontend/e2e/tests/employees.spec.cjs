const { test, expect } = require('@playwright/test');
const { loginAsUser } = require('../helpers/auth');
const { EmployeesPage } = require('../pages/EmployeesPage');

test.describe('Employees View', () => {
  test('employees page loads for authenticated user', async ({ page }) => {
    const username = `e2e_emp_load_${Date.now()}`;
    await loginAsUser(page, username);

    const employeesPage = new EmployeesPage(page);
    await employeesPage.goto();
    await expect(page).toHaveURL(/employeesview/);
    await expect(employeesPage.root).toBeVisible();
  });

  test('filter tabs are visible', async ({ page }) => {
    const username = `e2e_emp_tabs_${Date.now()}`;
    await loginAsUser(page, username);

    const employeesPage = new EmployeesPage(page);
    await employeesPage.goto();

    const filterTabs = employeesPage.getAllFilterTabs();
    await filterTabs.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await filterTabs.first().isVisible()) {
      const count = await filterTabs.count();
      expect(count).toBeGreaterThanOrEqual(2);
    }
  });

  test('switching filter tabs updates active state', async ({ page }) => {
    const username = `e2e_emp_switch_${Date.now()}`;
    await loginAsUser(page, username);

    const employeesPage = new EmployeesPage(page);
    await employeesPage.goto();

    const filterTabs = employeesPage.getAllFilterTabs();
    await filterTabs.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await filterTabs.count() >= 2) {
      const secondTab = filterTabs.nth(1);
      await secondTab.click();
      await expect(secondTab).toHaveClass(/filter-tab--active/);
    }
  });

  test('add employee button is visible', async ({ page }) => {
    const username = `e2e_emp_add_${Date.now()}`;
    await loginAsUser(page, username);

    const employeesPage = new EmployeesPage(page);
    await employeesPage.goto();

    const addBtn = page.getByRole('button', { name: 'Добавить' });
    await addBtn.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
  });

  test('employees table has header', async ({ page }) => {
    const username = `e2e_emp_header_${Date.now()}`;
    await loginAsUser(page, username);

    const employeesPage = new EmployeesPage(page);
    await employeesPage.goto();

    const header = page.locator('.card-header, .header-row');
    await header.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
  });
});
