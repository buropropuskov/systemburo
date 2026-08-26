import { ref } from 'vue';
import { getOnboardingStatus } from '@/api/onboarding';

/**
 * Что человек уже прошёл: версии пройденных туров и список доведённых до финала.
 *
 * Живёт отдельно от стора по той же причине, что и gatingData: это внешние данные
 * со своим кэшем и in-flight-промисом, а стору от них нужны три ссылки и один
 * метод. На ошибке сети `loaded` остаётся false - хост тогда не автозапускает тур,
 * и человек не получит обучение повторно из-за упавшего запроса.
 *
 * @returns {{
 *   completedByTour: import('vue').Ref<Record<string, number>>,
 *   finishedTours: import('vue').Ref<string[]>,
 *   loaded: import('vue').Ref<boolean>,
 *   load: () => Promise<void>,
 *   reset: () => void,
 * }}
 */
export function createTourStatus() {
  const completedByTour = ref({});
  // Ключи туров, доведённых до финального шага. Отдельно от completedByTour:
  // закрытый на середине тур тоже «пройден» в смысле автозапуска, но значка
  // «Пройден» в меню не заслуживает.
  const finishedTours = ref([]);
  const loaded = ref(false);
  let promise = null;

  async function load() {
    if (promise) return promise;
    promise = (async () => {
      try {
        const data = await getOnboardingStatus();
        completedByTour.value = { ...(data?.completed || {}) };
        finishedTours.value = Array.isArray(data?.finished) ? [...data.finished] : [];
        loaded.value = true;
      } catch {
        loaded.value = false;
      } finally {
        promise = null;
      }
    })();
    return promise;
  }

  /** Смена пользователя: следующий подтянет свой статус. */
  function reset() {
    completedByTour.value = {};
    finishedTours.value = [];
    loaded.value = false;
    promise = null;
  }

  return { completedByTour, finishedTours, loaded, load, reset };
}
