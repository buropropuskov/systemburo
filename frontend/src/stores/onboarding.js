import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { onboardingSteps, ONBOARDING_VERSION } from '@/components/onboarding/onboardingSteps';

const STORAGE_KEY = 'onboarding-tour';

/**
 * Стор онбординг-тура. Держит ГЛОБАЛЬНЫЙ индекс активного шага по всему
 * массиву onboardingSteps; хост-компонент режет массив на сегменты по route
 * и водит driver.js внутри текущего сегмента.
 */
export const useOnboardingStore = defineStore('onboarding', () => {
  const isActive = ref(false);
  const currentIndex = ref(0);
  // Авто-запуск (срез 6) ставит флаг завершения даже при пропуске, ручной — нет.
  const isManual = ref(false);
  // Тур переходит между страницами: на границе сегмента driver уничтожается,
  // выполняется router.push, и pendingSegment сигналит хосту подхватить
  // следующий сегмент после навигации (router.afterEach).
  const pendingSegment = ref(false);

  const steps = computed(() => onboardingSteps);
  const totalSteps = computed(() => steps.value.length);
  const currentStep = computed(() => steps.value[currentIndex.value] || null);

  const canShowTour = computed(() => {
    const auth = useAuthStore();
    return auth.isAuthenticated;
  });

  /**
   * Тур пройден только если сохранённый флаг помечен completed И его версия
   * совпадает с текущей. Чтение в try/catch: приватный режим / отключённый
   * storage кидают при доступе.
   *
   * @returns {boolean}
   */
  function hasCompleted() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return false;
      const parsed = JSON.parse(raw);
      return parsed.completed === true && parsed.version === ONBOARDING_VERSION;
    } catch {
      return false;
    }
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

  function clearPending() {
    pendingSegment.value = false;
  }

  function markCompleted() {
    try {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({ completed: true, version: ONBOARDING_VERSION }),
      );
    } catch {
      // storage недоступен (приватный режим / квота) - тур всё равно отработал,
      // просто переиграется при следующем авто-запуске. Не критично.
    }
  }

  function reset() {
    isActive.value = false;
    currentIndex.value = 0;
    pendingSegment.value = false;
  }

  return {
    isActive,
    currentIndex,
    steps,
    totalSteps,
    currentStep,
    canShowTour,
    isManual,
    pendingSegment,
    hasCompleted,
    start,
    stop,
    setIndex,
    advanceSegment,
    clearPending,
    markCompleted,
    reset,
  };
});
