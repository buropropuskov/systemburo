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

  test('создание через API + отображение в /carsview', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const carNumber = `E2E${Date.now().toString().slice(-7)}`;
    const car = await createCar(request, token, {
      number: carNumber,
      mark: e2eName('mark'),
      status: false,
    });
    createdIds.push(car.id);
    expect(car.number).toBe(carNumber);

    await loginAsSuperAdminUI(page);
    await page.goto('/carsview');
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});

    // Найти машину в таблице - текст с номером
    const cell = page.locator('table').getByText(carNumber).first();
    await expect(cell).toBeVisible({ timeout: 10000 });
  });

  test('удаление через API убирает машину из UI', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const carNumber = `E2E${Date.now().toString().slice(-7)}`;
    const car = await createCar(request, token, {
      number: carNumber,
      mark: e2eName('mark2'),
      status: false,
    });
    // НЕ добавляем в createdIds - удалим вручную в тесте

    await loginAsSuperAdminUI(page);
    await page.goto('/carsview');
    await expect(page.locator('table').getByText(carNumber).first()).toBeVisible({ timeout: 10000 });

    // Удаляем через API
    await deleteCar(request, token, car.id);

    // Перезагружаем и проверяем что нет
    await page.reload();
    await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
    await expect(page.locator('table').getByText(carNumber)).toHaveCount(0, { timeout: 10000 });
  });
});
