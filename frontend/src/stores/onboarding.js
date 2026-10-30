import { defineStore } from 'pinia';
import { ref, computed, watch } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { usePermissionsStore } from '@/stores/permissions';
import { usePDConsentStore } from '@/stores/pdConsent';
import {
  getTour,
  availableTours as availableToursFor,
  pickAutostartTour as pickAutostartTourFrom,
} from '@/components/onboarding/tours';
import { markOnboardingComplete } from '@/api/onboarding';
import { createGatingData } from '@/components/onboarding/gatingData';
import { createTourDemoData } from '@/components/onboarding/tourDemoData';
import { createProgressTracker } from '@/components/onboarding/tourProgress';
import { createTourStatus } from '@/components/onboarding/tourStatus';
import { createActiveStepSet, applicableTourSteps } from '@/components/onboarding/activeStepSet';
import { createTourContext } from '@/components/onboarding/tourContext';
import { createContextReadiness, factPolicyKey } from '@/components/onboarding/contextReadiness';

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
  const currentStepId = ref(null);
  // Ключ активного тура: он же уходит в markCompleted и определяет набор шагов.
  const activeTourKey = ref(null);
  const demo = createTourDemoData({ getTourKey: () => activeTourKey.value, getActorId: () => Number(useAuthStore().userId) });
  // Авто-запуск ставит флаг завершения даже при пропуске, ручной — нет.
  const isManual = ref(false);
  const partialRun = ref(false);
  // Тур переходит между страницами: на границе сегмента driver уничтожается,
  // выполняется router.push, и pendingSegment сигналит хосту подхватить
  // следующий сегмент после навигации (router.afterEach).
  const pendingSegment = ref(false);

  // Шаги, реально выброшенные в ЭТОМ прохождении: цели на экране не оказалось
  // (кнопки, которой человеку не положено; пустого списка). Нужны для честной
  // нумерации: «Шаг N из M» считается по пройденному маршруту. Раньше из счёта
  // выбрасывались все `optional` разом - и на карточке заявки, где optional
  // ВЕСЬ сегмент, счётчик замирал на девять шагов подряд.
  const skippedIndexes = ref([]);
  function markSkipped(index) {
    if (skippedIndexes.value.includes(index)) return;
    skippedIndexes.value = [...skippedIndexes.value, index];
  }

  // Сигнал раскрытия свёрнутого узла (`reveal.open` шага): владелец узла
  // (NavMenu/App/UserApplications) реагирует watch'ем и сам закрывает то, что
  // открыл, когда сигнал гаснет. Тур в чужой DOM не лезет - см. reveal.js.
  const revealOpen = ref(null), revealOpenRevision = ref(0);
  function setRevealOpen(target) {
    revealOpen.value = target || null; revealOpenRevision.value++;
  }

  // Пройденные версии по турам: { [tourKey]: number|null }. Загружается одним
  // GET /onboarding; null/отсутствие ключа = тур не проходили.
  const {
    completedByTour, finishedTours, loaded: statusLoaded, load: loadStatus, reset: resetStatus,
  } = createTourStatus();
  // In-flight промис загрузки статуса - чтобы конкурентные maybeAutostart
  // (onMounted + watch route) не слали два GET (урок про гонки авто-fetch).

  // Роль в согласовании заявок (принимающий/согласующий) - гейт туров accept и
  // approve. Правами не определяется: роль задаётся записью в справочнике.

  // Route фактовой таблицы для шага отметки въезда/выезда в туре охраны.
  // С какого шага подняли тур в этот раз: 0 - с начала, больше - продолжили с
  // сохранённой позиции. Хост показывает это в первом поповере.
  const resumedFrom = ref(0);

  /** Прогресс хранится по пользователю: за одним компьютером работают посменно. */
  const progress = createProgressTracker(() => useAuthStore().userId);

  // Роль в согласовании, маршрут фактовой таблицы и наличие своей заявки живут в
  // gatingData.js: от них зависит СОСТАВ тура, поэтому они резолвятся до старта.
  const gating = createGatingData({ factPolicyKey: () => factPolicyKey(usePermissionsStore()),
    availablePolicyKey: () => JSON.stringify([usePermissionsStore().loaded, usePermissionsStore().hasPermission('page.available')]) });
  const {
    approvalRole, approvalRoleLoaded, factTableRoute, hasOwnApplication,
    ensureApprovalRole, ensureFactRoute, ensureOwnApplication, reset: resetGatingData,
    factRouteLoaded, ownApplicationLoaded, securityPostContext,
    accessibleContext, accessibleContextLoaded,
  } = gating;
  const contextReady = createContextReadiness({ auth: useAuthStore(), permissions: usePermissionsStore(),
    data: gating, getContext: () => tourContext.value });
  const { ensureSelectionContext, prepareTour } = contextReady;

  /**
   * Плоский снимок прав и ролей для гейтинга туров - реестр работает с ним, а не
   * с Pinia напрямую, поэтому правила доступности проверяются как чистые функции.
   */
  const tourContext = createTourContext(useAuthStore, usePermissionsStore, () => ({
    approvalRole, factTableRoute, hasOwnApplication, securityPostContext,
    userTypeCode: contextReady.knownType(), factRouteLoaded,
    accessibleContext,
  }));

  const activeTour = computed(() => (activeTourKey.value ? getTour(activeTourKey.value) : null));
  const accessReady = computed(() => activeTourKey.value
    ? contextReady.isReady(activeTourKey.value) : usePermissionsStore().loaded === true);

  const { steps, revision: stepsRevision, capture: captureStepSet, reset: resetStepSet } = createActiveStepSet({
    activeTour, tourContext, accessReady, isActive, currentIndex, currentStepId, skippedIndexes,
    saveProgress: next => { if (!partialRun.value) progress.save(activeTourKey.value, currentIndex.value, true, next); },
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




  /** Compatibility entry point; preparation now waits only for its operation. */
  async function ensureGatingContext() {
    const result = activeTourKey.value ? await prepareTour(activeTourKey.value) : await ensureSelectionContext();
    return result.status === 'ready';
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
   * Запустить тур с первого шага. Идемпотентно: повторный вызов при активном
   * туре ничего не делает. Незнакомый или ещё не написанный тур не запускается.
   *
   * @param {{ tour: string, manual?: boolean }} options
   * @returns {boolean} стартовал ли тур
   */

  function start({ tour, manual = false, restart = false, fromStepId } = {}) {
    if (isActive.value || contextReady.status(tour).status !== 'ready') return false;
    const entry = getTour(tour);
    if (!entry || !entry.steps.length) return false;
    if (!demo.canStart(tour)) return false;
    const requested = fromStepId === undefined ? null : applicableTourSteps(entry, tourContext.value).findIndex(step => step.id === fromStepId);
    if (requested !== null && (!manual || requested < 0)) return false;
    activeTourKey.value = entry.key;
    isManual.value = manual;
    partialRun.value = requested > 0;
    // Продолжаем с места, где человек остановился: обучение длинное, и начинать
    // заново после перерыва - значит бросить на середине. `restart` - явная просьба
    // из меню; досмотренный тур тоже начинаем с начала, продолжать в нём нечего, а
    // залежавшаяся запись уводила сразу на «Готово» (#2603). Клампим по ФАКТИЧЕСКОМУ
    // набору: часть шагов гейтится правами, и старая позиция ушла бы за последний шаг.
    const saved = requested ?? (restart || hasFinished(entry.key) ? 0 : progress.resumeIndex(entry.key, steps.value));
    resumedFrom.value = Math.min(saved, Math.max(0, steps.value.length - 1));
    currentIndex.value = resumedFrom.value;
    currentStepId.value = steps.value[currentIndex.value]?.id || null;
    captureStepSet();
    skippedIndexes.value = [];
    isActive.value = true;
    demo.start(entry.key, hasOwnApplication.value);
    return true;
  }

  /**
   * Тур для автозапуска: самый приоритетный из доступных и непройденных.
   *
   * @returns {object|null} запись реестра или null
   */
  /**
   * Тур для автозапуска - и только при ПЕРВОМ входе, пока человеку не показали
   * ни одного тура. Иначе получается «сыпет турами подряд»: завершил один, вернулся
   * на «Обзор», watch(route) снова зовёт автозапуск, а тот находит следующий
   * непройденный - и так по всем доступным. Остальные туры человек запускает сам
   * из меню «Обучение».
   *
   * @returns {object|null}
   */
  function pickAutostartTour() {
    if (hasSeenAnyTour()) return null;
    return pickAutostartTourFrom(tourContext.value, (key) => hasCompleted(key));
  }

  /**
   * Показывали ли человеку хоть один тур: любая запись прогресса, независимо от
   * версии и от того, досмотрел он до конца или закрыл на середине.
   *
   * @returns {boolean}
   */
  function hasSeenAnyTour() {
    return Object.values(completedByTour.value).some((v) => v !== null && v !== undefined);
  }

  function stop(finished = false) {
    // Закрыли осознанно - храним; после финала нет: stop идёт последним и вернул бы запись (#2600).
    if (activeTourKey.value && !finished && !partialRun.value) progress.save(activeTourKey.value, currentIndex.value, false, steps.value);
    isActive.value = false;
    demo.stop();
  }

  function setIndex(i) {
    currentIndex.value = i;
    currentStepId.value = steps.value[i]?.id || null;
    if (activeTourKey.value && !partialRun.value) progress.save(activeTourKey.value, i, true, steps.value);
  }

  /**
   * Перейти к следующему сегменту (на другой странице): сдвинуть глобальный
   * индекс на первый шаг следующего сегмента и поднять флаг ожидания навигации.
   */
  function advanceSegment() {
    currentIndex.value += 1;
    currentStepId.value = steps.value[currentIndex.value]?.id || null;
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
    currentStepId.value = steps.value[index]?.id || null;
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
  function markCompleted(finished = false) {
    const tour = activeTour.value;
    if (!tour || partialRun.value) return;
    // Идемпотентно по паре (версия, признак финала): повторный вызов при том же
    // исходе не шлёт второй POST, а вот «закрыл на середине» -> «досмотрел»
    // пройти обязан, иначе отметка о полном прохождении не запишется никогда.
    // Чистим ДО раннего выхода: у второго прохода он срабатывал раньше (#2590).
    if (finished) progress.clear(tour.key);
    if (hasCompleted(tour.key) && (!finished || hasFinished(tour.key))) return;
    completedByTour.value = { ...completedByTour.value, [tour.key]: tour.version };
    if (finished && !finishedTours.value.includes(tour.key)) {
      finishedTours.value = [...finishedTours.value, tour.key];
    }
    statusLoaded.value = true;
    markOnboardingComplete(tour.key, tour.version, finished).catch(() => {
      // best-effort: статус локально уже стоит, бэкенд догонит при следующем разе
    });
  }

  /**
   * Тур доведён до финального шага - для бейджа «Пройден» в меню обучения.
   *
   * @param {string} tourKey
   * @returns {boolean}
   */
  function hasFinished(tourKey = activeTourKey.value) {
    return finishedTours.value.includes(tourKey);
  }

  function reset() {
    demo.stop();
    // Clear the owner before Driver teardown can attempt a completion write.
    activeTourKey.value = null;
    partialRun.value = false;
    isActive.value = false;
    currentIndex.value = 0;
    currentStepId.value = null;
    resetStepSet();
    skippedIndexes.value = [];
    pendingSegment.value = false;
    demo.api.setDemoAttachment(null);
    revealOpen.value = null;
    // Сброс статуса при logout - следующий юзер на этом устройстве подтянет свой.
    resetStatus();
    // Роль в согласовании, route фактовой таблицы и наличие своей заявки - per-user.
    resetGatingData();
  }

  // Impersonation changes the actor without logout; token rotation does not.
  watch(() => useAuthStore().userId ?? useAuthStore().username, reset, { flush: 'sync' });

  return {
    ...demo.api,
    revealOpen,
    revealOpenRevision, setRevealOpen,
    isActive,
    currentIndex,
    currentStepId,
    stepsRevision,
    accessReady,
    ensureSelectionContext,
    prepareTour,
    contextIdentity: contextReady.identity,
    skippedIndexes,
    markSkipped,
    activeTourKey,
    activeTour,
    steps,
    totalSteps,
    currentStep,
    availableTours,
    tourContext,
    canShowTour,
    isManual,
    partialRun,
    stepsForTour: key => contextReady.status(key).status === 'ready' ? applicableTourSteps(getTour(key), tourContext.value) : [],
    pendingSegment,
    completedByTour,
    statusLoaded,
    approvalRole,
    approvalRoleLoaded,
    factTableRoute,
    factRouteLoaded,
    securityPostContext,
    accessibleContext, accessibleContextLoaded,
    ownApplicationLoaded,
    ensureApprovalRole,
    ensureGatingContext,
    ensureOwnApplication,
    hasOwnApplication,
    resumedFrom,
    hasProgress: progress.has,
    interruptedTour: progress.interrupted,
    ensureFactRoute,
    loadStatus,
    hasCompleted,
    hasFinished,
    hasSeenAnyTour,
    finishedTours,
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
