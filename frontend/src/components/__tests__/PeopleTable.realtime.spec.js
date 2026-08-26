import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import eventStream from '@/services/eventStream';
import PeopleTable from '../PeopleTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function deferred() {
  let resolve;
  const promise = new Promise((res) => { resolve = res; });
  return { promise, resolve };
}

function mountTable(props = {}) {
  return mount(PeopleTable, {
    props: {
      tableName: 'КПП-72',
      currentUserId: 1,
      currentUserName: 'Тест',
      ...props,
    },
    global: {
      stubs: { teleport: true, transition: false, 'transition-group': false },
    },
  });
}

describe('PeopleTable - real-time обновление по сигналу tables.refresh (#840)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return Promise.resolve(okResponse({ table: { id: 7 } }));
      if (url.startsWith('/system-tables')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/organizations')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/employees/active-for-table/')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/employees/history/current-status')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/notifications/deletion-settings')) return Promise.resolve(okResponse({}));
      return Promise.resolve(okResponse({}));
    });
    eventStream.connect.mockClear();
    eventStream.disconnect.mockClear();
    eventStream.subscribe.mockClear();
    eventStream.subscribe.mockImplementation(() => vi.fn());
    eventStream.onStatus.mockClear();
    eventStream.onStatus.mockImplementation(() => vi.fn());
  });

  it('после резолва tableId по имени таблицы подписывается на scope tables:<id> и подключается', async () => {
    const wrapper = mountTable();
    await flushPromises();

    expect(eventStream.connect).toHaveBeenCalledTimes(1);
    expect(eventStream.subscribe).toHaveBeenCalledWith('tables:7', expect.any(Function));
    expect(wrapper.vm.currentTableId).toBe(7);

    wrapper.unmount();
  });

  it('колбэк subscribe дёргает silentRefresh', async () => {
    const wrapper = mountTable();
    await flushPromises();

    const scopeCb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'tables:7')[1];
    apiRequest.mockClear();
    scopeCb();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/employees/active-for-table/7', { method: 'GET' });

    wrapper.unmount();
  });

  it('тик 60с-поллинга не бьёт по сети при sseConnected=true, но бьёт при false', async () => {
    vi.useFakeTimers();
    const wrapper = mountTable();
    await flushPromises();

    wrapper.vm.sseConnected = true;
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(60000);
    expect(apiRequest).not.toHaveBeenCalledWith('/employees/active-for-table/7', { method: 'GET' });

    wrapper.vm.sseConnected = false;
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(60000);
    expect(apiRequest).toHaveBeenCalledWith('/employees/active-for-table/7', { method: 'GET' });

    wrapper.unmount();
    vi.useRealTimers();
  });

  it('seq-guard: устаревший (медленный) silentRefresh не затирает данные более свежего (быстрого)', async () => {
    const wrapper = mountTable();
    await flushPromises();

    const slow = deferred();
    const fast = deferred();
    let empCall = 0;
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return Promise.resolve(okResponse({ table: { id: 7 } }));
      if (url.startsWith('/employees/active-for-table/')) {
        empCall += 1;
        if (empCall === 1) {
          return slow.promise.then(() => okResponse([{ id: 1, last_name: 'Old' }]));
        }
        return fast.promise.then(() => okResponse([{ id: 2, last_name: 'New' }]));
      }
      if (url.startsWith('/organizations')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/system-tables')) return Promise.resolve(okResponse([]));
      return Promise.resolve(okResponse([]));
    });

    // Первый вызов (медленный ответ) стартует первым - самый старый seq.
    const p1 = wrapper.vm.silentRefresh();
    // Второй вызов (быстрый ответ) стартует следом - более свежий seq.
    const p2 = wrapper.vm.silentRefresh();

    fast.resolve();
    await flushPromises();
    expect(wrapper.vm.itemsData.map((i) => i.id)).toEqual([2]);

    slow.resolve();
    await flushPromises();
    await Promise.all([p1, p2]);

    expect(wrapper.vm.itemsData.map((i) => i.id)).toEqual([2]);

    wrapper.unmount();
  });

  it('при unmount отписывается от scope и статуса, отключает eventStream', async () => {
    const unsubScope = vi.fn();
    const unsubStatus = vi.fn();
    eventStream.subscribe.mockImplementation(() => unsubScope);
    eventStream.onStatus.mockImplementation(() => unsubStatus);

    const wrapper = mountTable();
    await flushPromises();
    wrapper.unmount();

    expect(unsubScope).toHaveBeenCalledTimes(1);
    expect(unsubStatus).toHaveBeenCalledTimes(1);
    expect(eventStream.disconnect).toHaveBeenCalledTimes(1);
  });
});
