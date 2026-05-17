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

// Merge UI отсутствует в AdminPermissionGroups.vue - только POST /api/permission-groups/merge.
// Тест проверяет API: union ключей двух групп даёт новую группу со всеми ключами.
test.describe('Admin / Permission Groups - merge via API', () => {
  const createdIds = [];

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
      name: mergedName,
      source_ids: [groupA.id, groupB.id],
    });
    createdIds.push(merged.id);

    expect(merged.name).toBe(mergedName);
    // Union должен содержать все 4 ключа
    expect(merged.keys.sort()).toEqual([
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
