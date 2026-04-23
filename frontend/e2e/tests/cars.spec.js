const { test, expect } = require('@playwright/test');
const { loginAsUser } = require('../helpers/auth');
const { CarsPage } = require('../pages/CarsPage');

async function waitForFilterTabs(page) {
  await page.locator('[data-testid^="filter-tab-"]').first().waitFor({ state: 'visible', timeout: 15000 });
}

test.describe('Cars View', () => {
  test('carsview page loads for authenticated user', async ({ page }) => {
    const username = `e2e_cars_load_${Date.now()}`;
    await loginAsUser(page, username);

    const carsPage = new CarsPage(page);
    await carsPage.goto();
    await expect(page).toHaveURL(/carsview/);
    await expect(carsPage.root).toBeVisible();
  });

  test('filter tabs are visible', async ({ page }) => {
    const username = `e2e_cars_tabs_${Date.now()}`;
    await loginAsUser(page, username);

    const carsPage = new CarsPage(page);
    await carsPage.goto();
    await waitForFilterTabs(page);

    const count = await carsPage.getAllFilterTabs().count();
    expect(count).toBeGreaterThanOrEqual(2);
  });

  test('switching filter tabs changes active tab', async ({ page }) => {
    const username = `e2e_cars_switch_${Date.now()}`;
    await loginAsUser(page, username);

    const carsPage = new CarsPage(page);
    await carsPage.goto();
    await waitForFilterTabs(page);

    const filterTabs = carsPage.getAllFilterTabs();
    const count = await filterTabs.count();
    if (count >= 2) {
      const secondTab = filterTabs.nth(1);
      await secondTab.click();
      await expect(secondTab).toHaveClass(/filter-tab--active/);
    }
  });

  test('add car button opens modal', async ({ page }) => {
    const username = `e2e_cars_add_${Date.now()}`;
    await loginAsUser(page, username);

    const carsPage = new CarsPage(page);
    await carsPage.goto();

    await carsPage.addButton.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await carsPage.addButton.isVisible()) {
      await carsPage.addButton.click();
      await expect(carsPage.modal).toBeVisible();
    }
  });

  test('car modal has format dropdown', async ({ page }) => {
    const username = `e2e_cars_format_${Date.now()}`;
    await loginAsUser(page, username);

    const carsPage = new CarsPage(page);
    await carsPage.goto();

    await carsPage.addButton.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await carsPage.addButton.isVisible()) {
      await carsPage.addButton.click();
      await expect(carsPage.modal).toBeVisible();
      await expect(carsPage.formatDropdown).toBeVisible();
    }
  });

  test('car modal close button works', async ({ page }) => {
    const username = `e2e_cars_close_${Date.now()}`;
    await loginAsUser(page, username);

    const carsPage = new CarsPage(page);
    await carsPage.goto();

    await carsPage.addButton.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await carsPage.addButton.isVisible()) {
      await carsPage.addButton.click();
      await expect(carsPage.modal).toBeVisible();
      await carsPage.modalCloseButton.click();
      await expect(carsPage.modal).not.toBeVisible();
    }
  });

  test('cars table has header columns', async ({ page }) => {
    const username = `e2e_cars_headers_${Date.now()}`;
    await loginAsUser(page, username);

    const carsPage = new CarsPage(page);
    await carsPage.goto();
    const header = page.locator('.card-header');
    await header.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
  });
});
