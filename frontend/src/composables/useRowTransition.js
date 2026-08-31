import { ref, computed } from 'vue';

/**
 * Имя перехода для списка, который умеет и живую вставку, и полную замену набора.
 *
 * Каскад появления строк рассчитан на приход одной-двух заявок в реальном
 * времени (#840). При смене фильтра меняется весь набор, и тот же каскад
 * прогоняет через прозрачность все строки разом - список мигает целиком.
 *
 * Пустое имя отключает переходы, поэтому на время замены набора каскад гасится
 * и возвращается сразу после отрисовки.
 *
 * @param {string} name имя перехода для живой вставки
 */
export function useRowTransition(name) {
  const replacing = ref(false);

  // Флаг снимается в следующем тике: TransitionGroup читает имя перехода
  // в момент вставки узлов, раньше нельзя.
  async function whileReplacing(work) {
    replacing.value = true;
    try {
      return await work();
    } finally {
      await Promise.resolve();
      replacing.value = false;
    }
  }

  return {
    transitionName: computed(() => (replacing.value ? '' : name)),
    whileReplacing,
  };
}
