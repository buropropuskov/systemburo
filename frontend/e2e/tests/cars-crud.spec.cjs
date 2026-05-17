const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const {
  listCars,
  createCar,
  deleteCar,
  cleanupE2eCars,
} = require('../helpers/cars');
const { loginAsSuperAdmin, e2eName } = require('../helpers/permissions');

test.describe('Cars CRUD - create via API, verify in UI', () => {
  const createdIds = [];

  test.afterAll(async ({ request }) => {
    const token = await loginAsSuperAdmin(request).catch(() => null);
    if (token) {
      for (const id of createdIds) await deleteCar(request, token, id).catch(() => {});
    }
    await cleanupE2eCars(request).catch(() => {});
  });

  test('создание через API + отображение в /carsview (фильтр "Все")', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const carNumber = `E2E${Date.now().toString().slice(-7)}`;
    const car = await createCar(request, token, {
      number: carNumber,
      mark: e2eName('mark'),
      status: false,
      // organization_id=1 (Бюро пропусков) чтобы машина была видна superadmin'у
      organization_id: 1,
    });
    createdIds.push(car.id);
    expect(car.number).toBe(carNumber);

    await loginAsSuperAdminUI(page);
    await page.goto('/carsview');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});

    // Переключиться на фильтр "Все в системе" чтобы наверняка увидеть
    const allTab = page.getByRole('button', { name: /Все в системе/ });
    if (await allTab.isVisible({ timeout: 2000 }).catch(() => false)) {
      await allTab.click();
      await page.waitForTimeout(500);
    }

    const cell = page.locator('table').getByText(carNumber).first();
    await expect(cell).toBeVisible({ timeout: 15000 });
  });

  test('удаление через API убирает машину из UI', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const carNumber = `E2E${Date.now().toString().slice(-7)}`;
    const car = await createCar(request, token, {
      number: carNumber,
      mark: e2eName('mark2'),
      status: false,
      organization_id: 1,
    });

    await loginAsSuperAdminUI(page);
    await page.goto('/carsview');
    const allTab = page.getByRole('button', { name: /Все в системе/ });
    if (await allTab.isVisible({ timeout: 2000 }).catch(() => false)) {
      await allTab.click();
      await page.waitForTimeout(500);
    }
    await expect(page.locator('table').getByText(carNumber).first()).toBeVisible({ timeout: 15000 });

    await deleteCar(request, token, car.id);

    await page.reload();
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    if (await allTab.isVisible({ timeout: 2000 }).catch(() => false)) {
      await allTab.click();
      await page.waitForTimeout(500);
    }
    await expect(page.locator('table').getByText(carNumber)).toHaveCount(0, { timeout: 15000 });
  });
});
