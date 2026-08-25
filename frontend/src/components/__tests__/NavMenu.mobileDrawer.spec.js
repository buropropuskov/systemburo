import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';

// #1097 S3: на touch-устройстве тап по бургеру синтезирует mouseenter на рельсе,
// как только drawer выезжает под пальцем - без гейта mobileOpen рельс переключался
// в desktop-режим "expanded" (248px) вместо мобильных 280px/85vw.

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
    },
  });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: false });
  getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
});

afterEach(() => {
  wrapper?.unmount();
});

describe('NavMenu: мобильный drawer не ловит desktop hover-разворот', () => {
  it('mouseenter на рельсе при открытом drawer не добавляет класс expanded', async () => {
    wrapper = mountNav();
    await flushPromises();

    await wrapper.setData({ mobileOpen: true });
    await wrapper.find('.nav-menu').trigger('mouseenter');

    expect(wrapper.find('.nav-menu').classes()).not.toContain('expanded');
    expect(wrapper.find('.nav-menu').classes()).toContain('nav-menu--mobile-open');
  });

  it('desktop-hover (drawer закрыт) по-прежнему разворачивает рельс', async () => {
    wrapper = mountNav();
    await flushPromises();

    await wrapper.find('.nav-menu').trigger('mouseenter');

    expect(wrapper.find('.nav-menu').classes()).toContain('expanded');
  });
});
