import { describe, it, expect, beforeEach } from 'vitest';
import { useEntitySelection } from '../useEntitySelection';

describe('useEntitySelection', () => {
  let selection;

  beforeEach(() => {
    selection = useEntitySelection();
  });

  describe('toggle', () => {
    it('adds item to tempSelected', () => {
      selection.toggle({ id: 1, name: 'A' });
      expect(selection.tempSelected.value).toHaveLength(1);
      expect(selection.tempSelected.value[0]).toEqual({ id: 1, name: 'A' });
    });

    it('removes item if already selected', () => {
      selection.toggle({ id: 1, name: 'A' });
      selection.toggle({ id: 1, name: 'A' });
      expect(selection.tempSelected.value).toHaveLength(0);
    });

    it('handles multiple items', () => {
      selection.toggle({ id: 1, name: 'A' });
      selection.toggle({ id: 2, name: 'B' });
      expect(selection.tempSelected.value).toHaveLength(2);
    });
  });

  describe('isSelected', () => {
    it('returns true for selected item', () => {
      selection.toggle({ id: 1, name: 'A' });
      expect(selection.isSelected({ id: 1 })).toBe(true);
    });

    it('returns false for unselected item', () => {
      expect(selection.isSelected({ id: 1 })).toBe(false);
    });
  });

  describe('confirm', () => {
    it('copies tempSelected to confirmed', () => {
      selection.toggle({ id: 1, name: 'A' });
      selection.toggle({ id: 2, name: 'B' });
      selection.confirm();
      expect(selection.confirmed.value).toHaveLength(2);
      expect(selection.confirmedCount.value).toBe(2);
    });

    it('confirmed is independent from tempSelected after confirm', () => {
      selection.toggle({ id: 1, name: 'A' });
      selection.confirm();
      selection.toggle({ id: 1, name: 'A' });
      expect(selection.tempSelected.value).toHaveLength(0);
      expect(selection.confirmed.value).toHaveLength(1);
    });
  });

  describe('syncFromConfirmed', () => {
    it('restores tempSelected from confirmed', () => {
      selection.toggle({ id: 1, name: 'A' });
      selection.confirm();
      selection.toggle({ id: 1, name: 'A' });
      expect(selection.tempSelected.value).toHaveLength(0);

      selection.syncFromConfirmed();
      expect(selection.tempSelected.value).toHaveLength(1);
    });
  });

  describe('reset', () => {
    it('clears both tempSelected and confirmed', () => {
      selection.toggle({ id: 1, name: 'A' });
      selection.confirm();
      selection.reset();

      expect(selection.tempSelected.value).toHaveLength(0);
      expect(selection.confirmed.value).toHaveLength(0);
    });
  });

  describe('count', () => {
    it('reflects tempSelected length', () => {
      expect(selection.count.value).toBe(0);
      selection.toggle({ id: 1, name: 'A' });
      expect(selection.count.value).toBe(1);
      selection.toggle({ id: 2, name: 'B' });
      expect(selection.count.value).toBe(2);
      selection.toggle({ id: 1, name: 'A' });
      expect(selection.count.value).toBe(1);
    });
  });

  describe('custom idKey', () => {
    it('uses custom key for identification', () => {
      const sel = useEntitySelection('uid');
      sel.toggle({ uid: 'abc', name: 'A' });
      expect(sel.isSelected({ uid: 'abc' })).toBe(true);
      expect(sel.isSelected({ uid: 'xyz' })).toBe(false);
      sel.toggle({ uid: 'abc', name: 'A' });
      expect(sel.tempSelected.value).toHaveLength(0);
    });
  });
});
