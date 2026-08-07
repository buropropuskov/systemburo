import { defineStore } from 'pinia';
import { ref } from 'vue';
import { apiRequest } from '@/api/client';
import { useUiStore } from '@/stores/ui';

/**
 * Стек уведомлений об удалении с отменой (#186).
 * Таймер и колбэки живут в сторе (вне жизненного цикла компонентов), поэтому
 * удаление доводится до конца даже при переходе на другую вкладку, а уведомления
 * накапливаются и не перекрывают друг друга.
 */

const timers = new Map();
const callbacks = new Map();
const TICK_MS = 100;

export const useDeletionsStore = defineStore('deletions', () => {
  const items = ref([]);
  // Длительности (мс), настраиваются в Админка->Настройки.
  const deleteDuration = ref(10000);
  const restoreDuration = ref(5000);
  let durationsLoaded = false;

  async function loadDurations() {
    if (durationsLoaded) return;
    try {
      const res = await apiRequest('/settings/notifications');
      const data = await res.json();
      const del = Number(data?.delete_duration);
      const rest = Number(data?.restore_duration);
      // Защёлкиваем только при успешном ответе с валидными значениями. До
      // авторизации запрос отдаёт 401 с JSON-телом (без throw), и значения
      // остаются дефолтными - в этом случае не латчим, чтобы перечитать после логина.
      if (del > 0 || rest > 0) {
        if (del > 0) deleteDuration.value = del * 1000;
        if (rest > 0) restoreDuration.value = rest * 1000;
        durationsLoaded = true;
      }
    } catch {
      // оставляем durationsLoaded = false для повторной попытки
    }
  }

  /**
   * Принудительно выставить длительности (в секундах).
   * Вызывается при сохранении настроек, чтобы изменение применилось сразу,
   * без перезагрузки страницы.
   * @param {number} deleteSec длительность плашки удаления, сек
   * @param {number} restoreSec длительность плашки восстановления, сек
   */
  function setDurations(deleteSec, restoreSec) {
    const del = Number(deleteSec);
    const rest = Number(restoreSec);
    if (del > 0) deleteDuration.value = del * 1000;
    if (rest > 0) restoreDuration.value = rest * 1000;
    durationsLoaded = true;
  }

  const DEFAULT_TITLES = { success: 'Успешно', error: 'Ошибка', warning: 'Внимание', info: 'Уведомление' };

  function enqueue({ prefix = '', bold = '', suffix = '', onConfirm, onUndo, showUndo = true, duration, type = 'success', title }) {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    const ms = duration || deleteDuration.value;
    const resolvedTitle = title === '' ? '' : title || DEFAULT_TITLES[type] || '';
    items.value.push({ id, title: resolvedTitle, prefix, bold, suffix, progress: 100, showUndo, type });
    callbacks.set(id, { onConfirm, onUndo });
    const step = 100 / (Math.max(ms, TICK_MS) / TICK_MS);
    const timer = setInterval(() => tick(id, step), TICK_MS);
    timers.set(id, timer);
    return id;
  }

  // Информационное уведомление в том же стиле (без отмены), напр. о восстановлении.
  // type: 'success' (по умолчанию, зелёный -> красный прогресс), 'error' (красный),
  // 'warning' (янтарный, для частичных/bulk-итогов) или 'info' (синий, нейтральный).
  // title: явный заголовок (дефолт от type из DEFAULT_TITLES, пустая строка = без заголовка).
  function notify({ prefix = '', bold = '', suffix = '', duration, type = 'success', title }) {
    // Во время онбординга фоновые подсказки («Место разгрузки выбрано
    // автоматически...») наезжают на поповер и сбивают с шага. Ошибки пропускаем
    // всегда: молча проглоченный отказ оставит человека гадать, почему не вышло.
    if (useUiStore().tourActive && type !== 'error') return null;
    return enqueue({ prefix, bold, suffix, showUndo: false, duration: duration || restoreDuration.value, type, title });
  }

  function tick(id, step) {
    const it = items.value.find(i => i.id === id);
    if (!it) {
      stopTimer(id);
      return;
    }
    it.progress = Math.max(0, it.progress - step);
    if (it.progress <= 0) confirm(id);
  }

  function confirm(id) {
    stopTimer(id);
    const cb = callbacks.get(id);
    remove(id);
    if (cb && cb.onConfirm) cb.onConfirm();
  }

  function undo(id) {
    stopTimer(id);
    const cb = callbacks.get(id);
    remove(id);
    if (cb && cb.onUndo) cb.onUndo();
  }

  function stopTimer(id) {
    const t = timers.get(id);
    if (t) {
      clearInterval(t);
      timers.delete(id);
    }
  }

  function remove(id) {
    items.value = items.value.filter(i => i.id !== id);
    callbacks.delete(id);
  }

  /**
   * Закрыть уведомление кликом по карточке = финализировать (как по истечении таймера):
   * для отложенного удаления с undo выполняет onConfirm (иначе delete потерялся бы -
   * запись не удалена, а toast пропал), для обычного notify (без onConfirm) просто закрывает.
   * Отмена (onUndo) - отдельная кнопка "Отменить".
   */
  function dismiss(id) {
    confirm(id);
  }

  return { items, enqueue, notify, undo, dismiss, loadDurations, setDurations };
});
