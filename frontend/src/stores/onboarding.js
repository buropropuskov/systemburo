import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { onboardingSteps, ONBOARDING_VERSION } from '@/components/onboarding/onboardingSteps';
import { securityOnboardingSteps } from '@/components/onboarding/securityOnboardingSteps';
import { getOnboardingStatus, markOnboardingComplete } from '@/api/onboarding';

/**
 * Стор онбординг-тура. Держит ГЛОБАЛЬНЫЙ индекс активного шага по всему
 * массиву onboardingSteps; хост-компонент режет массив на сегменты по route
 * и водит driver.js внутри текущего сегмента.
 *
 * Статус «пройден» хранится per-user на бэкенде (а не в localStorage), чтобы
 * переживал смену браузера/устройства и сбрасывался админом. completedVersion -
 * версия тура, которую прошёл юзер (null = не проходил).
 */
export const useOnboardingStore = defineStore('onboarding', () => {
  const isActive = ref(false);
  const currentIndex = ref(0);
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

  // Per-user статус с бэкенда: версия пройденного тура и флаг загрузки.
  const completedVersion = ref(null);
  const statusLoaded = ref(false);
  // In-flight промис загрузки статуса - чтобы конкурентные maybeAutostart
  // (onMounted + watch route) не слали два GET (урок про гонки авто-fetch).
  let loadStatusPromise = null;

  // Тур ветвится по типу пользователя: охранник смотрит вложения и таблицы, а не
  // подаёт заявки - ему отдельный сценарий. Версия и per-user флаг завершения общие
  // (юзер ровно одного типа). Автозапуск на /news сам покажет нужный сценарий.
  const steps = computed(() => {
    const auth = useAuthStore();
    return auth.isSecurity ? securityOnboardingSteps : onboardingSteps;
  });
  const totalSteps = computed(() => steps.value.length);
  const currentStep = computed(() => steps.value[currentIndex.value] || null);

  const canShowTour = computed(() => {
    const auth = useAuthStore();
    return auth.isAuthenticated;
  });

  /**
   * Подтянуть per-user статус с бэкенда (один раз за сессию). На ошибке сети
   * statusLoaded остаётся false - хост тогда не автозапускает тур (fail-safe),
   * кнопка «Обучение» по-прежнему работает.
   */
  async function loadStatus() {
    if (loadStatusPromise) return loadStatusPromise;
    loadStatusPromise = (async () => {
      try {
        const data = await getOnboardingStatus();
        completedVersion.value = data?.completed_version ?? null;
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
   * Тур пройден, если юзер прошёл версию не ниже текущей. Подъём
   * ONBOARDING_VERSION делает тур «непройденным» и показывает заново.
   *
   * @returns {boolean}
   */
  function hasCompleted() {
    return completedVersion.value !== null && completedVersion.value >= ONBOARDING_VERSION;
  }

  /**
   * Запустить тур с первого шага. Идемпотентно: повторный вызов при активном
   * туре ничего не делает.
   *
   * @param {{ manual?: boolean }} [options]
   */
  function start({ manual = false } = {}) {
    if (isActive.value) return;
    isManual.value = manual;
    currentIndex.value = 0;
    isActive.value = true;
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

  function clearPending() {
    pendingSegment.value = false;
  }

  /**
   * Пометить тур пройденным для текущего юзера. Локально обновляем
   * completedVersion сразу (чтобы автозапуск не сработал повторно в этой
   * сессии), запись на бэкенд - fire-and-forget: сбой сети не должен ломать
   * завершение тура (в худшем случае переиграется при следующем входе).
   */
  function markCompleted() {
    // Идемпотентно: разные пути закрытия (Esc/Готово/CTA) могут позвать дважды -
    // версию ставим и POST шлём только если ещё не помечено.
    if (completedVersion.value !== null && completedVersion.value >= ONBOARDING_VERSION) return;
    completedVersion.value = ONBOARDING_VERSION;
    statusLoaded.value = true;
    markOnboardingComplete(ONBOARDING_VERSION).catch(() => {
      // best-effort: статус локально уже стоит, бэкенд догонит при следующем разе
    });
  }

  function reset() {
    isActive.value = false;
    currentIndex.value = 0;
    pendingSegment.value = false;
    demoAttachmentType.value = null;
    // Сброс статуса при logout - следующий юзер на этом устройстве подтянет свой.
    completedVersion.value = null;
    statusLoaded.value = false;
  }

  return {
    demoAttachmentType,
    setDemoAttachment,
    isActive,
    currentIndex,
    steps,
    totalSteps,
    currentStep,
    canShowTour,
    isManual,
    pendingSegment,
    completedVersion,
    statusLoaded,
    loadStatus,
    hasCompleted,
    start,
    stop,
    setIndex,
    advanceSegment,
    retreatSegment,
    clearPending,
    markCompleted,
    reset,
  };
});
