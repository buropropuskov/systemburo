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
import { getOnboardingStatus, markOnboardingComplete } from '@/api/onboarding';
import { createGatingData } from '@/components/onboarding/gatingData';
import { syncDemoBackend } from '@/components/onboarding/demoBackend';

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
  const revealOpen = ref(null);
  function setRevealOpen(target) {
    revealOpen.value = target || null;
  }

  // Пройденные версии по турам: { [tourKey]: number|null }. Загружается одним
  // GET /onboarding; null/отсутствие ключа = тур не проходили.
  const completedByTour = ref({});
  // Ключи туров, доведённых до финального шага. Отдельно от completedByTour:
  // тот гасит автозапуск фактом показа, а «Пройден» в меню заслуживает только
  // досмотренный до конца - иначе пропуск врал бы, что человек всё видел.
  const finishedTours = ref([]);
  const statusLoaded = ref(false);
  // In-flight промис загрузки статуса - чтобы конкурентные maybeAutostart
  // (onMounted + watch route) не слали два GET (урок про гонки авто-fetch).
  let loadStatusPromise = null;

  // Роль в согласовании заявок (принимающий/согласующий) - гейт туров accept и
  // approve. Правами не определяется: роль задаётся записью в справочнике.

  // Route фактовой таблицы для шага отметки въезда/выезда в туре охраны.
  // Роль в согласовании, маршрут фактовой таблицы и наличие своей заявки живут в
  // gatingData.js: от них зависит СОСТАВ тура, поэтому они резолвятся до старта.
  const {
    approvalRole, approvalRoleLoaded, factTableRoute, hasOwnApplication,
    ensureApprovalRole, ensureFactRoute, ensureOwnApplication, reset: resetGatingData,
  } = createGatingData();

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
      hasOwnApplication: hasOwnApplication.value,
    };
  });

  const activeTour = computed(() => (activeTourKey.value ? getTour(activeTourKey.value) : null));

  /**
   * Шаги активного тура. Сегмент фактовой таблицы охранника добавляется в ХВОСТ
   * (buildSteps), когда route резолвлен - так индексы ранних шагов не сдвигаются,
   * даже если route доезжает уже после старта тура.
   *
   * Шаги с `requires` выбрасываются, если права нет - иначе человек упирался бы в
   * ожидание цели. Фильтр здесь, а не в хосте: так шаг не попадает ни в навигацию,
   * ни в счётчик «Шаг N из M».
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
        finishedTours.value = Array.isArray(data?.finished) ? [...data.finished] : [];
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
      // Сегмент фактовой таблицы добавляется в хвост шагов охраны, когда route
      // резолвится. Если ждать этого уже во время тура, счётчик прыгает: «Шаг 1
      // из 14» превращается в «из 20». Резолвим заранее, вместе с правами.
      ensureFactRoute(),
      // По той же причине, что и route фактовой таблицы: шаги про карточку
      // заявки должны быть решены ДО старта, иначе счётчик тает на ходу.
      ensureOwnApplication(),
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
    skippedIndexes.value = [];
    isActive.value = true;
    syncDemoBackend(true, hasOwnApplication.value);
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

  function stop() {
    isActive.value = false;
    syncDemoBackend(false);
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
  function markCompleted(finished = false) {
    const tour = activeTour.value;
    if (!tour) return;
    // Идемпотентно по паре (версия, признак финала): повторный вызов при том же
    // исходе не шлёт второй POST, а вот «закрыл на середине» -> «досмотрел»
    // пройти обязан, иначе отметка о полном прохождении не запишется никогда.
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
    isActive.value = false;
    currentIndex.value = 0;
    skippedIndexes.value = [];
    activeTourKey.value = null;
    pendingSegment.value = false;
    demoAttachmentType.value = null;
    revealOpen.value = null;
    // Сброс статуса при logout - следующий юзер на этом устройстве подтянет свой.
    completedByTour.value = {};
    finishedTours.value = [];
    statusLoaded.value = false;
    // Роль в согласовании, route фактовой таблицы и наличие своей заявки - per-user.
    resetGatingData();
    syncDemoBackend(false);
  }

  return {
    demoAttachmentType,
    setDemoAttachment,
    revealOpen,
    setRevealOpen,
    isActive,
    currentIndex,
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
    pendingSegment,
    completedByTour,
    statusLoaded,
    approvalRole,
    approvalRoleLoaded,
    factTableRoute,
    ensureApprovalRole,
    ensureGatingContext,
    ensureOwnApplication,
    hasOwnApplication,
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
