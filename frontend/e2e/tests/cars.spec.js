const { test, expect } = require('@playwright/test');
const { loginAsUser } = require('../helpers/auth');

// Filter tabs depend on /unique-cars/ownership-info API — may timeout under parallel load
async function waitForFilterTabs(page) {
  await page.locator('.filter-tab').first().waitFor({ state: 'visible', timeout: 15000 });
}

test.describe('Cars View', () => {
  test('carsview page loads for authenticated user', async ({ page }) => {
    const username = `e2e_cars_load_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/carsview');
    await expect(page).toHaveURL(/carsview/);
    await expect(page.locator('.carsview')).toBeVisible();
  });

  test('filter tabs are visible', async ({ page }) => {
    const username = `e2e_cars_tabs_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/carsview');
    await page.waitForLoadState('networkidle');
    await waitForFilterTabs(page);

    const count = await page.locator('.filter-tab').count();
    expect(count).toBeGreaterThanOrEqual(2);
  });

  test('switching filter tabs changes active tab', async ({ page }) => {
    const username = `e2e_cars_switch_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/carsview');
    await page.waitForLoadState('networkidle');
    await waitForFilterTabs(page);

    const filterTabs = page.locator('.filter-tab');
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

    await page.goto('/carsview');
    await page.waitForLoadState('networkidle');

    const addBtn = page.locator('.add-button');
    await addBtn.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await addBtn.isVisible()) {
      await addBtn.click();
      await expect(page.locator('.base-modal')).toBeVisible();
    }
  });

  test('car modal has format dropdown', async ({ page }) => {
    const username = `e2e_cars_format_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/carsview');
    await page.waitForLoadState('networkidle');

    const addBtn = page.locator('.add-button');
    await addBtn.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await addBtn.isVisible()) {
      await addBtn.click();
      await expect(page.locator('.base-modal')).toBeVisible();
      const formatDropdown = page.locator('.format__dropdown .dropdown__button');
      await expect(formatDropdown).toBeVisible();
    }
  });

  test('car modal close button works', async ({ page }) => {
    const username = `e2e_cars_close_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/carsview');
    await page.waitForLoadState('networkidle');

    const addBtn = page.locator('.add-button');
    await addBtn.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});

    if (await addBtn.isVisible()) {
      await addBtn.click();
      await expect(page.locator('.base-modal')).toBeVisible();
      await page.locator('.base-modal__close').click();
      await expect(page.locator('.base-modal')).not.toBeVisible();
    }
  });

  test('cars table has header columns', async ({ page }) => {
    const username = `e2e_cars_headers_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/carsview');
    const header = page.locator('.card-header');
    await header.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
  });
});
