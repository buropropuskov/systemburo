import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import CarsView from '../CarsView.vue';

// Устойчивость реестра машин к ошибкам бэка (#1173): при упавшей первичной загрузке
// показываем error+retry вместо "Автомобилей нет", а во время in-flight retry -
// спиннер, НЕ ложное "нет данных" (retry выставляет composable listLoading, а не
// верхнеуровневый this.loading).

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

let wrapper;

describe('CarsView — устойчивость к ошибкам бэка (#1173)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueCarsPaginated.mockReset();
  });

  afterEach(() => wrapper?.unmount());

  it('первичная загрузка упала - error+retry вместо "Автомобилей нет"', async () => {
    getUniqueCarsPaginated.mockRejectedValue(new Error('502'));
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.vm.listError).toBe(true);
    expect(wrapper.find('[data-testid="cars-list-error"]').exists()).toBe(true);
    expect(wrapper.find('.no-data-message').exists()).toBe(false);
    expect(wrapper.vm.hasMoreCars).toBe(false);
    expect(wrapper.find('[data-testid="cars-scroll-sentinel"]').exists()).toBe(false);
  });

  it('во время in-flight retry рендерится спиннер, НЕ "Автомобилей нет"', async () => {
    getUniqueCarsPaginated.mockRejectedValueOnce(new Error('502'));
    wrapper = mountView();
    await flushPromises();
    expect(wrapper.vm.listError).toBe(true);

    // Deferred: retry подвисает - ловим ПРОМЕЖУТОЧНОЕ состояние.
    let resolveRetry;
    getUniqueCarsPaginated.mockImplementationOnce(
      () => new Promise((r) => { resolveRetry = r; }),
    );
    await wrapper.find('[data-testid="cars-list-error"] button').trigger('click');
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.listLoading).toBe(true);
    expect(wrapper.find('[data-testid="cars-list-loading"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="cars-list-error"]').exists()).toBe(false);
    expect(wrapper.find('.no-data-message').exists()).toBe(false);

    resolveRetry({ items: [{ id: 1, number: 'A1' }], meta: { total: 1, page: 1, per_page: 30 } });
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.carsData.map((c) => c.id)).toEqual([1]);
    expect(wrapper.find('[data-testid="cars-list-loading"]').exists()).toBe(false);
  });

  it('ошибка догрузки: sentinel показывает error+retry, circuit-breaker гасит автодолбёж', async () => {
    getUniqueCarsPaginated.mockResolvedValueOnce({
      items: [{ id: 1, number: 'A1' }],
      meta: { total: 4, page: 1, per_page: 30 },
    });
    wrapper = mountView();
    await flushPromises();
    wrapper.vm.loading = false;
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.carsData).toHaveLength(1);

    getUniqueCarsPaginated.mockRejectedValueOnce(new Error('502'));
    await wrapper.vm.loadMoreCarsList(wrapper.vm.buildCarsPage).catch(() => {});
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.carsData.map((c) => c.id)).toEqual([1]);
    expect(wrapper.vm.carsPage).toBe(1);
    expect(wrapper.vm.listError).toBe(true);
    expect(wrapper.find('[data-testid="cars-scroll-sentinel-error"]').exists()).toBe(true);

    const callsAfterFailure = getUniqueCarsPaginated.mock.calls.length;
    await wrapper.vm.loadMoreCarsList(wrapper.vm.buildCarsPage);
    expect(getUniqueCarsPaginated.mock.calls.length).toBe(callsAfterFailure);
  });
});
