const { test, expect } = require('@playwright/test');
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

  test('POST /unique-cars создаёт машину, GET возвращает её', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const carNumber = `E2E${Date.now().toString().slice(-7)}`;
    const car = await createCar(request, token, {
      number: carNumber,
      mark: e2eName('mark'),
      status: false,
      organization_id: 1,
    });
    createdIds.push(car.id);
    expect(car.number).toBe(carNumber);

    const all = await listCars(request, token);
    const found = all.find(c => c.id === car.id);
    expect(found).toBeTruthy();
    expect(found.number).toBe(carNumber);
  });

  test('DELETE /unique-cars/:id убирает машину из списка', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const carNumber = `E2E${Date.now().toString().slice(-7)}`;
    const car = await createCar(request, token, {
      number: carNumber,
      mark: e2eName('mark2'),
      status: false,
      organization_id: 1,
    });

    await deleteCar(request, token, car.id);

    const all = await listCars(request, token);
    expect(all.find(c => c.id === car.id)).toBeFalsy();
  });
});
