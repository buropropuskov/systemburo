import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';

// #1097 W3.3: «Сообщить о проблеме» переехало из шапки в бургер-drawer на мобилке.
// Кнопка гейтится правом header.report_problem и виднa только при открытом drawer;
// тап закрывает drawer и (после его анимации ухода) открывает модалку обратной связи.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }),
}));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

function mountNav() {
  return mount(NavMenu, {
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/news', params: {} },
      },
      stubs: { FeedbackModal: true },
    },
  });
}

const FEEDBACK = '[data-testid="header-button-feedback"]';
let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
});

afterEach(() => {
  wrapper?.unmount();
  vi.useRealTimers();
});

describe('NavMenu: кнопка «Сообщить о проблеме» в drawer (W3.3)', () => {
  it('видна в открытом drawer при праве header.report_problem', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
    wrapper = mountNav();
    await flushPromises();

    // drawer закрыт -> кнопки нет
    expect(wrapper.find(FEEDBACK).exists()).toBe(false);

    await wrapper.setData({ mobileOpen: true });
    expect(wrapper.find(FEEDBACK).exists()).toBe(true);
    expect(wrapper.find(FEEDBACK).text()).toContain('Сообщить о проблеме');
  });

  it('скрыта без права header.report_problem даже при открытом drawer', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue({ mode: 'user', permissions: [], denied: [] });
    wrapper = mountNav();
    await flushPromises();

    await wrapper.setData({ mobileOpen: true });
    expect(wrapper.find(FEEDBACK).exists()).toBe(false);
  });

  it('тап закрывает drawer и после анимации открывает модалку обратной связи', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
    wrapper = mountNav();
    await flushPromises();
    await wrapper.setData({ mobileOpen: true });

    vi.useFakeTimers();
    await wrapper.find(FEEDBACK).trigger('click');

    // drawer закрывается сразу, модалка ждёт конца анимации ухода
    expect(wrapper.vm.mobileOpen).toBe(false);
    expect(wrapper.vm.showFeedbackModal).toBe(false);

    await vi.advanceTimersByTimeAsync(300);
    expect(wrapper.vm.showFeedbackModal).toBe(true);
  });
});
