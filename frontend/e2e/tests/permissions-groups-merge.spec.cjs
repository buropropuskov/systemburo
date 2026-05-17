const { test, expect } = require('@playwright/test');
const {
  loginAsSuperAdmin,
  createGroup,
  listGroups,
  deleteGroup,
  mergeGroups,
  cleanupE2eEntities,
  e2eName,
} = require('../helpers/permissions');

// Merge - операция переноса юзера со старых групп на новую объединённую.
// Эндпоинт требует user_id + source_group_ids + new_group_name. Тест
// делает merge для самого superadmin'а - семантически странно но валидно
// (superadmin суперюзер, доп-группы у него игнорируются resolver'ом).
test.describe('Admin / Permission Groups - merge via API', () => {
  const createdIds = [];
  let superAdminUserId = null;

  test.beforeAll(async ({ request }) => {
    // Узнаём ID супер-админа через login - JWT содержит user_id, но проще
    // через парс ответа (он включает user_id в payload).
    const SUPER = require('../helpers/permissions').SUPER_ADMIN;
    const apiBase = process.env.E2E_API_BASE_URL || '/api';
    const res = await request.post(`${apiBase}/login`, {
      data: { username: SUPER.username, password: SUPER.password },
    });
    const body = await res.json();
    const token = body.data.token;
    // user_id в JWT payload
    const payload = JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString());
    superAdminUserId = payload.user_id;
    expect(superAdminUserId).toBeGreaterThan(0);
  });

  test.afterAll(async ({ request }) => {
    const token = await loginAsSuperAdmin(request).catch(() => null);
    if (token) {
      for (const id of createdIds) {
        await deleteGroup(request, token, id).catch(() => {});
      }
    }
    await cleanupE2eEntities(request).catch(() => {});
  });

  test('merge двух групп даёт union их keys', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);

    const groupA = await createGroup(request, token, {
      name: e2eName('merge_a'),
      keys: ['page.cars', 'entity.cars.read'],
    });
    const groupB = await createGroup(request, token, {
      name: e2eName('merge_b'),
      keys: ['page.employees', 'entity.employees.read'],
    });
    createdIds.push(groupA.id, groupB.id);

    const mergedName = e2eName('merge_result');
    const merged = await mergeGroups(request, token, {
      user_id: superAdminUserId,
      source_group_ids: [groupA.id, groupB.id],
      new_group_name: mergedName,
    });
    createdIds.push(merged.id);

    expect(merged.name).toBe(mergedName);
    // Union должен содержать все 4 ключа (порядок может отличаться)
    expect([...merged.keys].sort()).toEqual([
      'entity.cars.read',
      'entity.employees.read',
      'page.cars',
      'page.employees',
    ]);

    // Проверим что merged видна в списке
    const all = await listGroups(request, token);
    expect(all.find(g => g.id === merged.id)).toBeTruthy();
  });
});
