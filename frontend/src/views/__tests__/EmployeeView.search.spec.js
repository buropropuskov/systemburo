import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeView from '../EmployeeView.vue';

// Реестр сотрудников переведён на серверный поиск/пагинацию (#1158, срез 3): проверяем,
// что search_query/page/per_page реально уходят на бэк (getUniqueEmployeesPaginated),
// пагинация аккумулирует порции, а смена фильтра/поиска сбрасывает аккумулятор.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listPersonBlacklist: vi.fn().mockResolvedValue([]) }));

const getUniqueEmployeesPaginated = vi.fn();
vi.mock('@/api/employees', () => ({
  getUniqueEmployeesPaginated: (...args) => getUniqueEmployeesPaginated(...args),
}));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  EmployeeEditModal: true,
  EmployeeDetailsModal: true,
  ApplicationDetail: true,
};

function mountView() {
  return mount(EmployeeView, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn() } } },
  });
}

function page(items, total, pageNum = 1, perPage = 30) {
  return Promise.resolve({ items, meta: { total, page: pageNum, per_page: perPage } });
}

let wrapper;

describe('EmployeeView - серверный поиск и пагинация (#1158, срез 3)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueEmployeesPaginated.mockReset();
    getUniqueEmployeesPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });

  afterEach(() => {
    wrapper?.unmount();
    vi.useRealTimers();
  });

  it('первичная загрузка без поиска не передаёт search_query', async () => {
    wrapper = mountView();
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalled();
    const params = getUniqueEmployeesPaginated.mock.calls.at(-1)[0];
    expect(params.filter_type).toBe('user');
    expect(params.page).toBe(1);
    expect(params.per_page).toBe(30);
    expect(params.search_query).toBeUndefined();
  });

  it('ввод в поиск уходит на сервер с дебаунсом (search_query в параметрах запроса)', async () => {
    vi.useFakeTimers();
    wrapper = mountView();
    await flushPromises();
    getUniqueEmployeesPaginated.mockClear();

    wrapper.vm.searchQuery = 'Иванов';
    await wrapper.vm.$nextTick();
    // До истечения дебаунса запрос ещё не ушёл.
    expect(getUniqueEmployeesPaginated).not.toHaveBeenCalled();

    vi.advanceTimersByTime(300);
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalled();
    const params = getUniqueEmployeesPaginated.mock.calls.at(-1)[0];
    expect(params.search_query).toBe('Иванов');
  });

  it('пагинация аккумулирует порции: loadMore добавляет к списку, не заменяет его', async () => {
    getUniqueEmployeesPaginated.mockImplementation((params) => (
      params.page === 1 ? page([{ id: 1, last_name: 'A' }], 2) : page([{ id: 2, last_name: 'B' }], 2)
    ));
    wrapper = mountView();
    await flushPromises();
    expect(wrapper.vm.employeesData).toHaveLength(1);

    await wrapper.vm.loadMoreEmployeesList(wrapper.vm.buildEmployeesPage);
    await flushPromises();

    expect(wrapper.vm.employeesData).toHaveLength(2);
    expect(wrapper.vm.employeesData.map((e) => e.id)).toEqual([1, 2]);
  });

  it('смена filter_type сбрасывает на страницу 1 и очищает накопленные порции', async () => {
    getUniqueEmployeesPaginated.mockImplementation((params) => {
      if (params.filter_type === 'user' && params.page === 1) return page([{ id: 1, last_name: 'A' }], 2);
      if (params.filter_type === 'user' && params.page === 2) return page([{ id: 2, last_name: 'B' }], 2);
      if (params.filter_type === 'organization') return page([{ id: 9, last_name: 'ORG1' }], 1);
      return page([], 0);
    });
    wrapper = mountView();
    await flushPromises();
    await wrapper.vm.loadMoreEmployeesList(wrapper.vm.buildEmployeesPage);
    await flushPromises();
    expect(wrapper.vm.employeesData).toHaveLength(2);

    wrapper.vm.switchFilter('organization');
    await flushPromises();

    expect(wrapper.vm.employeesData).toHaveLength(1);
    expect(wrapper.vm.employeesData[0].id).toBe(9);
  });

  it('счётчик футера использует серверный meta.total, а не размер загруженной порции', async () => {
    getUniqueEmployeesPaginated.mockResolvedValue({
      items: [{ id: 1, last_name: 'A' }],
      meta: { total: 42, page: 1, per_page: 30 },
    });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.vm.employeesTotal).toBe(42);
    expect(wrapper.vm.footerText).toBe('Показано 1 из 42');
  });
});
