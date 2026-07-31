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

  it('не обнуляет счётчик при ошибке опроса и не играет ложный звук при восстановлении', async () => {
    hasPermission.mockImplementation((key) => key === 'page.center');
    getUnreadCount.mockResolvedValue({ count: 5 });
    const wrapper = await mountNav();
    wrapper.vm.soundStore.setEnabled(true);
    expect(wrapper.vm.newApplicationsCount).toBe(5);
    // Сетевой сбой опроса не должен обнулять базу сравнения.
    getUnreadCount.mockRejectedValueOnce(new Error('network'));
    await wrapper.vm.fetchNewApplicationsCount();
    expect(wrapper.vm.newApplicationsCount).toBe(5);
    // Следующий успешный опрос с тем же реальным значением - не рост - без ложного звука.
    getUnreadCount.mockResolvedValue({ count: 5 });
    await wrapper.vm.fetchNewApplicationsCount();
    expect(playPreset).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it('игнорирует устаревший конкурентный ответ и не играет ложный звук на мнимом росте', async () => {
    hasPermission.mockImplementation((key) => key === 'page.center');
    getUnreadCount.mockResolvedValue({ count: 5 });
    const wrapper = await mountNav();
    wrapper.vm.soundStore.setEnabled(true);
    // A - медленный вызов с УСТАРЕВШИМ снимком 5; B - быстрый с реально упавшим 4.
    let resolveA;
    getUnreadCount
      .mockImplementationOnce(() => new Promise((res) => { resolveA = () => res({ count: 5 }); }))
      .mockImplementationOnce(() => Promise.resolve({ count: 4 }));
    const pA = wrapper.vm.fetchNewApplicationsCount();
    const pB = wrapper.vm.fetchNewApplicationsCount();
    await pB;
    expect(wrapper.vm.newApplicationsCount).toBe(4);
    resolveA();
    await pA;
    // Устаревший A не затирает актуальное 4 и не играет звук на 5 > 4.
    expect(wrapper.vm.newApplicationsCount).toBe(4);
    expect(playPreset).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it('перезапрашивает счётчик по событию application-read (мгновенное гашение бейджа)', async () => {
    hasPermission.mockImplementation((key) => key === 'page.center');
    getUnreadCount.mockResolvedValue({ count: 9 });
    const wrapper = await mountNav();
    const onCall = wrapper.vm.$bus.on.mock.calls.find((c) => c[0] === 'application-read');
    expect(onCall).toBeTruthy();
    const readHandler = onCall[1];
    getUnreadCount.mockClear();
    getUnreadCount.mockResolvedValue({ count: 8 });
    readHandler();
    await flushPromises();
    expect(getUnreadCount).toHaveBeenCalled();
    expect(wrapper.vm.newApplicationsCount).toBe(8);
    wrapper.unmount();
  });
});
