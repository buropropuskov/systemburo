import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// #840 V4: открытая деталь заявки подписывается на application:<id> и по сигналу
// application.updated перезапрашивает статус/согласующих/вопросы без F5.
const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...a) => apiRequest(...a) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue(undefined) }));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: vi.fn() }) }));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import ApplicationDetail from '../ApplicationDetail.vue';
import eventStream from '@/services/eventStream';

function mountDetail(app = { id: 7, application_number: 'A-7', status: 'В работе', sender_user_id: 1 }) {
  return shallowMount(ApplicationDetail, {
    props: { application: app, currentUserId: 1, mode: 'user' },
  });
}

describe('ApplicationDetail - real-time application.updated (#840 V4)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
    eventStream.connect.mockClear();
    eventStream.disconnect.mockClear();
    eventStream.subscribe.mockClear().mockImplementation(() => vi.fn());
  });

  it('при монтировании подключается и подписывается на application:<id>', async () => {
    mountDetail();
    await flushPromises();
    expect(eventStream.connect).toHaveBeenCalledTimes(1);
    expect(eventStream.subscribe).toHaveBeenCalledWith('application:7', expect.any(Function));
  });

  it('колбэк сигнала перезапрашивает деталь (loadApplicationDetails)', async () => {
    mountDetail();
    await flushPromises();
    const cb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'application:7')[1];
    apiRequest.mockClear();
    cb();
    await flushPromises();
    const paths = apiRequest.mock.calls.map((c) => c[0]);
    expect(paths).toContain('/applications/7/details');
  });

  it('при смене id переподписывается на новый scope, сняв старую подписку', async () => {
    const unsub = vi.fn();
    eventStream.subscribe.mockImplementation(() => unsub);
    const wrapper = mountDetail();
    await flushPromises();

    await wrapper.setProps({
      application: { id: 8, application_number: 'A-8', status: 'В работе', sender_user_id: 1 },
    });
    await flushPromises();

    expect(unsub).toHaveBeenCalled();
    expect(eventStream.subscribe).toHaveBeenCalledWith('application:8', expect.any(Function));
  });

  it('при unmount отписывается и отключает eventStream', async () => {
    const unsub = vi.fn();
    eventStream.subscribe.mockImplementation(() => unsub);
    const wrapper = mountDetail();
    await flushPromises();

    wrapper.unmount();
    expect(unsub).toHaveBeenCalledTimes(1);
    expect(eventStream.disconnect).toHaveBeenCalledTimes(1);
  });

  it('refreshLiveDetail рефетчит деталь с preserveSelection', async () => {
    const wrapper = mountDetail();
    await flushPromises();

    const spy = vi.spyOn(wrapper.vm, 'loadApplicationDetails');
    wrapper.vm.refreshLiveDetail();

    expect(spy).toHaveBeenCalledWith(wrapper.vm.applicationData, { preserveSelection: true });
  });

  it('live-рефетч (preserveSelection) сохраняет выбранное вложение, не сбрасывает на первое', async () => {
    apiRequest.mockImplementation((url) => {
      if (url.endsWith('/attachments')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([{ id: 10 }, { id: 20 }]) });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) });
    });

    const wrapper = mountDetail();
    await flushPromises();

    // Пользователь смотрит второе вложение.
    wrapper.vm.selectedAttachment = { id: 20 };
    await wrapper.vm.loadApplicationDetails(wrapper.vm.applicationData, { preserveSelection: true });
    await flushPromises();

    expect(wrapper.vm.selectedAttachment.id).toBe(20);
  });
});
