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
import CarsTable from '../CarsTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function deferred() {
  let resolve;
  const promise = new Promise((res) => { resolve = res; });
  return { promise, resolve };
}

function mountTable(props = {}) {
  return mount(CarsTable, {
    props: {
      tableName: 'КПП 1',
      tableId: 42,
      currentUserId: 1,
      currentUserName: 'Тест',
      ...props,
    },
    global: {
      stubs: { teleport: true, transition: false, 'transition-group': false },
    },
  });
}

describe('CarsTable - real-time обновление по сигналу tables.refresh (#840)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/unload-places')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/license-plate-formats')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/cars/active-for-table/')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/cars/unload-places')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/cars/history/current-status')) return Promise.resolve(okResponse([]));
      if (url.startsWith('/organizations')) return Promise.resolve(okResponse([]));
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

  it('при монтировании подписывается на scope tables:<tableId> и подключается', async () => {
    const wrapper = mountTable();
    await flushPromises();

    expect(eventStream.connect).toHaveBeenCalledTimes(1);
    expect(eventStream.subscribe).toHaveBeenCalledWith('tables:42', expect.any(Function));

    wrapper.unmount();
  });

  it('колбэк subscribe дёргает silentRefresh', async () => {
    const wrapper = mountTable();
    await flushPromises();

    const scopeCb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'tables:42')[1];
    apiRequest.mockClear();
    scopeCb();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/cars/active-for-table/42', {});

    wrapper.unmount();
  });

  it('тик 60с-поллинга не бьёт по сети при sseConnected=true, но бьёт при false', async () => {
    vi.useFakeTimers();
    const wrapper = mountTable();
    await flushPromises();

    wrapper.vm.sseConnected = true;
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(60000);
    expect(apiRequest).not.toHaveBeenCalledWith('/cars/active-for-table/42', {});

    wrapper.vm.sseConnected = false;
    apiRequest.mockClear();
    await vi.advanceTimersByTimeAsync(60000);
    expect(apiRequest).toHaveBeenCalledWith('/cars/active-for-table/42', {});

    wrapper.unmount();
    vi.useRealTimers();
  });

  it('seq-guard: устаревший (медленный) silentRefresh не затирает данные более свежего (быстрого)', async () => {
    const wrapper = mountTable();
    await flushPromises();

    const slow = deferred();
    const fast = deferred();
    let carsCall = 0;
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/cars/active-for-table/')) {
        carsCall += 1;
        if (carsCall === 1) {
          return slow.promise.then(() => okResponse([
            { id: 1, car_number: 'OLD', car_brand: '', organization: '', status: 1 },
          ]));
        }
        return fast.promise.then(() => okResponse([
          { id: 2, car_number: 'NEW', car_brand: '', organization: '', status: 1 },
        ]));
      }
      if (url.startsWith('/organizations')) return Promise.resolve(okResponse([]));
      return Promise.resolve(okResponse([]));
    });

    // Первый вызов (позже резолвится медленным ответом) стартует первым - самый старый seq.
    const p1 = wrapper.vm.silentRefresh();
    // Второй вызов (резолвится быстро) стартует следом - более свежий seq.
    const p2 = wrapper.vm.silentRefresh();

    // Быстрый ответ приходит первым и должен применить свои данные.
    fast.resolve();
    await flushPromises();
    expect(wrapper.vm.itemsData.map((i) => i.id)).toEqual([2]);

    // Медленный (устаревший) ответ приходит позже - не должен затереть свежие данные.
    slow.resolve();
    await flushPromises();
    await Promise.all([p1, p2]);

    expect(wrapper.vm.itemsData.map((i) => i.id)).toEqual([2]);

    wrapper.unmount();
  });

  it('seq-guard в fetchCarUnloadPlaces: устаревший ответ не затирает карту мест разгрузки', async () => {
    const wrapper = mountTable();
    await flushPromises();

    // Зафиксируем текущее состояние карты и seq после первичной загрузки.
    wrapper.vm.carUnloadPlacesMap = { 1: [{ id: 5, name: 'Склад A' }] };
    const staleSeq = wrapper.vm.refreshSeq - 1; // seq предыдущего (устаревшего) вызова

    const dfr = deferred();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/cars/unload-places')) {
        return dfr.promise.then(() => okResponse([
          { car_id: 9, unload_place_id: 99, unload_place_name: 'STALE' },
        ]));
      }
      return Promise.resolve(okResponse([]));
    });

    const p = wrapper.vm.fetchCarUnloadPlaces(staleSeq);
    dfr.resolve();
    await flushPromises();
    await p;

    // Устаревший ответ отброшен guard'ом - карта не перезаписана данными STALE.
    expect(wrapper.vm.carUnloadPlacesMap).toEqual({ 1: [{ id: 5, name: 'Склад A' }] });

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
