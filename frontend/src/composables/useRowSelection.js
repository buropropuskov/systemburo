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
 *   isDragging: import('vue').Ref<boolean>,
 *   isSelected: (id: number|string) => boolean,
 *   toggle: (id: number|string) => void,
 *   selectRange: (id: number|string, orderedIds: Array<number|string>) => void,
 *   handleRowClick: (event: MouseEvent, id: number|string, orderedIds: Array<number|string>) => boolean,
 *   startDrag: (id: number|string, event: MouseEvent) => boolean,
 *   dragOver: (id: number|string) => void,
 *   endDrag: () => void,
 *   clearSelection: () => void,
 *   pruneSelection: (validIds: Array<number|string>) => void,
 * }}
 */
export function useRowSelection() {
  const selectedIds = ref([]);
  const anchorId = ref(null);
  const isDragging = ref(false);
  // Внутреннее состояние drag-select (#1227 P4). isDragging (реактивный, для
  // user-select CSS на время протяжки) поднимается ТОЛЬКО при реальном движении.
  // dragPending - drag "вооружён" на mousedown, но ещё не активен (одиночный
  // ctrl-клик остаётся обычным toggle). dragMoved - было движение, гард для
  // подавления хвостового @click (см. handleRowClick).
  let dragPending = false;
  let dragStartId = null;
  let dragMoved = false;

  function isSelected(id) {
    return selectedIds.value.includes(id);
  }

  function addToSelection(id) {
    if (!selectedIds.value.includes(id)) selectedIds.value.push(id);
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
    // Хвостовой @click завершившегося drag (mouseup на той же строке, что и
    // mousedown старта) - подавляем: иначе ctrl-toggle снял бы строку, которую
    // drag только что добавил. На каждый следующий mousedown флаг сбрасывается
    // (startDrag), так что легитимные клики не глотаются.
    if (dragMoved) {
      dragMoved = false;
      return true;
    }
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

  // Ctrl(Cmd)+зажать курсор на строке и вести (#1227 P4): пройденные мышью строки
  // ДОБАВЛЯЮТСЯ к выделению, никогда не снимаются (в отличие от toggle/selectRange).
  // startDrag ТОЛЬКО вооружает drag на mousedown - НЕ выделяет строку и не поднимает
  // isDragging сразу. Иначе ctrl+клик (тот же модификатор) конфликтует: mousedown
  // добавил бы строку, а следующий за ним @click ctrl-тогглом её снял. Реальное
  // выделение включается в dragOver при первом движении на ДРУГУЮ строку.
  // Возвращает false без модификатора - caller не начинает drag и не давит click.
  function startDrag(id, event) {
    dragMoved = false; // сброс хвостового-клик-гарда на каждом mousedown
    if (!event || !(event.ctrlKey || event.metaKey)) return false;
    dragPending = true;
    dragStartId = id;
    anchorId.value = id;
    return true;
  }

  function dragOver(id) {
    if (!dragPending || id === dragStartId) return;
    if (!dragMoved) {
      // первое реальное движение - активируем drag и включаем строку старта
      dragMoved = true;
      isDragging.value = true;
      addToSelection(dragStartId);
    }
    addToSelection(id);
  }

  function endDrag() {
    dragPending = false;
    isDragging.value = false;
    // dragMoved держим до хвостового @click (он его потребит) либо до следующего
    // mousedown (startDrag сбросит) - гард живёт ровно один трейлинг-клик.
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
    isDragging,
    isSelected,
    toggle,
    selectRange,
    handleRowClick,
    startDrag,
    dragOver,
    endDrag,
    clearSelection,
    pruneSelection,
  };
}
