import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';

// Частота опросов раздела (#2125): показатели, график и счётчики ленты живут в
// разных компонентах, но эндпоинт у каждого свой и один. После разбора экрана
// на вкладки показатели какое-то время читались дважды за окно - таймер остался
// и в оболочке, и во вкладке журнала.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import { apiRequest } from '@/api/client';
import { mountView, resetApiMocks, unmountAll } from './helpers/requestsView';

/** Сколько раз раздел сходил по адресу за время теста. */
function callsTo(prefix) {
  return apiRequest.mock.calls.filter(([url]) => url.startsWith(prefix)).length;
}

afterEach(() => {
  unmountAll();
  vi.useRealTimers();
});

beforeEach(() => {
  resetApiMocks();
});

describe('RequestsView, частота опросов', () => {
  it('за полминуты показатели и график читаются по одному разу', async () => {
    vi.useFakeTimers();
    await mountView();
    await flushPromises();
    apiRequest.mockClear();

    vi.advanceTimersByTime(30000);
    await flushPromises();

    expect(callsTo('/request-logs/stats'), 'показатели опрашивает только оболочка').toBe(1);
    expect(callsTo('/request-logs/timeline'), 'график опрашивает только журнал').toBe(1);
  });

  it('счётчики ленты идут своим шагом в пять секунд', async () => {
    vi.useFakeTimers();
    await mountView();
    await flushPromises();
    apiRequest.mockClear();

    vi.advanceTimersByTime(10000);
    await flushPromises();

    expect(callsTo('/request-logs/realtime')).toBe(2);
    expect(callsTo('/request-logs/stats'), 'показатели по пятисекундному шагу не ходят').toBe(0);
  });

  it('обновление списка подтягивает показатели сразу, не дожидаясь таймера', async () => {
    const { wrapper } = await mountView();
    await flushPromises();
    apiRequest.mockClear();

    await wrapper.get('.refresh-stub').trigger('click');
    await flushPromises();

    expect(callsTo('/request-logs/stats'), 'числа в шапке обязаны совпасть с обновлённым списком').toBe(1);
  });
});
