import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import CarsView from '../CarsView.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { apiRequest } from '@/api/client';

// Реестр машин переведён на серверный поиск/пагинацию (#1158, срез 2): проверяем,
// что search_query/page/per_page реально уходят на бэк (getUniqueCarsPaginated),
// пагинация аккумулирует порции, а смена фильтра/поиска сбрасывает аккумулятор.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listVehicleBlacklist: vi.fn().mockResolvedValue([]) }));

const getUniqueCarsPaginated = vi.fn();
vi.mock('@/api/cars', () => ({
  getUniqueCarsPaginated: (...args) => getUniqueCarsPaginated(...args),
}));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  ConfirmationModal: true,
  VehicleDetailsModal: true,
  ApplicationDetail: true,
};

function mountView() {
  return mount(CarsView, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

function page(items, total, pageNum = 1, perPage = 30) {
  return Promise.resolve({ items, meta: { total, page: pageNum, per_page: perPage } });
}

let wrapper;

describe('CarsView - серверный поиск и пагинация (#1158, срез 2)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueCarsPaginated.mockReset();
    getUniqueCarsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });

  afterEach(() => {
    wrapper?.unmount();
    vi.useRealTimers();
  });

  it('первичная загрузка без поиска не передаёт search_query', async () => {
    wrapper = mountView();
    await flushPromises();

    expect(getUniqueCarsPaginated).toHaveBeenCalled();
    const params = getUniqueCarsPaginated.mock.calls.at(-1)[0];
    expect(params.filter_type).toBe('user');
    expect(params.page).toBe(1);
    expect(params.per_page).toBe(30);
    expect(params.search_query).toBeUndefined();
  });

  it('ввод в поиск уходит на сервер с дебаунсом (search_query в параметрах запроса)', async () => {
    vi.useFakeTimers();
    wrapper = mountView();
    await flushPromises();
    getUniqueCarsPaginated.mockClear();

    wrapper.vm.searchQuery = 'А777АА';
    await wrapper.vm.$nextTick();
    // До истечения дебаунса запрос ещё не ушёл.
    expect(getUniqueCarsPaginated).not.toHaveBeenCalled();

    vi.advanceTimersByTime(300);
    await flushPromises();

    expect(getUniqueCarsPaginated).toHaveBeenCalled();
    const params = getUniqueCarsPaginated.mock.calls.at(-1)[0];
    expect(params.search_query).toBe('А777АА');
  });

  it('пагинация аккумулирует порции: loadMore добавляет к списку, не заменяет его', async () => {
    getUniqueCarsPaginated.mockImplementation((params) => (
      params.page === 1 ? page([{ id: 1, number: 'A1' }], 2) : page([{ id: 2, number: 'A2' }], 2)
    ));
    wrapper = mountView();
    await flushPromises();
    expect(wrapper.vm.carsData).toHaveLength(1);

    await wrapper.vm.loadMoreCarsList(wrapper.vm.buildCarsPage);
    await flushPromises();

    expect(wrapper.vm.carsData).toHaveLength(2);
    expect(wrapper.vm.carsData.map((c) => c.id)).toEqual([1, 2]);
  });

  it('смена filter_type сбрасывает на страницу 1 и очищает накопленные порции', async () => {
    getUniqueCarsPaginated.mockImplementation((params) => {
      if (params.filter_type === 'user' && params.page === 1) return page([{ id: 1, number: 'A1' }], 2);
      if (params.filter_type === 'user' && params.page === 2) return page([{ id: 2, number: 'A2' }], 2);
      if (params.filter_type === 'organization') return page([{ id: 9, number: 'ORG1' }], 1);
      return page([], 0);
    });
    wrapper = mountView();
    await flushPromises();
    await wrapper.vm.loadMoreCarsList(wrapper.vm.buildCarsPage);
    await flushPromises();
    expect(wrapper.vm.carsData).toHaveLength(2);

    wrapper.vm.switchFilter('organization');
    await flushPromises();

    expect(wrapper.vm.carsData).toHaveLength(1);
    expect(wrapper.vm.carsData[0].id).toBe(9);
  });

  it('поиск НЕ дёргает эндпоинты мест разгрузки (они не зависят от search_query)', async () => {
    vi.useFakeTimers();
    wrapper = mountView();
    await flushPromises();
    // На mount места разгрузки грузятся (withPlaces=true) - это ожидаемо.
    apiRequest.mockClear();

    wrapper.vm.searchQuery = 'BMW';
    await wrapper.vm.$nextTick();
    vi.advanceTimersByTime(300);
    await flushPromises();

    const placeCalls = apiRequest.mock.calls.filter(
      ([path]) => path === '/unload-places' || path === '/cars/unload-places',
    );
    expect(placeCalls).toHaveLength(0);
  });

  it('смена filter_type ГРУЗИТ места разгрузки (active_car_id/набор мест могли смениться)', async () => {
    wrapper = mountView();
    await flushPromises();
    apiRequest.mockClear();

    wrapper.vm.switchFilter('organization');
    await flushPromises();

    const placeCalls = apiRequest.mock.calls.filter(
      ([path]) => path === '/unload-places' || path === '/cars/unload-places',
    );
    expect(placeCalls.length).toBeGreaterThan(0);
  });

  it('счётчик футера использует серверный meta.total, а не размер загруженной порции', async () => {
    getUniqueCarsPaginated.mockResolvedValue({
      items: [{ id: 1, number: 'A1' }],
      meta: { total: 42, page: 1, per_page: 30 },
    });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.vm.carsTotal).toBe(42);
    expect(wrapper.vm.footerText).toBe('Показано 1 из 42');
  });
});

