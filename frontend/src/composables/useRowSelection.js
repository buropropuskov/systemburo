import { ref, computed } from 'vue';

/**
 * Ctrl/Shift-выделение строк таблицы проходной (машины/люди) для групповых
 * операций (#1194). Обычный клик по строке НЕ участвует в выделении - решает
 * caller (открывать деталь или обрабатывать modifier-клик через handleRowClick).
 *
 * @returns {{
 *   selectedIds: import('vue').Ref<number[]>,
 *   selectedCount: import('vue').ComputedRef<number>,
 *   anchorId: import('vue').Ref<number|string|null>,
 *   isSelected: (id: number|string) => boolean,
 *   toggle: (id: number|string) => void,
 *   selectRange: (id: number|string, orderedIds: Array<number|string>) => void,
 *   handleRowClick: (event: MouseEvent, id: number|string, orderedIds: Array<number|string>) => boolean,
 *   clearSelection: () => void,
 *   pruneSelection: (validIds: Array<number|string>) => void,
 * }}
 */
export function useRowSelection() {
  const selectedIds = ref([]);
  const anchorId = ref(null);

  function isSelected(id) {
    return selectedIds.value.includes(id);
  }

  function toggle(id) {
    const idx = selectedIds.value.indexOf(id);
    if (idx === -1) selectedIds.value.push(id);
    else selectedIds.value.splice(idx, 1);
    anchorId.value = id;
  }

  // orderedIds - id строк в ТЕКУЩЕМ отображаемом порядке (displayItems после
  // фильтра/сортировки), иначе диапазон посчитается по неверным соседям.
  function selectRange(id, orderedIds) {
    if (typeof window !== 'undefined' && window.getSelection) {
      window.getSelection().removeAllRanges();
    }
    if (anchorId.value == null || anchorId.value === id) {
      toggle(id);
      return;
    }
    const anchorIdx = orderedIds.indexOf(anchorId.value);
    const targetIdx = orderedIds.indexOf(id);
    if (anchorIdx === -1 || targetIdx === -1) {
      toggle(id);
      return;
    }
    const [from, to] = anchorIdx < targetIdx ? [anchorIdx, targetIdx] : [targetIdx, anchorIdx];
    // Направление диапазона - как у обычного toggle: если кликнутая строка
    // была не выбрана, весь диапазон выбираем, иначе весь диапазон снимаем.
    const shouldSelect = !isSelected(id);
    const next = new Set(selectedIds.value);
    for (let i = from; i <= to; i++) {
      const rowId = orderedIds[i];
      if (shouldSelect) next.add(rowId);
      else next.delete(rowId);
    }
    selectedIds.value = Array.from(next);
    anchorId.value = id;
  }

  // true - клик обработан как выделение, caller НЕ должен выполнять обычное
  // действие клика (открытие детали).
  function handleRowClick(event, id, orderedIds) {
    if (event.shiftKey) {
      selectRange(id, orderedIds);
      return true;
    }
    if (event.ctrlKey || event.metaKey) {
      toggle(id);
      return true;
    }
    return false;
  }

  function clearSelection() {
    selectedIds.value = [];
    anchorId.value = null;
  }

  // Выбранные строки, ушедшие из видимого списка (фильтр/поиск/поллинг),
  // убираем - иначе счётчик "Выбрано: N" врёт про невидимые строки.
  function pruneSelection(validIds) {
    if (!selectedIds.value.length) return;
    const valid = new Set(validIds);
    const pruned = selectedIds.value.filter(id => valid.has(id));
    if (pruned.length !== selectedIds.value.length) selectedIds.value = pruned;
  }

  const selectedCount = computed(() => selectedIds.value.length);

  return {
    selectedIds,
    selectedCount,
    anchorId,
    isSelected,
    toggle,
    selectRange,
    handleRowClick,
    clearSelection,
    pruneSelection,
  };
}
