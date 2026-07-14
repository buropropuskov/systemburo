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