/**
 * Переход из сквозного поиска ведёт к самой машине: `?q` сужает реестр, `?open`
 * раскрывает её карточку. Раньше открывался просто раздел.
 */
describe('CarsView - открытие карточки по ссылке из сквозного поиска', () => {
  const CAR = { id: 7, number: 'А777АА', mark: 'Toyota', status: true };

  function mountWithRoute(query, replace = vi.fn().mockResolvedValue(undefined)) {
    return mount(CarsView, {
      global: { stubs, mocks: { $route: { query }, $router: { push: vi.fn(), replace } } },
    });
  }

  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueCarsPaginated.mockReset();
    getUniqueCarsPaginated.mockResolvedValue({ items: [CAR], meta: { total: 1, page: 1, per_page: 30 } });
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('карточка найденной машины открывается сразу', async () => {
    wrapper = mountWithRoute({ q: 'а777', open: '7' });
    await flushPromises();

    expect(wrapper.vm.showDetailsViewModal).toBe(true);
    expect(wrapper.vm.detailsCar.plateNumber).toBe('А777АА');
  });

  it('open вычищается из адреса после открытия', async () => {
    const replace = vi.fn().mockResolvedValue(undefined);
    wrapper = mountWithRoute({ q: 'а777', open: '7' }, replace);
    await flushPromises();

    expect(replace).toHaveBeenCalledWith({ query: { q: 'а777' } });
  });

  it('машины нет среди загруженных - карточка не открывается', async () => {
    wrapper = mountWithRoute({ q: 'а777', open: '999' });
    await flushPromises();

    expect(wrapper.vm.showDetailsViewModal).toBe(false);
  });
});

/** Та же поправка, что у сотрудников: переход из поиска открывал «Мои машины». */
describe('CarsView - область реестра при переходе из поиска', () => {
  function seedPerms(allow) {
    const perms = usePermissionsStore();
    perms.mode = 'normal';
    perms.effective = Object.fromEntries(allow.map((k) => [k, { value: 'allow', source: 'role' }]));
  }

  function mountWithRoute(query) {
    return mount(CarsView, {
      global: { stubs, mocks: { $route: { query }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
    });
  }

  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueCarsPaginated.mockReset();
    getUniqueCarsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('с open в адресе список запрашивается по всей системе', async () => {
    seedPerms(['section.registry.all_system']);
    wrapper = mountWithRoute({ q: 'а777', open: '7' });
    await flushPromises();

    expect(getUniqueCarsPaginated).toHaveBeenCalledWith(expect.objectContaining({ filter_type: 'all_system' }));
  });

  it('обычный заход по-прежнему открывает «Мои машины»', async () => {
    seedPerms(['section.registry.all_system']);
    wrapper = mountWithRoute({});
    await flushPromises();

    expect(getUniqueCarsPaginated).toHaveBeenCalledWith(expect.objectContaining({ filter_type: 'user' }));
  });
});
