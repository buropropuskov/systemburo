import { ref } from 'vue';
import { getUnreadCount } from '@/api/applications';

/**
 * Числа для кнопок «Новые» и «Обновления» в шапке Центра.
 *
 * Считает сервер по всему скоупу доступа, а не экран по загруженным строкам.
 * Иначе включённый фильтр обнулял соседний счётчик: «Обновления» отдают только
 * прочитанные заявки, непрочитанных в выборке не остаётся, подпись «Новые: 5»
 * укорачивается до «Новые», кнопка теряет ширину и соседняя съезжает. Заодно
 * уходит занижение счёта, когда непрочитанных больше, чем загружено порцией.
 *
 * Сбой счётчика не повод ронять список: числа просто остаются прежними.
 */
export function useHeaderCounters() {
  const unread = ref(0);
  const statusUpdates = ref(0);

  async function refresh() {
    try {
      const data = await getUnreadCount();
      unread.value = data?.count ?? 0;
      statusUpdates.value = data?.status_updates ?? 0;
    } catch {
      // числа остаются прежними
    }
  }

  return { unread, statusUpdates, refresh };
}
