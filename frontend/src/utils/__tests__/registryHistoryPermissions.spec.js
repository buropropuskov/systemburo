import { describe, expect, it } from 'vitest';
import { canReadRegistryHistory } from '../registryHistoryPermissions';

describe('registry journal permissions', () => {
  it.each(['cars', 'employees'])('requires admin, read and history for %s', (entity) => {
    const perms = { mode: 'admin', hasPermission: () => true };
    expect(canReadRegistryHistory(perms, entity)).toBe(true);
    perms.mode = 'normal';
    expect(canReadRegistryHistory(perms, entity)).toBe(false);
    perms.mode = 'admin';
    for (const denied of [`entity.${entity}.read`, 'detail.full_history']) {
      perms.hasPermission = (key) => key !== denied;
      expect(canReadRegistryHistory(perms, entity)).toBe(false);
    }
    perms.mode = 'super'; perms.hasPermission = () => true;
    expect(canReadRegistryHistory(perms, entity)).toBe(true);
    expect(canReadRegistryHistory(perms, 'other')).toBe(false);
  });
});
