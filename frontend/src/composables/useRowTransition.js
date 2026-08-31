import { ref, computed } from 'vue';
import { pinLeavingElement } from '@/utils/listTransition';

/**
 * Имя перехода для списка, который живёт в двух режимах.
 *
 * Живая вставка (#840): пришла новая заявка - она въезжает сверху, соседи
 * расступаются. Здесь уместен каскад появления.
 *
 * Смена фильтра: набор меняется целиком. Тот же каскад прогонял через
 * прозрачность все строки разом, и список мигал. Нужен другой рисунок -
 * отсеянные заявки уезжают влево, оставшиеся подтягиваются на освободившиеся
 * места, а те, что пришли в выборку, появляются без каскада.
 *
 * Поэтому не «включено/выключено», а два набора классов: `app-row` и
 * `app-row-filter`.
 *
 * @param {string} liveName переход для живой вставки
 * @param {string} replaceName переход для замены набора
 */
export function useRowTransition(liveName, replaceName) {
  const replacing = ref(false);

  // Имя читается в момент вставки узлов, поэтому флаг снимается тиком позже -
  // иначе Vue возьмёт живой набор классов на строках, которые едут по фильтру.
  // finally обязателен: на ошибке запроса режим остался бы включённым навсегда.
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
    transitionName: computed(() => (replacing.value ? replaceName : liveName)),
    whileReplacing,
    // Закрепление уходящих отдаётся отсюда же: у списка с двумя режимами перехода
    // оно нужно всегда, и отдельный импорт в каждом компоненте только множил бы связи.
    onBeforeLeave: pinLeavingElement,
  };
}
