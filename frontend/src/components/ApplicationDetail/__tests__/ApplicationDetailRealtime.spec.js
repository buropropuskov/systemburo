import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount, flushPromises, enableAutoUnmount } from '@vue/test-utils';
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
import { useDeletionsStore } from '@/stores/deletions';

enableAutoUnmount(afterEach);

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
    expect(eventStream.subscribe).toHaveBeenCalledWith('application:7', expect.any(Function), { fallback: true });
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
    expect(eventStream.subscribe).toHaveBeenCalledWith('application:8', expect.any(Function), { fallback: true });
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
  it('refresh Promise waits for detail, history, questions and forward messages', async () => {
    const releases = [];
    const deferred = () => new Promise((resolve) => releases.push(resolve));
    const host = {
      applicationData: { id: 7 },
      loadApplicationDetails: vi.fn(deferred),
      $refs: {
        historyComponent: { loadHistory: vi.fn(deferred) },
        questionsComponent: { load: vi.fn(deferred) },
        forwardMessagesComponent: { load: vi.fn(deferred) },
      },
    };
    let done = false;
    const pending = ApplicationDetail.methods.refreshLiveDetail.call(host).then(() => { done = true; });
    expect(releases).toHaveLength(4);
    releases.slice(0, 3).forEach((resolve) => resolve());
    await flushPromises();
    expect(done).toBe(false);
    releases[3]();
    await pending;
    expect(done).toBe(true);
  });

  it('live refresh preserves a locally edited comment', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    wrapper.vm.actionComment = 'local draft';
    apiRequest.mockImplementation((url) => Promise.resolve({ ok: true, json: async () =>
      url.endsWith('/details') ? { responsible_users: [{ id: 1, approval_comment: 'saved comment' }] }
        : url.endsWith('/attachments') ? [] : [] }));
    await wrapper.vm.refreshLiveDetail();
    expect(wrapper.vm.actionComment).toBe('local draft');
    wrapper.unmount();
  });

  it('live refresh waits for the separately loaded supplements', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    let finish;
    vi.spyOn(wrapper.vm, 'loadSupplements').mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    apiRequest.mockImplementation((url) => Promise.resolve({ ok: true, json: async () =>
      url.endsWith('/details') ? { id: 7, supplements_count: 1 } : [] }));
    let done = false;
    const pending = wrapper.vm.refreshLiveDetail().then(() => { done = true; });
    await flushPromises();
    expect(finish).toBeTypeOf('function');
    expect(done).toBe(false);
    finish();
    await pending;
    expect(done).toBe(true);
    wrapper.unmount();
  });

  it.each([403, 404])('закрывает карточку сразу после ответа детали %i, пока ещё грузятся вложения', async (status) => {
    const wrapper = mountDetail({ id: 7, message: 'Учебное сообщение' });
    await flushPromises();
    expect(wrapper.text()).toContain('Учебное сообщение');
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    let finishAttachments;
    apiRequest.mockImplementation((url) => {
      if (url.endsWith('/details')) return Promise.resolve({ ok: false, status });
      if (url.endsWith('/attachments')) return new Promise((resolve) => { finishAttachments = resolve; });
      return Promise.resolve({ ok: true, json: async () => [] });
    });
    const update = eventStream.subscribe.mock.calls.find(([scope]) => scope === 'application:7')[1];
    const pending = update();
    await flushPromises();
    expect(wrapper.text()).not.toContain('Учебное сообщение');
    expect(wrapper.emitted('close')).toHaveLength(1);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Заявка больше недоступна' }));
    finishAttachments({ ok: true, json: async () => [{ id: 52 }] });
    await pending;
    expect(wrapper.vm.applicationData).toEqual({});
  });

  it('сохраняет карточку при ошибке сервера и при отказе только по вложениям', async () => {
    const wrapper = mountDetail({ id: 7, message: 'Учебное сообщение' });
    await flushPromises();
    wrapper.vm.attachments = [{ id: 12, attachment_name: 'Прежнее вложение' }];
    wrapper.vm.selectedAttachment = wrapper.vm.attachments[0];
    wrapper.vm.attachmentCars = [{ id: 33 }];
    wrapper.vm.selectedVehicle = wrapper.vm.attachmentCars[0];
    wrapper.vm.showVehicleModal = true;
    apiRequest.mockImplementation((url) => Promise.resolve(url.endsWith('/attachments')
      ? { ok: false, status: 403 }
      : { ok: true, json: async () => ({}) }));
    await wrapper.vm.refreshLiveDetail();
    expect(wrapper.vm.selectedAttachment).toBeNull();
    expect(wrapper.vm.attachmentCars).toEqual([]);
    expect(wrapper.vm.showVehicleModal).toBe(false);
    apiRequest.mockResolvedValue({ ok: false, status: 503 });
    await wrapper.vm.refreshLiveDetail();
    expect(wrapper.text()).toContain('Учебное сообщение');
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('игнорирует поздний отказ для предыдущей заявки', async () => {
    const wrapper = mountDetail({ id: 7, message: 'Прежнее сообщение' });
    await flushPromises();
    let finish;
    apiRequest.mockImplementation((url) => url === '/applications/7/details'
      ? new Promise((resolve) => { finish = resolve; })
      : Promise.resolve({ ok: true, json: async () => [] }));
    const pending = wrapper.vm.refreshLiveDetail();
    await wrapper.setProps({ application: { id: 8, message: 'Новое сообщение' } });
    await flushPromises();
    finish({ ok: false, status: 403 });
    await pending;
    expect(wrapper.emitted('close')).toBeUndefined();
    expect(wrapper.text()).toContain('Новое сообщение');
  });

  it.each([[403, 200], [200, 403]])('ответ %i не отменяет более новое решение %i по той же заявке', async (oldStatus, status) => {
    const wrapper = mountDetail({ id: 7, message: 'Учебное сообщение' });
    await flushPromises();
    let finish, delay = true;
    const result = (code) => ({ ok: code === 200, status: code, json: async () => ({ message: 'Свежая заявка' }) });
    apiRequest.mockImplementation((url) => {
      if (!url.endsWith('/details')) return Promise.resolve({ ok: true, json: async () => [] });
      if (!delay) return Promise.resolve(result(status));
      delay = false;
      return new Promise((resolve) => { finish = resolve; });
    });
    const earlier = wrapper.vm.refreshLiveDetail();
    await wrapper.vm.refreshLiveDetail();
    finish(result(oldStatus));
    await earlier;
    if (status === 200) {
      expect(wrapper.emitted('close')).toBeUndefined();
      expect(wrapper.text()).toContain('Свежая заявка');
    } else {
      expect(wrapper.emitted('close')).toHaveLength(1);
      expect(wrapper.text()).not.toContain('Свежая заявка');
    }
  });

  it('проверяет открытую видимую заявку даже без SSE и после возврата во вкладку', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] });
    const hidden = vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
    try {
      const wrapper = mountDetail({ id: 7, message: 'Учебное сообщение' });
      await flushPromises();
      apiRequest.mockClear();
      hidden.mockReturnValue(true);
      vi.advanceTimersByTime(60_000);
      expect(apiRequest).not.toHaveBeenCalled();
      hidden.mockReturnValue(false);
      document.dispatchEvent(new Event('visibilitychange'));
      await flushPromises();
      expect(apiRequest.mock.calls.map(([path]) => path)).toEqual(['/applications/7/details']);
      apiRequest.mockRejectedValueOnce(new Error('offline'));
      window.dispatchEvent(new Event('online'));
      await flushPromises();
      expect(wrapper.emitted('close')).toBeUndefined();
      apiRequest.mockResolvedValue({ ok: false, status: 403 });
      vi.advanceTimersByTime(60_000);
      await flushPromises();
      expect(wrapper.emitted('close')).toHaveLength(1);
      const timer = wrapper.vm.accessCheckTimer;
      const clear = vi.spyOn(globalThis, 'clearInterval');
      wrapper.unmount();
      expect(clear).toHaveBeenCalledWith(timer);
      clear.mockRestore();
    } finally {
      hidden.mockRestore();
      vi.useRealTimers();
    }
  });

});
