import { defineStore } from 'pinia';
import { ref } from 'vue';
import { apiRequest } from '@/api/client';

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
    durationsLoaded = true;
    try {
      const res = await apiRequest('/settings/notifications');
      const data = await res.json();
      if (data && Number(data.delete_duration) > 0) deleteDuration.value = Number(data.delete_duration) * 1000;
      if (data && Number(data.restore_duration) > 0) restoreDuration.value = Number(data.restore_duration) * 1000;
    } catch {
      durationsLoaded = false;
    }
  }

  function enqueue({ prefix = '', bold = '', suffix = '', onConfirm, onUndo, showUndo = true, duration }) {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    const ms = duration || deleteDuration.value;
    items.value.push({ id, prefix, bold, suffix, progress: 100, showUndo });
    callbacks.set(id, { onConfirm, onUndo });
    const step = 100 / (Math.max(ms, TICK_MS) / TICK_MS);
    const timer = setInterval(() => tick(id, step), TICK_MS);
    timers.set(id, timer);
    return id;
  }

  // Информационное уведомление в том же стиле (без отмены), напр. о восстановлении.
  function notify({ prefix = '', bold = '', suffix = '', duration }) {
    return enqueue({ prefix, bold, suffix, showUndo: false, duration: duration || restoreDuration.value });
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

  return { items, enqueue, notify, undo, loadDurations };
});
