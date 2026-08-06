import { usePermissionsStore } from '@/stores/permissions';

/**
 * Строит маршрут к заявке из уведомления (#973): заявитель без доступа к
 * Центру заявок открывает её в личном кабинете. Раньше этот выбор пути был
 * продублирован в UserNotifications.vue и UserNotificationsInline.vue; с
 * появлением модалки подробностей (#1748 S6) переход стал третьим местом,
 * куда его надо было бы копировать - вынесен сюда.
 *
 * Composable только СТРОИТ маршрут (чтение permissions-стора), сам переход
 * (`$router.push`) и закрытие панели остаются на вызывающем компоненте.
 *
 * @returns {{ resolveApplicationRoute: (applicationId: number) => { path: string, query: { open: number } } }}
 */
export function useNotificationNavigation() {
  function resolveApplicationRoute(applicationId) {
    const path = usePermissionsStore().hasPermission('page.center') ? '/center' : '/personal-cabinet';
    return { path, query: { open: applicationId } };
  }

  return { resolveApplicationRoute };
}
