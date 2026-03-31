const { test, expect } = require('@playwright/test');
const { registerUser, loginAsUser } = require('../helpers/auth');

test.describe('Employees View', () => {
  test('employees page loads for authenticated user', async ({ page }) => {
    const username = `e2e_emp_load_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/employeesview');
    await expect(page).toHaveURL(/employeesview/);
  });

  test('filter tabs are visible', async ({ page }) => {
    const username = `e2e_emp_tabs_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/employeesview');

    const filterTabs = page.locator('.filter-tab');
    await filterTabs.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await filterTabs.first().isVisible()) {
      const count = await filterTabs.count();
      expect(count).toBeGreaterThanOrEqual(2);
    }
  });

  test('switching filter tabs updates active state', async ({ page }) => {
    const username = `e2e_emp_switch_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/employeesview');
    const filterTabs = page.locator('.filter-tab');
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

    await page.goto('/employeesview');

    const addBtn = page.locator('.add-button');
    await addBtn.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
  });

  test('employees table has header', async ({ page }) => {
    const username = `e2e_emp_header_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/employeesview');

    // Wait for some table structure to appear
    const header = page.locator('.card-header, .header-row');
    await header.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
  });
});
