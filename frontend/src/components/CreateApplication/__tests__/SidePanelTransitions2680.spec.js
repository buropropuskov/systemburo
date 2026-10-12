import { describe, expect, it, vi } from 'vitest';
import vehiclePanels from '../vehicleSidePanels';
import employeePanels from '../employeeSidePanels';
describe('stale side-panel transitions', () => {
  it('leaving unloading does not reset a newly selected passage panel', () => {
    const ctx = { selectedSidePanel: 'table', isMainShifted: true, selectedUnloadPlace: {} };
    vehiclePanels.methods.onPlaceLeave.call(ctx);
    expect(ctx.isMainShifted).toBe(true);
    expect(ctx.selectedUnloadPlace).toBeNull();
  });
  it('leaving passage does not reset newly selected unloading', () => {
    const ctx = { selectedSidePanel: 'place', isMainShifted: true, selectedTable: {} };
    vehiclePanels.methods.onTableLeave.call(ctx);
    expect(ctx.isMainShifted).toBe(true);
    expect(ctx.selectedTable).toBeNull();
  });
  it('a prior leave cannot clear a reopened panel of the same type', () => {
    const ctx = { selectedSidePanel: 'place', isMainShifted: true, selectedUnloadPlace: { id: 1 } };
    vehiclePanels.methods.onPlaceLeave.call(ctx);
    expect(ctx.selectedUnloadPlace).toEqual({ id: 1 });
  });
  it('closing a person panel cancels its pending opening', () => {
    vi.useFakeTimers();
    try {
      const ctx = { showPlaceModal: false, shiftTimer: setTimeout(() => { ctx.showPlaceModal = true; }, 300) };
      employeePanels.methods.closeTableDetails.call(ctx);
      vi.advanceTimersByTime(300);
      expect(ctx.showPlaceModal).toBe(false);
      expect(ctx.shiftTimer).toBeNull();
    } finally { vi.useRealTimers(); }
  });
});
