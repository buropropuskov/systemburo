const { test, expect } = require('@playwright/test');
const {
  loginAsSuperAdmin,
  createRole,
  createGroup,
  setRoleDefaultGroups,
  cleanupE2eEntities,
  e2eName,
} = require('../helpers/permissions');

test.describe('Admin / Role-Group binding (через API)', () => {
  test.afterAll(async ({ request }) => {
    await cleanupE2eEntities(request).catch(() => {});
  });

  test('создаёт роль, группу, привязывает default-группу к роли', async ({ request }) => {
    const token = await loginAsSuperAdmin(request);

    const groupName = e2eName('grp');
    const group = await createGroup(request, token, { name: groupName, keys: [] });
    expect(group.id).toBeTruthy();

    const roleCode = e2eName('role');
    const role = await createRole(request, token, {
      name: `Bound Role ${roleCode}`,
      code: roleCode,
    });
    expect(role.id).toBeTruthy();

    await setRoleDefaultGroups(request, token, role.id, [group.id]);
  });
});
