import { ref } from 'vue';
import { getSecurityFactRoute } from '@/api/onboarding';
import { getMyApprovalRole } from '@/api/approvers';
import { getUserApplicationsPaginated } from '@/api/applications';

/**
 * Данные, от которых зависит СОСТАВ тура: роль в согласовании, маршрут фактовой
 * таблицы охранника и наличие у человека собственной заявки.
 *
 * Живут отдельно от стора по одной причине: их надо знать ДО старта. Пока состав
 * шагов выяснялся по ходу, тур платил за это дважды - ожиданием цели, которой на
 * экране не будет, и знаменателем «Шаг N из M», который таял на глазах (у нового
 * человека 57 -> 55 -> 48). Каждый резолвер идемпотентен и держит in-flight
 * промис: гейтинг зовут и меню, и автозапуск, а запрос нужен один.
 *
 * Отклонённый запрос оставляет безопасные пустые данные, но не завершает резолв:
 * следующий вызов может повторить запрос. reset отделяет ответы прежнего
 * пользователя от нового контекста, включая ещё не законченный импорт auth.
 * Хелпер фактовой таблицы сам сводит ошибки к null; его контракт здесь сохранён.
 *
 * @returns {{
 *   approvalRole: import('vue').Ref<{isApprover: boolean, isReviewer: boolean}>,
 *   approvalRoleLoaded: import('vue').Ref<boolean>,
 *   factTableRoute: import('vue').Ref<string|null>,
 *   hasOwnApplication: import('vue').Ref<boolean>,
 *   ensureApprovalRole: () => Promise<void>,
 *   ensureFactRoute: () => Promise<void>,
 *   ensureOwnApplication: () => Promise<void>,
 *   reset: () => void,
 * }}
 */
export function createGatingData() {
  let generation = 0;
  const approvalRole = ref({ isApprover: false, isReviewer: false });
  const approvalRoleLoaded = ref(false);
  let approvalRolePromise = null;

  const factTableRoute = ref(null);
  let factRouteResolved = false;
  let factRoutePromise = null;

  const hasOwnApplication = ref(false);
  let ownApplicationResolved = false;
  let ownApplicationPromise = null;

  async function ensureApprovalRole() {
    if (approvalRoleLoaded.value) return;
    if (approvalRolePromise) return approvalRolePromise;
    const requestGeneration = generation;
    approvalRolePromise = Promise.resolve().then(async () => {
      if (requestGeneration !== generation) return;
      try {
        const data = await getMyApprovalRole();
        if (requestGeneration !== generation) return;
        approvalRole.value = {
          isApprover: Boolean(data?.is_approver),
          isReviewer: Boolean(data?.is_reviewer),
        };
        approvalRoleLoaded.value = true;
      } catch {
        if (requestGeneration !== generation) return;
        approvalRole.value = { isApprover: false, isReviewer: false };
      } finally {
        if (requestGeneration === generation) approvalRolePromise = null;
      }
    });
    return approvalRolePromise;
  }

  async function ensureFactRoute() {
    if (factRouteResolved) return;
    if (factRoutePromise) return factRoutePromise;
    const requestGeneration = generation;
    factRoutePromise = Promise.resolve().then(async () => {
      if (requestGeneration !== generation) return;
      try {
        const route = await getSecurityFactRoute();
        if (requestGeneration !== generation) return;
        factTableRoute.value = route;
        factRouteResolved = true;
      } catch {
        if (requestGeneration !== generation) return;
        factTableRoute.value = null;
      } finally {
        if (requestGeneration === generation) factRoutePromise = null;
      }
    });
    return factRoutePromise;
  }

  /**
   * Нужен только факт наличия, поэтому просим одну запись - и ровно с тем же
   * фильтром, что стоит в кабинете по умолчанию (вкладка «Мои заявки»,
   * `sender_user_id`). Без фильтра ответ приходит со заявками ОРГАНИЗАЦИИ, и
   * человек, не подавший ни одной заявки, считался бы «с заявкой»: шаги про её
   * карточку оставались в туре, а строки в списке так и не появлялось.
   */
  async function ensureOwnApplication() {
    if (ownApplicationResolved) return;
    if (ownApplicationPromise) return ownApplicationPromise;
    const requestGeneration = generation;
    ownApplicationPromise = Promise.resolve().then(async () => {
      if (requestGeneration !== generation) return;
      try {
        // Стор авторизации подтягиваем на месте: статический импорт замыкал круг
        // (стор онбординга -> этот модуль -> стор авторизации -> ... ), и в
        // собранном бандле реестр туров успевал прочитаться пустым - меню
        // «Обучение» открывалось без единого пункта.
        const { useAuthStore } = await import('@/stores/auth');
        if (requestGeneration !== generation) return;
        const senderId = useAuthStore().userId;
        const { items, meta } = await getUserApplicationsPaginated({
          page: 1,
          per_page: 1,
          ...(senderId ? { sender_user_id: senderId } : {}),
        });
        if (requestGeneration !== generation) return;
        hasOwnApplication.value = Boolean(meta?.total > 0 || items?.length);
        ownApplicationResolved = true;
      } catch {
        if (requestGeneration !== generation) return;
        hasOwnApplication.value = false;
      } finally {
        if (requestGeneration === generation) ownApplicationPromise = null;
      }
    });
    return ownApplicationPromise;
  }

  /** Смена пользователя: всё это - per-user, следующий подтянет своё. */
  function reset() {
    generation += 1;
    approvalRole.value = { isApprover: false, isReviewer: false };
    approvalRoleLoaded.value = false;
    approvalRolePromise = null;
    factTableRoute.value = null;
    factRouteResolved = false;
    factRoutePromise = null;
    hasOwnApplication.value = false;
    ownApplicationResolved = false;
    ownApplicationPromise = null;
  }

  return {
    approvalRole,
    approvalRoleLoaded,
    factTableRoute,
    hasOwnApplication,
    ensureApprovalRole,
    ensureFactRoute,
    ensureOwnApplication,
    reset,
  };
}
