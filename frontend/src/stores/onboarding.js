import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { usePermissionsStore } from '@/stores/permissions';
import { usePDConsentStore } from '@/stores/pdConsent';
import {
  getTour,
  buildTourSteps,
  availableTours as availableToursFor,
  pickAutostartTour as pickAutostartTourFrom,
} from '@/components/onboarding/tours';
import { getOnboardingStatus, markOnboardingComplete, getSecurityFactRoute } from '@/api/onboarding';
import { getMyApprovalRole } from '@/api/approvers';

/**
 * Стор онбординг-туров. Держит ГЛОБАЛЬНЫЙ индекс активного шага по всему набору
 * шагов активного тура; хост-компонент режет набор на сегменты по route и водит
 * driver.js внутри текущего сегмента.
 *
 * Сценарий выбирается не типом пользователя, а реестром туров (tours.js): у одного
 * человека может быть право сразу на несколько туров. Статус «пройден» хранится
 * per-user И per-tour на бэкенде (не в localStorage), чтобы переживал смену
 * браузера/устройства и сбрасывался админом по конкретному туру.
 */
export const useOnboardingStore = defineStore('onboarding', () => {
  const isActive = ref(false);
  const currentIndex = ref(0);
  // Ключ активного тура: он же уходит в markCompleted и определяет набор шагов.
  const activeTourKey = ref(null);
  // Авто-запуск ставит флаг завершения даже при пропуске, ручной — нет.
  const isManual = ref(false);
  // Тур переходит между страницами: на границе сегмента driver уничтожается,
  // выполняется router.push, и pendingSegment сигналит хосту подхватить
  // следующий сегмент после навигации (router.afterEach).
  const pendingSegment = ref(false);

  // Демо-вложение для онбординга оформления заявки: тур просит BlankSelector
  // добавить вложение нужного типа ('cars'/'people'), чтобы показать реальную
  // форму; null = убрать демо-вложение (очистка после сегмента/тура).
  const demoAttachmentType = ref(null);
  function setDemoAttachment(type) {
    demoAttachmentType.value = type || null;
  }

  // Сигнал раскрытия свёрнутого узла (`reveal.open` шага): владелец узла
  // (NavMenu/App/UserApplications) реагирует watch'ем и сам закрывает то, что
  // открыл, когда сигнал гаснет. Тур в чужой DOM не лезет - см. reveal.js.
  const revealOpen = ref(null);
  function setRevealOpen(target) {
    revealOpen.value = target || null;
  }

  // Пройденные версии по турам: { [tourKey]: number|null }. Загружается одним
  // GET /onboarding; null/отсутствие ключа = тур не проходили.
  const completedByTour = ref({});
  const statusLoaded = ref(false);
  // In-flight промис загрузки статуса - чтобы конкурентные maybeAutostart
  // (onMounted + watch route) не слали два GET (урок про гонки авто-fetch).
  let loadStatusPromise = null;

  // Роль в согласовании заявок (принимающий/согласующий) - гейт туров accept и
  // approve. Правами не определяется: роль задаётся записью в справочнике.
  const approvalRole = ref({ isApprover: false, isReviewer: false });
  const approvalRoleLoaded = ref(false);
  let approvalRolePromise = null;

  // Route фактовой таблицы для шага отметки въезда/выезда в туре охраны.
  // Резолвится один раз за сессию (ensureFactRoute) из /system-tables: у разных
  // охранников разные доступные таблицы. null = подходящей таблицы нет ->
  // сегмент отметки в тур не добавляется.
  const factTableRoute = ref(null);
  let factRouteResolved = false;

  /**
   * Плоский снимок прав и ролей для гейтинга туров - реестр работает с ним, а не
   * с Pinia напрямую, поэтому правила доступности проверяются как чистые функции.
   */
  const tourContext = computed(() => {
    const auth = useAuthStore();
    const permissions = usePermissionsStore();
    return {
      isAuthenticated: auth.isAuthenticated,
      isSecurity: auth.isSecurity,
      can: (key) => permissions.hasPermission(key),
      approvalRole: approvalRole.value,
      factTableRoute: factTableRoute.value,
    };
  });

  const activeTour = computed(() => (activeTourKey.value ? getTour(activeTourKey.value) : null));

  /**
   * Шаги активного тура. Сегмент фактовой таблицы охранника добавляется в ХВОСТ
   * (buildSteps), когда route резолвлен - так индексы ранних шагов не сдвигаются,
   * даже если route доезжает уже после старта тура.
   *
   * Шаги с `requires` выбрасываются, если права нет: иначе человек без права
   * ждал бы таймаут ожидания цели и получал поповер по центру без подсветки.
   * Фильтр здесь, а не в хосте, чтобы отсутствующий шаг не попадал ни в
   * навигацию, ни в счётчик «Шаг N из M».
   */
  const steps = computed(() => {
    if (!activeTour.value) return [];
    const permissions = usePermissionsStore();
    return buildTourSteps(activeTour.value, tourContext.value)
      .filter((s) => !s.requires || permissions.hasPermission(s.requires));
  });
  const totalSteps = computed(() => steps.value.length);
  const currentStep = computed(() => steps.value[currentIndex.value] || null);

  /** Туры, доступные пользователю (написанные и прошедшие гейт) - пункты меню «Обучение». */
  const availableTours = computed(() => availableToursFor(tourContext.value));

  // Пока не дано согласие на обработку ПД, тур не показываем - ни автозапуском,
  // ни кнопкой «Обучение»: driver.js подсветил бы интерфейс под неснимаемым
  // окном согласия (#1567). После подтверждения окно уходит, флаг гаснет, и
  // автозапуск срабатывает сам - маршрут к этому моменту уже /news.
  const canShowTour = computed(() => {
    const auth = useAuthStore();
    return auth.isAuthenticated && !usePDConsentStore().required;
  });

  /**
   * Подтянуть per-user статус с бэкенда (один раз за сессию). На ошибке сети
   * statusLoaded остаётся false - хост тогда не автозапускает тур (fail-safe),
   * меню «Обучение» по-прежнему работает.
   */
  async function loadStatus() {
    if (loadStatusPromise) return loadStatusPromise;
    loadStatusPromise = (async () => {
      try {
        const data = await getOnboardingStatus();
        completedByTour.value = { ...(data?.completed || {}) };
        statusLoaded.value = true;
      } catch {
        statusLoaded.value = false;
      } finally {
        loadStatusPromise = null;
      }
    })();
    return loadStatusPromise;
  }

  /**
   * Подтянуть роль в согласовании (один раз за сессию, с in-flight промисом -
   * гейтинг зовут и меню, и автозапуск одновременно). На ошибке роль остаётся
   * пустой: туры accept/approve просто не появятся, остальное работает.
   */
  async function ensureApprovalRole() {
    if (approvalRoleLoaded.value) return;
    if (approvalRolePromise) return approvalRolePromise;
    approvalRolePromise = (async () => {
      try {
        const data = await getMyApprovalRole();
        approvalRole.value = {
          isApprover: Boolean(data?.is_approver),
          isReviewer: Boolean(data?.is_reviewer),
        };
        approvalRoleLoaded.value = true;
      } catch {
        approvalRole.value = { isApprover: false, isReviewer: false };
      } finally {
        approvalRolePromise = null;
      }
    })();
    return approvalRolePromise;
  }

  /**
   * Дождаться всего, из чего складывается гейтинг: прав, типа пользователя и роли
   * в согласовании. Автозапуск выбирает тур ОДИН раз, поэтому на холодной загрузке
   * без этого ожидания администратор получил бы тур заявителя - права ещё не
   * приехали, и его сценарий не прошёл бы гейт. Все три вызова идемпотентны
   * (кэш/in-flight), лишних запросов не добавляют.
   */
  async function ensureGatingContext() {
    const auth = useAuthStore();
    await Promise.all([
      usePermissionsStore().fetchPermissions(),
      auth.userTypeCode === null ? auth.loadUserTypeCode() : null,
      ensureApprovalRole(),
    ]);
  }

  /**
   * Тур пройден, если юзер прошёл версию не ниже текущей ВЕРСИИ ЭТОГО ТУРА.
   * Подъём версии одного тура не трогает остальные.
   *
   * @param {string} [tourKey] по умолчанию - активный тур
   * @returns {boolean}
   */
  function hasCompleted(tourKey = activeTourKey.value) {
    const tour = getTour(tourKey);
    if (!tour) return false;
    const done = completedByTour.value[tourKey];
    return done !== null && done !== undefined && done >= tour.version;
  }

  /**
   * Тур проходили, но с тех пор он обновился (пройденная версия ниже текущей) -
   * для бейджа «Обновлён» в меню.
   *
   * @param {string} tourKey
   * @returns {boolean}
   */
  function isOutdated(tourKey) {
    const tour = getTour(tourKey);
    if (!tour) return false;
    const done = completedByTour.value[tourKey];
    return done !== null && done !== undefined && done < tour.version;
  }

  /**
   * Предвычислить route фактовой таблицы для тура охраны (один раз за сессию).
   * На ошибке/отсутствии таблицы route остаётся null и сегмент отметки не
   * добавляется. Запускается фоном из start() - сегмент в хвосте steps, поэтому
   * индексы ранних шагов не сдвигаются, даже если route доедет уже после показа
   * первого шага.
   */
  async function ensureFactRoute() {
    if (factRouteResolved) return;
    factRouteResolved = true;
    factTableRoute.value = await getSecurityFactRoute();
  }

  /**
   * Запустить тур с первого шага. Идемпотентно: повторный вызов при активном
   * туре ничего не делает. Незнакомый или ещё не написанный тур не запускается.
   *
   * @param {{ tour: string, manual?: boolean }} options
   * @returns {boolean} стартовал ли тур
   */
  function start({ tour, manual = false } = {}) {
    if (isActive.value) return false;
    const entry = getTour(tour);
    if (!entry || !entry.steps.length) return false;
    activeTourKey.value = entry.key;
    isManual.value = manual;
    currentIndex.value = 0;
    isActive.value = true;
    // Фоновый резолв фактовой таблицы: не блокирует показ первого шага, сегмент
    // отметки добавится в хвост, как только route приедет.
    if (entry.key === 'guard') ensureFactRoute();
    return true;
  }

  /**
   * Тур для автозапуска: самый приоритетный из доступных и непройденных.
   *
   * @returns {object|null} запись реестра или null
   */
  function pickAutostartTour() {
    return pickAutostartTourFrom(tourContext.value, (key) => hasCompleted(key));
  }

  function stop() {
    isActive.value = false;
  }

  function setIndex(i) {
    currentIndex.value = i;
  }

  /**
   * Перейти к следующему сегменту (на другой странице): сдвинуть глобальный
   * индекс на первый шаг следующего сегмента и поднять флаг ожидания навигации.
   */
  function advanceSegment() {
    currentIndex.value += 1;
    pendingSegment.value = true;
  }

  /**
   * Вернуться к шагу на предыдущей странице (cross-page «Назад»): ставим
   * глобальный индекс на конкретный шаг и поднимаем флаг ожидания навигации.
   *
   * @param {number} index глобальный индекс шага, на который возвращаемся
   */
  function retreatSegment(index) {
    currentIndex.value = index;
    pendingSegment.value = true;
  }

  /**
   * Прыжок к шагу другого сегмента по индексу (вперёд через недостижимый
   * optional-сегмент). Механика та же, что у retreatSegment - индекс + флаг
   * ожидания навигации; отличается только семантикой направления на стороне
   * хоста (он сам делает router.push). Применяется, когда сегмент фактовой
   * таблицы недостижим и тур перепрыгивает его к финалу.
   *
   * @param {number} index глобальный индекс целевого шага
   */
  function jumpToSegment(index) {
    retreatSegment(index);
  }

  function clearPending() {
    pendingSegment.value = false;
  }

  /**
   * Пометить АКТИВНЫЙ тур пройденным для текущего юзера. Локально обновляем карту
   * сразу (чтобы автозапуск не сработал повторно в этой сессии), запись на
   * бэкенд - fire-and-forget: сбой сети не должен ломать завершение тура (в
   * худшем случае переиграется при следующем входе).
   */
  function markCompleted() {
    const tour = activeTour.value;
    if (!tour) return;
    // Идемпотентно: разные пути закрытия (Esc/Готово/CTA) могут позвать дважды -
    // версию ставим и POST шлём только если ещё не помечено.
    if (hasCompleted(tour.key)) return;
    completedByTour.value = { ...completedByTour.value, [tour.key]: tour.version };
    statusLoaded.value = true;
    markOnboardingComplete(tour.key, tour.version).catch(() => {
      // best-effort: статус локально уже стоит, бэкенд догонит при следующем разе
    });
  }

  function reset() {
    isActive.value = false;
    currentIndex.value = 0;
    activeTourKey.value = null;
    pendingSegment.value = false;
    demoAttachmentType.value = null;
    revealOpen.value = null;
    // Сброс статуса при logout - следующий юзер на этом устройстве подтянет свой.
    completedByTour.value = {};
    statusLoaded.value = false;
    // Роль в согласовании и route фактовой таблицы тоже per-user.
    approvalRole.value = { isApprover: false, isReviewer: false };
    approvalRoleLoaded.value = false;
    factTableRoute.value = null;
    factRouteResolved = false;
  }

  return {
    demoAttachmentType,
    setDemoAttachment,
    revealOpen,
    setRevealOpen,
    isActive,
    currentIndex,
    activeTourKey,
    activeTour,
    steps,
    totalSteps,
    currentStep,
    availableTours,
    tourContext,
    canShowTour,
    isManual,
    pendingSegment,
    completedByTour,
    statusLoaded,
    approvalRole,
    approvalRoleLoaded,
    factTableRoute,
    ensureApprovalRole,
    ensureGatingContext,
    ensureFactRoute,
    loadStatus,
    hasCompleted,
    isOutdated,
    pickAutostartTour,
    start,
    stop,
    setIndex,
    advanceSegment,
    retreatSegment,
    jumpToSegment,
    clearPending,
    markCompleted,
    reset,
  };
});
