// Гейт страниц-ошибок. Роуты /500, /maintenance и /403 - обычные пути SPA, то
// есть до этого гейта любой мог просто ввести адрес и увидеть "инцидент",
// которого не было: страница 500 рисует статус, ID и время как зафиксированные,
// а её кнопка отправляла разработчикам bug-report по несуществующей ошибке.
// Пускаем на такую страницу только когда событие действительно произошло.

import { loadBugContext, clearBugContext } from '@/composables/useBugReport';
import { useMaintenanceStore } from '@/stores/maintenance';

/**
 * Признак реального события для каждой страницы-ошибки:
 *   Error500    - контекст инцидента, сохранённый api-клиентом перед редиректом;
 *   Maintenance - включённый режим техработ (стор загружается в bootstrap);
 *   Forbidden   - permission из query, его проставляет permission-guard.
 */
const REACHABILITY = {
  Error500: () => Boolean(loadBugContext()),
  Maintenance: () => useMaintenanceStore().enabled,
  Forbidden: (to) => Boolean(to.query?.permission),
};

/**
 * Можно ли показать целевой маршрут. Для обычных страниц - всегда true,
 * решение принимается только по трём страницам-ошибкам.
 *
 * @param {import('vue-router').RouteLocationNormalized} to
 * @returns {boolean}
 */
export function isErrorPageReachable(to) {
  const hasRealEvent = REACHABILITY[to.name];
  return hasRealEvent ? hasRealEvent(to) : true;
}

/**
 * Уход с /500 закрывает инцидент: контекст одноразовый, иначе кнопкой "назад"
 * можно вернуться к уже просмотренной ошибке, хотя новой не было. Живёт в
 * guard'е, а не в самом компоненте - тогда очистка не зависит от того,
 * смонтировался ли Error500.vue.
 *
 * @param {import('vue-router').RouteLocationNormalized} to
 * @param {import('vue-router').RouteLocationNormalized} from
 */
export function closeIncidentOnLeave(to, from) {
  if (from.name === 'Error500' && to.name !== 'Error500') clearBugContext();
}
