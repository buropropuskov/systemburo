import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// #1097 W3.9: на мобилке ApplicationDetail - bottom-sheet со свайп-вниз-закрытием
// (переиспользует useSwipeDismiss). Протягивание за порог должно эмитить close.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
}));

import { usePermissionsStore } from '@/stores/permissions';
import ApplicationDetail from '../ApplicationDetail.vue';

function mountDetail(appOverrides = {}, mode = 'center') {
  return shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'Непрочитано', ...appOverrides },
      currentUserId: 1,
      mode,
    },
  });
}

/** Жест: старт -> протяжка на dy пикселей вниз -> отпускание. */
function swipe(vm, dy) {
  vm.onSheetTouchStart({ touches: [{ clientY: 100 }], target: { closest: () => null } });
  vm.onSheetTouchMove({ touches: [{ clientY: 100 + dy }], cancelable: false, preventDefault: () => {} });
  vm.onSheetTouchEnd();
}

describe('ApplicationDetail - свайп-вниз-закрытие (bottom-sheet, #1097 W3.9)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('протягивание за порог -> emit("close")', () => {
    vi.useFakeTimers();
    const wrapper = mountDetail();
    // getScrollTop у sheetScroll в jsdom = 0 -> жест активен даже без ползунка.
    swipe(wrapper.vm, 200);
    // Закрытие после слайда-вниз (setTimeout ~260мс в useSwipeDismiss).
    vi.advanceTimersByTime(300);
    expect(wrapper.emitted('close')).toBeTruthy();
    vi.useRealTimers();
  });

  it('протягивание НЕ за порог -> close не эмитится', () => {
    const wrapper = mountDetail();
    swipe(wrapper.vm, 30);
    expect(wrapper.emitted('close')).toBeFalsy();
  });

  it('в процессе протягивания лист сдвинут (sheetOffset), отпускание сбрасывает', () => {
    const wrapper = mountDetail();
    wrapper.vm.onSheetTouchStart({ touches: [{ clientY: 100 }], target: { closest: () => null } });
    wrapper.vm.onSheetTouchMove({ touches: [{ clientY: 150 }], cancelable: false, preventDefault: () => {} });
    expect(wrapper.vm.sheetOffset).toBe(50);
    expect(wrapper.vm.sheetDragging).toBe(true);
    wrapper.vm.onSheetTouchEnd();
    expect(wrapper.vm.sheetOffset).toBe(0);
    expect(wrapper.vm.sheetDragging).toBe(false);
  });
});

// #1097 W3.10: кнопку «Открыть в окне» убрали - сообщение открывается тапом по превью.
describe('ApplicationDetail - открытие сообщения тапом по превью (#1097 W3.10)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('тап по превью открывает модалку сообщения', () => {
    const wrapper = mountDetail();
    expect(wrapper.vm.showMessageModal).toBe(false);
    wrapper.vm.openMessageFromPreview({ target: { closest: () => null } });
    expect(wrapper.vm.showMessageModal).toBe(true);
  });

  it('клик по ссылке внутри сообщения НЕ открывает модалку (даём перейти по ссылке)', () => {
    const wrapper = mountDetail();
    wrapper.vm.openMessageFromPreview({ target: { closest: (sel) => (sel === 'a' ? { tagName: 'A' } : null) } });
    expect(wrapper.vm.showMessageModal).toBe(false);
  });
});

// #1097 W3.8: кнопку "Скачать" убрали из строки списка на мобилке, перенесли в деталь.
describe('ApplicationDetail - кнопка "Скачать" в детали (#1097 W3.8)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  const btn = (w) => w.find('[data-testid="app-detail-button-download"]');

  it('есть при has_blank_template (mode=user - без права экспорта) и эмитит download с заявкой', async () => {
    const wrapper = mountDetail({ has_blank_template: true }, 'user');
    expect(btn(wrapper).exists()).toBe(true);
    await btn(wrapper).trigger('click');
    expect(wrapper.emitted('download')).toBeTruthy();
    expect(wrapper.emitted('download')[0][0].id).toBe(7);
  });

  it('нет без has_blank_template', () => {
    const wrapper = mountDetail({ has_blank_template: false }, 'user');
    expect(btn(wrapper).exists()).toBe(false);
  });

  it('в mode=center нужен action.export.applications: с правом - есть', () => {
    usePermissionsStore().effective = { 'action.export.applications': { value: 'allow', source: 'role' } };
    const wrapper = mountDetail({ has_blank_template: true }, 'center');
    expect(btn(wrapper).exists()).toBe(true);
  });

  it('в mode=center без action.export.applications - кнопки нет (как в строке Центра)', () => {
    const wrapper = mountDetail({ has_blank_template: true }, 'center');
    expect(btn(wrapper).exists()).toBe(false);
  });
});
