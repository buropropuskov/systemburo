import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Вкладка «Аналитика» раздела мониторинга (#2125): под фильтром периода стоит
// подпись, что именно показано - запрошенный период, сутки с записями и откуда
// взяты числа. Раньше месяц без свёрнутых агрегатов выглядел как «запросов не
// было», хотя данные просто лежали в детальных партициях.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import RequestsView from '@/views/RequestsView.vue';
import { apiRequest } from '@/api/client';

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" />' },
  SearchComponent: { template: '<input class="search-stub" />' },
  RealTimeChart: { template: '<div class="chart-stub" />' },
  LoaderSpinner: { template: '<div class="loader-stub" />' },
  AppIcon: { template: '<i />' },
};

function history(over = {}) {
  return {
    totals: { requests: 102, errors: 2, error_rate: 1.96, avg_duration_ms: 8.4 },
    coverage: {
      requested_from: '2026-07-05',
      requested_to: '2026-08-19',
      from: '2026-07-10',
      to: '2026-08-19',
      days: 25,
      source: 'mixed',
      aggregated_through: '2026-07-18',
      exact_p95: false,
      ...(over.coverage || {}),
    },
    daily: [{ day: '2026-07-10', requests: 100, errors: 2 }],
    top_endpoints: [
      { endpoint: '/api/applications', requests: 100, avg_duration_ms: 8.4, p95_duration_ms: 240.5, error_rate: 2 },
    ],
    top_users: [],
    ...(over.root || {}),
  };
}

// Раздел дёргает пять эндпоинтов при монтировании; аналитика - только по клику
// на вкладку, поэтому её ответ подставляется отдельно.
function mockApi(historyBody) {
  apiRequest.mockImplementation((url) => {
    const body = url.startsWith('/request-logs/history') ? historyBody : [];
    return Promise.resolve({ ok: true, json: () => Promise.resolve(body) });
  });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
});

afterEach(() => {
  wrapper?.unmount();
});

async function openAnalytics(historyBody) {
  mockApi(historyBody);
  wrapper = mount(RequestsView, { global: { stubs } });
  await flushPromises();
  await wrapper.vm.switchToAnalytics();
  await flushPromises();
  return wrapper.find('.analytics-tab');
}

describe('RequestsView, охват периода аналитики', () => {
  it('называет запрошенный период, сутки с записями и источник чисел', async () => {
    const tab = await openAnalytics(history());
    const note = tab.find('.coverage-note').text();

    expect(note).toContain('Запрошен период 05.07.2026 - 19.08.2026');
    expect(note).toContain('Записи есть за 10.07.2026 - 19.08.2026');
    expect(note).toContain('суток с данными: 25');
    expect(note).toContain('по свёрнутым итогам и подробным записям');
  });

  it('на пустом периоде объясняет словами, а не показывает голые нули', async () => {
    const tab = await openAnalytics(history({
      coverage: { from: '', to: '', days: 0, source: 'empty' },
      root: { totals: { requests: 0, errors: 0, error_rate: 0, avg_duration_ms: 0 }, daily: [], top_endpoints: [] },
    }));

    expect(tab.find('.coverage-note').text()).toBe('За период 05.07.2026 - 19.08.2026 записей нет.');
  });

  it('помечает перцентиль звёздочкой, пока в периоде есть свёрнутые сутки', async () => {
    const tab = await openAnalytics(history());

    expect(tab.find('.hist-table thead').text()).toContain('p95*');
    expect(tab.findAll('.coverage-note').at(-1).text())
      .toContain('наибольшее суточное значение');
  });

  it('без свёрнутых суток перцентиль честный и оговорки нет', async () => {
    const tab = await openAnalytics(history({
      coverage: { source: 'detailed', exact_p95: true, aggregated_through: '' },
    }));

    expect(tab.find('.hist-table thead').text()).not.toContain('p95*');
    expect(tab.findAll('.coverage-note')).toHaveLength(1);
  });

  it('показывает взвешенную среднюю с дробной частью, а не округлённой в ноль', async () => {
    const tab = await openAnalytics(history({
      root: { totals: { requests: 100, errors: 0, error_rate: 0, avg_duration_ms: 0.4 } },
    }));

    expect(tab.text()).toContain('0.4мс');
  });
});
