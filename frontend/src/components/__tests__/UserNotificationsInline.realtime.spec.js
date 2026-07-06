import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import { apiRequest } from '@/api/client';
import eventStream from '@/services/eventStream';
import UserNotificationsInline from '../UserNotificationsInline.vue';

function mountN() {
  setActivePinia(createPinia());
  return mount(UserNotificationsInline);
}

describe('UserNotificationsInline - real-time доставка через SSE (#840)', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
    eventStream.connect.mockClear();
    eventStream.disconnect.mockClear();
    eventStream.subscribe.mockClear();
    eventStream.subscribe.mockImplementation(() => vi.fn());
    eventStream.onStatus.mockClear();
    eventStream.onStatus.mockImplementation(() => vi.fn());
  });

  it('при mount подписывается на scope notifications и подключается', async () => {
    const wrapper = mountN();
    await flushPromises();

    expect(eventStream.connect).toHaveBeenCalledTimes(1);
    expect(eventStream.subscribe).toHaveBeenCalledWith('notifications', expect.any(Function));

    wrapper.unmount();
  });

  it('колбэк subscribe дёргает fetchNotifications', async () => {
    const wrapper = mountN();
    await flushPromises();

    const scopeCb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'notifications')[1];
    apiRequest.mockClear();
    scopeCb();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/notifications');

    wrapper.unmount();
  });

  it('тик поллинга не бьёт по сети при sseConnected=true, но бьёт при false', async () => {
    vi.useFakeTimers();
    const wrapper = mountN();
    await flushPromises();

    wrapper.vm.sseConnected = true;
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(30000);
    expect(apiRequest).not.toHaveBeenCalled();

    wrapper.vm.sseConnected = false;
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(30000);
    expect(apiRequest).toHaveBeenCalledWith('/notifications');

    wrapper.unmount();
    vi.useRealTimers();
  });

  it('при unmount отписывается и отключает eventStream', async () => {
    const unsubScope = vi.fn();
    const unsubStatus = vi.fn();
    eventStream.subscribe.mockImplementation(() => unsubScope);
    eventStream.onStatus.mockImplementation(() => unsubStatus);

    const wrapper = mountN();
    await flushPromises();
    wrapper.unmount();

    expect(unsubScope).toHaveBeenCalledTimes(1);
    expect(unsubStatus).toHaveBeenCalledTimes(1);
    expect(eventStream.disconnect).toHaveBeenCalledTimes(1);
  });
});
