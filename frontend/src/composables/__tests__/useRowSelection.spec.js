import { describe, it, expect, beforeEach } from 'vitest';
import { useRowSelection } from '../useRowSelection';

function clickEvent({ shiftKey = false, ctrlKey = false, metaKey = false } = {}) {
  return { shiftKey, ctrlKey, metaKey };
}

describe('useRowSelection', () => {
  let selection;
  const orderedIds = [1, 2, 3, 4, 5];

  beforeEach(() => {
    selection = useRowSelection();
  });

  describe('toggle', () => {
    it('добавляет id при первом вызове', () => {
      selection.toggle(2);
      expect(selection.selectedIds.value).toEqual([2]);
      expect(selection.isSelected(2)).toBe(true);
    });

    it('снимает выделение при повторном вызове', () => {
      selection.toggle(2);
      selection.toggle(2);
      expect(selection.selectedIds.value).toHaveLength(0);
    });

    it('обновляет якорь на последний тоггл', () => {
      selection.toggle(2);
      selection.toggle(4);
      expect(selection.anchorId.value).toBe(4);
    });
  });

  describe('selectRange (shift)', () => {
    it('без якоря ведёт себя как toggle', () => {
      selection.selectRange(3, orderedIds);
      expect(selection.selectedIds.value).toEqual([3]);
    });

    it('выделяет диапазон от якоря до цели (вперёд)', () => {
      selection.toggle(2); // якорь = 2
      selection.selectRange(4, orderedIds);
      expect(selection.selectedIds.value.sort()).toEqual([2, 3, 4]);
    });

    it('выделяет диапазон от якоря до цели (назад)', () => {
      selection.toggle(4); // якорь = 4
      selection.selectRange(2, orderedIds);
      expect(selection.selectedIds.value.sort()).toEqual([2, 3, 4]);
    });

    it('повторный shift на уже выбранной строке снимает весь диапазон', () => {
      selection.toggle(2);
      selection.selectRange(4, orderedIds); // выбрали 2,3,4
      selection.toggle(1); // якорь = 1, выбрано [2,3,4,1]
      selection.selectRange(4, orderedIds); // 4 уже выбрана -> диапазон 1..4 снимается
      expect(selection.selectedIds.value).toHaveLength(0);
    });

    it('id вне orderedIds деградирует до toggle', () => {
      selection.toggle(2);
      selection.selectRange(999, orderedIds);
      expect(selection.selectedIds.value.sort()).toEqual([2, 999]);
    });
  });

  describe('handleRowClick', () => {
    it('обычный клик (без модификаторов) не обрабатывается - возвращает false', () => {
      const handled = selection.handleRowClick(clickEvent(), 1, orderedIds);
      expect(handled).toBe(false);
      expect(selection.selectedIds.value).toHaveLength(0);
    });

    it('ctrl-клик тоглит строку и возвращает true', () => {
      const handled = selection.handleRowClick(clickEvent({ ctrlKey: true }), 2, orderedIds);
      expect(handled).toBe(true);
      expect(selection.isSelected(2)).toBe(true);
    });

    it('meta-клик (Mac Cmd) тоглит строку', () => {
      const handled = selection.handleRowClick(clickEvent({ metaKey: true }), 3, orderedIds);
      expect(handled).toBe(true);
      expect(selection.isSelected(3)).toBe(true);
    });

    it('shift-клик выделяет диапазон и возвращает true', () => {
      selection.handleRowClick(clickEvent({ ctrlKey: true }), 1, orderedIds);
      const handled = selection.handleRowClick(clickEvent({ shiftKey: true }), 3, orderedIds);
      expect(handled).toBe(true);
      expect(selection.selectedIds.value.sort()).toEqual([1, 2, 3]);
    });
  });

  describe('drag-select (startDrag/dragOver/endDrag)', () => {
    it('startDrag с ctrl вооружает drag (true), но НЕ выделяет строку и не поднимает isDragging до движения', () => {
      const handled = selection.startDrag(2, clickEvent({ ctrlKey: true }));
      expect(handled).toBe(true);
      expect(selection.isSelected(2)).toBe(false);
      expect(selection.isDragging.value).toBe(false);
    });

    it('startDrag без модификатора не вооружает drag и ничего не выбирает', () => {
      const handled = selection.startDrag(2, clickEvent());
      expect(handled).toBe(false);
      expect(selection.isDragging.value).toBe(false);
      expect(selection.selectedIds.value).toHaveLength(0);
    });

    it('регресс #1194: ctrl+клик без движения остаётся обычным toggle (mousedown не крадёт выделение)', () => {
      // gesture: mousedown(startDrag) -> mouseup(endDrag) -> click(handleRowClick)
      selection.startDrag(2, clickEvent({ ctrlKey: true }));
      selection.endDrag();
      const handled = selection.handleRowClick(clickEvent({ ctrlKey: true }), 2, orderedIds);
      expect(handled).toBe(true);
      expect(selection.isSelected(2)).toBe(true); // ВЫБРАНА, а не снята
    });

    it('первое движение (dragOver другой строки) активирует drag: добавляет строку старта и пройденную', () => {
      selection.startDrag(2, clickEvent({ ctrlKey: true }));
      selection.dragOver(3);
      expect(selection.isDragging.value).toBe(true);
      expect(selection.selectedIds.value.sort()).toEqual([2, 3]);
    });

    it('drag ДОБАВЛЯЕТ пройденные строки, не сбрасывая прежнее выделение', () => {
      selection.toggle(1); // прежнее выделение до drag
      selection.startDrag(2, clickEvent({ ctrlKey: true }));
      selection.dragOver(3);
      selection.dragOver(4);
      expect(selection.selectedIds.value.sort()).toEqual([1, 2, 3, 4]);
    });

    it('drag с metaKey (Mac Cmd) работает так же', () => {
      selection.startDrag(2, clickEvent({ metaKey: true }));
      selection.dragOver(3);
      expect(selection.selectedIds.value.sort()).toEqual([2, 3]);
    });

    it('drag не снимает уже выбранную строку при повторном проходе', () => {
      selection.startDrag(2, clickEvent({ ctrlKey: true }));
      selection.dragOver(3);
      selection.dragOver(3);
      expect(selection.selectedIds.value.sort()).toEqual([2, 3]);
    });

    it('dragOver до startDrag ничего не добавляет', () => {
      selection.dragOver(5);
      expect(selection.selectedIds.value).toHaveLength(0);
    });

    it('endDrag останавливает drag - последующий dragOver не добавляет', () => {
      selection.startDrag(2, clickEvent({ ctrlKey: true }));
      selection.dragOver(3);
      selection.endDrag();
      selection.dragOver(5);
      expect(selection.selectedIds.value.sort()).toEqual([2, 3]);
      expect(selection.isDragging.value).toBe(false);
    });

    it('хвостовой click после реального drag подавляется (не снимает добавленную строку старта)', () => {
      selection.startDrag(2, clickEvent({ ctrlKey: true }));
      selection.dragOver(3); // было движение -> dragMoved
      selection.endDrag();
      const handled = selection.handleRowClick(clickEvent({ ctrlKey: true }), 2, orderedIds);
      expect(handled).toBe(true); // клик потреблён
      expect(selection.isSelected(2)).toBe(true); // строка старта осталась выбранной
    });

    it('гард хвостового клика живёт один жест - следующий mousedown его сбрасывает', () => {
      selection.startDrag(2, clickEvent({ ctrlKey: true }));
      selection.dragOver(3);
      selection.endDrag();
      // новый жест: mousedown сбрасывает гард -> обычный ctrl-клик снова тоггл
      selection.startDrag(5, clickEvent({ ctrlKey: true }));
      selection.endDrag();
      const handled = selection.handleRowClick(clickEvent({ ctrlKey: true }), 5, orderedIds);
      expect(handled).toBe(true);
      expect(selection.isSelected(5)).toBe(true);
    });

    it('регресс: обычный ctrl-тоггл и shift-диапазон работают рядом с drag-API', () => {
      selection.toggle(1);
      selection.selectRange(3, orderedIds);
      expect(selection.selectedIds.value.sort()).toEqual([1, 2, 3]);
      expect(selection.isDragging.value).toBe(false);
    });
  });

  describe('clearSelection', () => {
    it('сбрасывает выделение и якорь', () => {
      selection.toggle(1);
      selection.toggle(2);
      selection.clearSelection();
      expect(selection.selectedIds.value).toHaveLength(0);
      expect(selection.anchorId.value).toBeNull();
    });
  });

  describe('pruneSelection', () => {
    it('убирает id, которых больше нет в валидном списке', () => {
      selection.toggle(1);
      selection.toggle(2);
      selection.toggle(3);
      selection.pruneSelection([1, 3]);
      expect(selection.selectedIds.value.sort()).toEqual([1, 3]);
    });

    it('не трогает selectedIds, если выделение пустое', () => {
      selection.pruneSelection([1, 2]);
      expect(selection.selectedIds.value).toHaveLength(0);
    });

    it('не создаёт новую ссылку, если ничего не изменилось', () => {
      selection.toggle(1);
      const before = selection.selectedIds.value;
      selection.pruneSelection([1, 2, 3]);
      expect(selection.selectedIds.value).toBe(before);
    });
  });

  describe('selectedCount', () => {
    it('отражает длину selectedIds', () => {
      expect(selection.selectedCount.value).toBe(0);
      selection.toggle(1);
      selection.toggle(2);
      expect(selection.selectedCount.value).toBe(2);
    });
  });
});
