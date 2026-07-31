import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// #840: NavMenu подписывается на feedback (бейдж обратной связи) и system-tables
// (список таблиц в меню) - мгновенное обновление без 30с/60с-опроса.
const getUnreadCount = vi.fn();
vi.mock('@/api/applications', () => ({ getUnreadCount: (...a) => getUnreadCount(...a) }));

const getFeedbackStats = vi.fn();
vi.mock('@/api/feedback', () => ({ getFeedbackStats: (...a) => getFeedbackStats(...a) }));

vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...a) => apiRequest(...a) }));

const hasPermission = vi.fn();
const fetchPermissions = vi.fn();
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission, fetchPermissions }),
}));

vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import NavMenu from '../NavMenu.vue';
import eventStream from '@/services/eventStream';

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

describe('NavMenu - real-time feedback + system-tables (#840)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUnreadCount.mockReset().mockResolvedValue({ count: 0 });
    getFeedbackStats.mockReset().mockResolvedValue({ unread: 0 });
    apiRequest.mockReset();
    hasPermission.mockReset().mockReturnValue(false);
    fetchPermissions.mockReset();
    eventStream.connect.mockClear();
    eventStream.disconnect.mockClear();
    eventStream.subscribe.mockClear().mockImplementation(() => vi.fn());
    eventStream.onStatus.mockClear().mockImplementation(() => vi.fn());
  });

  it('подписывается на scope feedback и system-tables при монтировании', async () => {
    const wrapper = await mountNav();
    expect(eventStream.subscribe).toHaveBeenCalledWith('feedback', expect.any(Function));
    expect(eventStream.subscribe).toHaveBeenCalledWith('system-tables', expect.any(Function));
    wrapper.unmount();
  });

  it('колбэк feedback дёргает fetchNewFeedbackCount, system-tables - fetchSystemTables', async () => {
    const wrapper = await mountNav();
    const feedbackCb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'feedback')[1];
    const tablesCb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'system-tables')[1];

    const fbSpy = vi.spyOn(wrapper.vm, 'fetchNewFeedbackCount');
    const stSpy = vi.spyOn(wrapper.vm, 'fetchSystemTables');
    feedbackCb();
    tablesCb();

    expect(fbSpy).toHaveBeenCalled();
    expect(stSpy).toHaveBeenCalled();
    wrapper.unmount();
  });

  it('onStatus ставит sseConnected; поллинг таблиц молчит при connected', async () => {
    vi.useFakeTimers();
    const wrapper = await mountNav();
    const statusCb = eventStream.onStatus.mock.calls[0][0];

    const calledSystemTables = () => apiRequest.mock.calls.some((c) => c[0] === '/system-tables');

    statusCb('connected');
    expect(wrapper.vm.sseConnected).toBe(true);
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(60000);
    expect(calledSystemTables()).toBe(false);

    statusCb('reconnecting');
    expect(wrapper.vm.sseConnected).toBe(false);
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(60000);
    expect(calledSystemTables()).toBe(true);

    wrapper.unmount();
    vi.useRealTimers();
  });

  it('при unmount отписывается от feedback/system-tables/статуса и отключает', async () => {
    const unsubFeedback = vi.fn();
    const unsubTables = vi.fn();
    const unsubStatus = vi.fn();
    eventStream.subscribe.mockImplementation((scope) => {
      if (scope === 'feedback') return unsubFeedback;
      if (scope === 'system-tables') return unsubTables;
      return vi.fn();
    });
    eventStream.onStatus.mockImplementation(() => unsubStatus);

    const wrapper = await mountNav();
    wrapper.unmount();

    expect(unsubFeedback).toHaveBeenCalledTimes(1);
    expect(unsubTables).toHaveBeenCalledTimes(1);
    expect(unsubStatus).toHaveBeenCalledTimes(1);
    expect(eventStream.disconnect).toHaveBeenCalled();
  });
});
