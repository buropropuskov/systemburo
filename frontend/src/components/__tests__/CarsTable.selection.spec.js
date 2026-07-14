import { describe, it, expect, beforeEach, vi } from 'vitest';
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

import CarsTable from '../CarsTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function baseItem(overrides) {
  return {
    id: 1,
    car_number: 'А1',
    car_brand: 'BMW',
    plateNumber: 'А1',
    mark: 'BMW',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    entry_time_from: '08:00',
    entry_time_to: '18:00',
    unloadPlaces: [],
    organization_name: 'ООО',
    ...overrides,
  };
}

function mountTable() {
  return mount(CarsTable, {
    props: {
      tableName: 'КПП 1',
      tableId: 42,
      currentUserId: 1,
      currentUserName: 'Тест',
    },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('CarsTable - ctrl/shift-выделение строк (#1194 S3)', () => {
  let items;

  beforeEach(async () => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    // Минимальные ответы, чтобы _loadData не падал (как в CarsTable.enlarged.spec.js).
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/unload-places')) return okResponse([]);
      if (url.startsWith('/license-plate-formats')) return okResponse([]);
      if (url.startsWith('/cars/active-for-table/')) return okResponse([]);
      if (url.startsWith('/cars/unload-places')) return okResponse([]);
      if (url.startsWith('/cars/history/current-status')) return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/notifications/deletion-settings')) return okResponse({});
      return okResponse({});
    });
    items = [
      baseItem({ id: 1, car_number: 'А1' }),
      baseItem({ id: 2, car_number: 'А2' }),
      baseItem({ id: 3, car_number: 'А3' }),
      baseItem({ id: 4, car_number: 'А4' }),
    ];
  });

  async function mountWithItems() {
    const wrapper = mountTable();
    await flushPromises();
    // Загрузка через API замокана пустой - строки для клик-теста подставляем
    // напрямую в itemsData (displayItems - чистый computed поверх него).
    await wrapper.setData({ itemsData: items });
    return wrapper;
  }

  it('обычный клик по строке открывает деталь машины, не выделяет', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click');

    expect(wrapper.vm.showVehicleDetails).toBe(true);
    expect(wrapper.vm.selectedIds).toHaveLength(0);
  });

  it('ctrl-клик выделяет строку и не открывает деталь', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[1].trigger('click', { ctrlKey: true });

    expect(wrapper.vm.showVehicleDetails).toBe(false);
    expect(wrapper.vm.selectedIds).toEqual([2]);
    expect(rows[1].classes()).toContain('item-row--selected');
  });

  it('повторный ctrl-клик снимает выделение строки', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[1].trigger('click', { ctrlKey: true });
    await rows[1].trigger('click', { ctrlKey: true });

    expect(wrapper.vm.selectedIds).toHaveLength(0);
  });

  it('shift-клик выделяет диапазон от якоря до цели', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click', { ctrlKey: true }); // якорь = 1
    await rows[2].trigger('click', { shiftKey: true }); // диапазон 1..3

    expect(wrapper.vm.selectedIds.slice().sort()).toEqual([1, 2, 3]);
  });

  it('bulk-bar появляется при выделении и показывает счётчик', async () => {
    const wrapper = await mountWithItems();
    expect(wrapper.find('[data-testid="cars-bulk-bar"]').exists()).toBe(false);

    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click', { ctrlKey: true });
    await rows[1].trigger('click', { ctrlKey: true });

    const bar = wrapper.find('[data-testid="cars-bulk-bar"]');
    expect(bar.exists()).toBe(true);
    expect(bar.text()).toContain('Выбрано: 2');
  });

  it('"Снять выбор" очищает выделение и убирает bulk-bar', async () => {
    const wrapper = await mountWithItems();
    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click', { ctrlKey: true });

    await wrapper.find('[data-testid="cars-bulk-clear"]').trigger('click');

    expect(wrapper.vm.selectedIds).toHaveLength(0);
    expect(wrapper.find('[data-testid="cars-bulk-bar"]').exists()).toBe(false);
  });
});
