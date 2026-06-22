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
        $router: { push: vi.fn() },
      },
    },
  });
  await flushPromises();
  return wrapper;
}

describe('NavMenu - звук новой заявки гейтится page.center', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUnreadCount.mockReset();
    getFeedbackStats.mockReset();
    playPreset.mockReset();
    apiRequest.mockReset();
    hasPermission.mockReset();
    fetchPermissions.mockReset();
    getUnreadCount.mockResolvedValue({ count: 0 });
  });

  it('без page.center не опрашивает счётчик и не играет звук', async () => {
    hasPermission.mockReturnValue(false);
    const wrapper = await mountNav();
    expect(getUnreadCount).not.toHaveBeenCalled();
    expect(playPreset).not.toHaveBeenCalled();
    expect(wrapper.vm.newApplicationsCount).toBe(0);
    wrapper.unmount();
  });

  it('с page.center опрашивает счётчик новых заявок', async () => {
    hasPermission.mockImplementation((key) => key === 'page.center');
    const wrapper = await mountNav();
    expect(getUnreadCount).toHaveBeenCalled();
    wrapper.unmount();
  });

  it('играет звук при росте счётчика только при наличии page.center', async () => {
    hasPermission.mockImplementation((key) => key === 'page.center');
    const wrapper = await mountNav();
    // Первый опрос спраймил soundPrimed; эмулируем рост 0 -> 2.
    getUnreadCount.mockResolvedValue({ count: 2 });
    wrapper.vm.soundStore.setEnabled(true);
    await wrapper.vm.fetchNewApplicationsCount();
    expect(playPreset).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.newApplicationsCount).toBe(2);
    wrapper.unmount();
  });
});
