import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const getUnreadCount = vi.fn();
vi.mock('@/api/applications', () => ({
  getUnreadCount: (...a) => getUnreadCount(...a),
}));

const getFeedbackStats = vi.fn();
vi.mock('@/api/feedback', () => ({
  getFeedbackStats: (...a) => getFeedbackStats(...a),
}));

const playPreset = vi.fn();
vi.mock('@/utils/notificationSound', () => ({
  playPreset: (...a) => playPreset(...a),
}));

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...a) => apiRequest(...a),
}));

const hasPermission = vi.fn();
const fetchPermissions = vi.fn();
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission, fetchPermissions }),
}));

import NavMenu from '../NavMenu.vue';

function jsonResponse(body) {
  return { ok: true, json: async () => body };
}

async function mountNav() {
  apiRequest.mockImplementation((url) => {
    if (url === '/users/me') return Promise.resolve(jsonResponse({ data: { is_banned: false } }));
    if (url === '/system-tables') return Promise.resolve(jsonResponse([]));
    return Promise.resolve(jsonResponse({}));
  });
  fetchPermissions.mockResolvedValue(undefined);
  const wrapper = shallowMount(NavMenu, {
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $route: { path: '/personal-cabinet', params: {} },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
      },
    },
  });
  await flushPromises();
  return wrapper;
}

describe('NavMenu — бейдж = непрочитанные + обновления статуса (#1349)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUnreadCount.mockReset();
    getFeedbackStats.mockReset();
    playPreset.mockReset();
    apiRequest.mockReset();
    hasPermission.mockReset();
    fetchPermissions.mockReset();
    hasPermission.mockImplementation((key) => key === 'page.center');
    getUnreadCount.mockResolvedValue({ count: 0, status_updates: 0 });
  });

  it('бейдж показывает сумму count + status_updates', async () => {
    getUnreadCount.mockResolvedValue({ count: 2, status_updates: 3 });
    const wrapper = await mountNav();
    expect(wrapper.vm.newApplicationsCount).toBe(5);
    wrapper.unmount();
  });

  it('рост ТОЛЬКО обновлений статуса (count не растёт) НЕ играет звук', async () => {
    getUnreadCount.mockResolvedValue({ count: 2, status_updates: 0 });
    const wrapper = await mountNav();
    wrapper.vm.soundStore.setEnabled(true);
    expect(wrapper.vm.newApplicationsCount).toBe(2);
    // Обновления статуса выросли 0 -> 5, непрочитанные те же (2).
    getUnreadCount.mockResolvedValue({ count: 2, status_updates: 5 });
    await wrapper.vm.fetchNewApplicationsCount();
    expect(playPreset).not.toHaveBeenCalled();
    expect(wrapper.vm.newApplicationsCount).toBe(7);
    wrapper.unmount();
  });

  it('рост непрочитанных играет звук и бейдж включает обновления', async () => {
    getUnreadCount.mockResolvedValue({ count: 1, status_updates: 2 });
    const wrapper = await mountNav();
    wrapper.vm.soundStore.setEnabled(true);
    expect(wrapper.vm.newApplicationsCount).toBe(3);
    // Непрочитанные выросли 1 -> 4.
    getUnreadCount.mockResolvedValue({ count: 4, status_updates: 2 });
    await wrapper.vm.fetchNewApplicationsCount();
    expect(playPreset).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.newApplicationsCount).toBe(6);
    wrapper.unmount();
  });
});
