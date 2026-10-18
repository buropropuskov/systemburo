// Общий журнал реестра сохраняет административную область доступа.
export function canReadRegistryHistory(permissions, entity) {
  return ['cars', 'employees'].includes(entity)
    && ['admin', 'super'].includes(permissions.mode)
    && permissions.hasPermission(`entity.${entity}.read`)
    && permissions.hasPermission('detail.full_history');
}
