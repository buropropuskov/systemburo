const { test, expect } = require('@playwright/test');
const {
  listEmployees,
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

  test('POST /unique-employees создаёт сотрудника, GET возвращает его', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const lastName = `E2E_Test_${Date.now()}`;
    const employee = await createEmployee(request, token, {
      last_name: lastName,
      first_name: 'Tester',
      middle_name: 'E2E',
      organization_id: 1,
    });
    createdIds.push(employee.id);

    const all = await listEmployees(request, token);
    const found = all.find(e => e.id === employee.id);
    expect(found).toBeTruthy();
    expect(found.last_name).toBe(lastName);
  });

  test('DELETE /unique-employees/:id убирает сотрудника из списка', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    const lastName = `E2E_Del_${Date.now()}`;
    const employee = await createEmployee(request, token, {
      last_name: lastName,
      first_name: 'Delete',
      middle_name: 'Me',
      organization_id: 1,
    });

    await deleteEmployee(request, token, employee.id);

    const all = await listEmployees(request, token);
    expect(all.find(e => e.id === employee.id)).toBeFalsy();
  });
});
