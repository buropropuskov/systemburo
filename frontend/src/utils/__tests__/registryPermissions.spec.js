import { describe, expect, it } from 'vitest';
import { canReadRegistryRow } from '../registryPermissions';

describe('registry row scope mirrors independent section entitlements', () => {
  const owner = { user_id: 1, organization_id: 10, company_id: 20 };
  it.each(['cars', 'employees'])('checks read and membership for %s', entity => {
    const keys = ['section.registry.organization', 'section.registry.company', 'section.registry.all_system'];
    for (let mask = 0; mask < 8; mask += 1) {
      const grants = new Set([`entity.${entity}.read`, ...keys.filter((_, i) => mask & (1 << i))]);
      const perms = { hasPermission: key => grants.has(key) };
      expect(canReadRegistryRow(perms, entity, { user_id: 1 }, owner)).toBe(true);
      expect(canReadRegistryRow(perms, entity, { user_id: 2, organization_id: 10 }, owner)).toBe(Boolean(mask & 5));
      expect(canReadRegistryRow(perms, entity, { user_id: 2, company_id: 20 }, owner)).toBe(Boolean(mask & 6));
      expect(canReadRegistryRow(perms, entity, { user_id: 2 }, owner)).toBe(Boolean(mask & 4));
      grants.delete(`entity.${entity}.read`);
      expect(canReadRegistryRow(perms, entity, { user_id: 1 }, owner)).toBe(false);
    }
  });
  it('does not match missing ownership or unknown entities', () => {
    const perms = { hasPermission: () => true };
    expect(canReadRegistryRow(perms, 'cars', {}, null)).toBe(false);
    expect(canReadRegistryRow(perms, 'unknown', {}, owner)).toBe(false);
    expect(canReadRegistryRow({ hasPermission: key => key === 'entity.cars.read' }, 'cars', {}, {})).toBe(false);
  });
});
