export { canReadRegistryHistory } from './registryHistoryPermissions';

const sameID = (a, b) => Number.isSafeInteger(a) && a > 0 && a === b;

// Совпадает с серверным registryReadScope.contains; не даёт права редактирования.
export function canReadRegistryRow(permissions, entity, row, owner) {
  if (!['cars', 'employees'].includes(entity) || !row || !owner
    || !permissions.hasPermission(`entity.${entity}.read`)) return false;
  return permissions.hasPermission('section.registry.all_system')
    || sameID(row.user_id, owner.user_id)
    || (permissions.hasPermission('section.registry.organization') && sameID(row.organization_id, owner.organization_id))
    || (permissions.hasPermission('section.registry.company') && sameID(row.company_id, owner.company_id));
}
