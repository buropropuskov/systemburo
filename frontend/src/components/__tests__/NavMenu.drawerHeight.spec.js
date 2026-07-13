import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';

// #1097 W3 срез 2: высота drawer'а тянется из visualViewport.height (CSS dvh лагает
// в Яндекс-браузере). Слушатели visualViewport вешаются на открытии и обязаны сниматься
// на закрытии - иначе утечка + залипшая --nav-drawer-h. Проверяем симметрию.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({ getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }) }));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

function mountNav() {
  return mount(NavMenu, {
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn() },
        $route: { path: '/news', params: {} },
      },
    },
  });
}

let wrapper;
let vv;
let origVv;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: false });
  getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
  origVv = window.visualViewport;
  vv = { height: 640, addEventListener: vi.fn(), removeEventListener: vi.fn() };
  Object.defineProperty(window, 'visualViewport', { configurable: true, value: vv });
});

afterEach(() => {
  wrapper?.unmount();
  Object.defineProperty(window, 'visualViewport', { configurable: true, value: origVv });
  document.documentElement.style.removeProperty('--nav-drawer-h');
});

describe('NavMenu: высота drawer через visualViewport (#1097 W3)', () => {
  it('открытие ставит --nav-drawer-h и вешает слушатели resize+scroll', async () => {
    wrapper = mountNav();
    await flushPromises();

    wrapper.vm.toggleMobile(); // open
    expect(wrapper.vm.mobileOpen).toBe(true);
    expect(document.documentElement.style.getPropertyValue('--nav-drawer-h')).toBe('640px');
    const events = vv.addEventListener.mock.calls.map((c) => c[0]);
    expect(events).toContain('resize');
    expect(events).toContain('scroll');
  });

  it('закрытие снимает слушатели и чистит переменную', async () => {
    wrapper = mountNav();
    await flushPromises();

    wrapper.vm.toggleMobile(); // open
    wrapper.vm.toggleMobile(); // close
    expect(wrapper.vm.mobileOpen).toBe(false);
    expect(document.documentElement.style.getPropertyValue('--nav-drawer-h')).toBe('');
    const removed = vv.removeEventListener.mock.calls.map((c) => c[0]);
    expect(removed).toContain('resize');
    expect(removed).toContain('scroll');
  });

  it('нет утечки: add и remove симметричны после нескольких open/close', async () => {
    wrapper = mountNav();
    await flushPromises();

    for (let i = 0; i < 3; i++) {
      wrapper.vm.toggleMobile(); // open
      wrapper.vm.toggleMobile(); // close
    }
    expect(vv.addEventListener.mock.calls.length).toBe(vv.removeEventListener.mock.calls.length);
  });
});
