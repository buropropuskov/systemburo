import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// #1097 W3.6: окно истории на мобилке - bottom-sheet со свайп-вниз-закрытием
// (переиспользует useSwipeDismiss через setup() + мост requestClose в mounted()).
// Протягивание за порог должно закрывать окно (showModal -> false).

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));

import ApplicationHistory from '../ApplicationHistory.vue';

function mountHistory() {
  return shallowMount(ApplicationHistory, {
    props: { applicationId: 1, applicationNumber: 'A-1' },
  });
}

/** Жест: старт -> протяжка на dy пикселей вниз -> отпускание. */
function swipe(vm, dy) {
  vm.onSheetTouchStart({ touches: [{ clientY: 100 }], target: { closest: () => null } });
  vm.onSheetTouchMove({ touches: [{ clientY: 100 + dy }], cancelable: false, preventDefault: () => {} });
  vm.onSheetTouchEnd();
}

describe('ApplicationHistory - свайп-вниз-закрытие (bottom-sheet, #1097 W3.6)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('свайп за порог -> окно истории закрывается (showModal=false)', async () => {
    vi.useFakeTimers();
    const wrapper = mountHistory();
    wrapper.vm.openModal();
    await flushPromises();
    expect(wrapper.vm.showModal).toBe(true);
    swipe(wrapper.vm, 200);
    // Закрытие после слайда-вниз (setTimeout ~260мс в useSwipeDismiss).
    vi.advanceTimersByTime(300);
    await flushPromises();
    expect(wrapper.vm.showModal).toBe(false);
    vi.useRealTimers();
  });

  it('свайп НЕ за порог -> окно остаётся открытым', async () => {
    const wrapper = mountHistory();
    wrapper.vm.openModal();
    await flushPromises();
    swipe(wrapper.vm, 30);
    expect(wrapper.vm.showModal).toBe(true);
  });

  it('в процессе протягивания лист сдвинут (sheetOffset), отпускание сбрасывает', () => {
    const wrapper = mountHistory();
    wrapper.vm.onSheetTouchStart({ touches: [{ clientY: 100 }], target: { closest: () => null } });
    wrapper.vm.onSheetTouchMove({ touches: [{ clientY: 150 }], cancelable: false, preventDefault: () => {} });
    expect(wrapper.vm.sheetOffset).toBe(50);
    expect(wrapper.vm.sheetDragging).toBe(true);
    wrapper.vm.onSheetTouchEnd();
    expect(wrapper.vm.sheetOffset).toBe(0);
    expect(wrapper.vm.sheetDragging).toBe(false);
  });
});
