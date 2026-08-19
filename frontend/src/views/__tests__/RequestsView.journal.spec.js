import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { reactive } from 'vue';
import { createPinia, setActivePinia } from 'pinia';

// Живой журнал раздела мониторинга (#2125): порядок строк задаёт сервер, отбор
// живёт в адресной строке, а медленный ответ прошлого фильтра не затирает
// свежий. До этого клик по заголовку двигал только стрелку.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import RequestsView from '@/views/RequestsView.vue';
import { apiRequest, apiRequestRaw } from '@/api/client';

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" />' },
  SearchComponent: { template: '<input class="search-stub" />' },
  RealTimeChart: { template: '<div class="chart-stub" />' },
  LoaderSpinner: { template: '<div class="loader-stub" />' },
  AppIcon: { template: '<i />' },
};

function logsPage(data = []) {
  return {
    ok: true,
    json: () => Promise.resolve({
      success: true,
      data,
      meta: { total: data.length, page: 1, per_page: 20 },
    }),
  };
}

/** Адреса, с которыми компонент сходил за списком журнала. */
function journalCalls() {
  return apiRequestRaw.mock.calls.map(([url]) => url);
}

// Роутер подменяется живым: replace обязан менять то, что компонент потом
// читает как текущий адрес, иначе повторная запись выглядит лишней и тест
// зеленеет там, где ссылка на самом деле не обновилась.
function mountView(query = {}) {
  const route = reactive({ query: { ...query } });
  const replace = vi.fn(({ query: next }) => {
    route.query = { ...next };
    return Promise.resolve();
  });
  const wrapper = mount(RequestsView, {
    global: {
      stubs,
      mocks: {
        $route: route,
        $router: { replace },
      },
    },
  });
  return { wrapper, route, replace };
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
  apiRequestRaw.mockResolvedValue(logsPage());
});

describe('RequestsView, порядок строк', () => {
  it('клик по заголовку перезапрашивает список, повторный клик разворачивает порядок', async () => {
    const { wrapper } = mountView();
    await flushPromises();
    apiRequestRaw.mockClear();

    const duration = wrapper.findAll('.header-col').find(col => col.text().includes('Отклик'));
    expect(duration, 'колонка «Отклик» есть в шапке таблицы').toBeTruthy();

    await duration.trigger('click');
    await flushPromises();
    expect(journalCalls().at(-1)).toContain('sort=duration');
    expect(journalCalls().at(-1)).toContain('order=desc');

    await duration.trigger('click');
    await flushPromises();
    expect(journalCalls().at(-1)).toContain('order=asc');
  });

  it('клик по другой колонке сбрасывает страницу на первую', async () => {
    const { wrapper } = mountView({ page: '3' });
    await flushPromises();
    expect(journalCalls()[0]).toContain('page=3');

    const method = wrapper.findAll('.header-col').find(col => col.text().includes('Метод'));
    await method.trigger('click');
    await flushPromises();

    const last = journalCalls().at(-1);
    expect(last, 'сортировка показывает первую страницу нового порядка').toContain('page=1');
    expect(last).toContain('sort=method');
  });
});

describe('RequestsView, отбор в адресной строке', () => {
  it('открывает журнал в том виде, в каком записан адрес', async () => {
    const { wrapper } = mountView({
      method: 'post',
      status: '500',
      from: '2026-08-01',
      to: '2026-08-19',
      sort: 'status',
      order: 'asc',
      per_page: '50',
    });
    await flushPromises();

    const url = journalCalls()[0];
    expect(url).toContain('method=POST');
    expect(url).toContain('status=500');
    expect(url).toContain('from_date=2026-08-01');
    expect(url).toContain('to_date=2026-08-19');
    expect(url).toContain('sort=status');
    expect(url).toContain('order=asc');
    expect(url).toContain('per_page=50');
    expect(wrapper.vm.sortField).toBe('status');
    expect(wrapper.vm.sortDirection).toBe('asc');
  });

  it('чужое поле сортировки из адреса не применяется', async () => {
    const { wrapper } = mountView({ sort: 'response_body', order: 'asc' });
    await flushPromises();

    expect(wrapper.vm.sortField).toBe('created_at');
    expect(journalCalls()[0]).toContain('sort=created_at');
  });

  it('смена отбора складывается обратно в адрес, значения по умолчанию не пишутся', async () => {
    const { wrapper, replace } = mountView();
    await flushPromises();
    replace.mockClear();

    wrapper.vm.filterMethod = 'DELETE';
    wrapper.vm.filterStartDate = '2026-08-10';
    await wrapper.vm.refreshLogs();
    await flushPromises();

    expect(replace).toHaveBeenCalledWith({ query: { method: 'DELETE', from: '2026-08-10' } });

    replace.mockClear();
    await wrapper.vm.clearFilters();
    await flushPromises();
    expect(replace).toHaveBeenCalledWith({ query: {} });
  });
});

describe('RequestsView, гонка запросов', () => {
  it('ответ прошлого отбора не затирает свежий', async () => {
    const { wrapper } = mountView();
    await flushPromises();

    let releaseStale;
    const stale = new Promise(resolve => { releaseStale = resolve; });
    apiRequestRaw.mockImplementationOnce(() => stale);
    apiRequestRaw.mockImplementationOnce(() => Promise.resolve(
      logsPage([{ id: 2, url: '/api/fresh', method: 'GET', response_status: 200, duration_us: 900 }])
    ));

    wrapper.vm.filterMethod = 'GET';
    const first = wrapper.vm.fetchLogs();
    wrapper.vm.filterMethod = 'POST';
    const second = wrapper.vm.fetchLogs();

    await second;
    releaseStale(logsPage([{ id: 1, url: '/api/stale', method: 'GET', response_status: 200, duration_us: 100 }]));
    await first;
    await flushPromises();

    expect(wrapper.vm.logs.map(l => l.url)).toEqual(['/api/fresh']);
    expect(wrapper.vm.isLoading, 'запоздавший ответ не гасит и не зажигает загрузку заново').toBe(false);
  });
});
