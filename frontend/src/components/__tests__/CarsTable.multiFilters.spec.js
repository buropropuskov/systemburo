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

function car(overrides) {
  return {
    id: 1,
    car_number: 'А1',
    car_brand: 'BMW',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    entry_time_from: '08:00',
    entry_time_to: '18:00',
    unloadPlaces: [],
    organization_id: 1,
    organization_name: 'Ромашка',
    company_id: 10,
    company: 'Компания А',
    ...overrides,
  };
}

const ITEMS = [
  car({ id: 1, car_number: 'А1', organization_id: 1, company_id: 10 }),
  car({ id: 2, car_number: 'А2', organization_id: 2, company_id: 11 }),
  car({ id: 3, car_number: 'А3', organization_id: 3, company_id: 10 }),
];

// Места разгрузки живут отдельной картой car_id -> [place], а не полем строки.
const PLACES_MAP = {
  1: [{ id: 100, name: 'Ворота 1' }],
  2: [{ id: 100, name: 'Ворота 1' }, { id: 200, name: 'Ворота 2' }],
  3: [{ id: 300, name: 'Ворота 3' }],
};

function mountTable(props = {}) {
  return mount(CarsTable, {
    props: {
      tableName: 'КПП 1',
      tableId: 42,
      currentUserId: 1,
      currentUserName: 'Тест',
      ...props,
    },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

async function mountWithItems(props = {}) {
  const wrapper = mountTable(props);
  await flushPromises();
  await wrapper.setData({ itemsData: ITEMS, carUnloadPlacesMap: PLACES_MAP });
  return wrapper;
}

function numbers(wrapper) {
  return wrapper.vm.displayItems.map(i => i.car_number);
}

describe('CarsTable - мультивыбор фильтров (#1398)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
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
  });

  it('пропсы не переданы (preview-монтирование) - фильтры не режут строки', async () => {
    const wrapper = await mountWithItems();
    expect(numbers(wrapper)).toEqual(['А1', 'А2', 'А3']);
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });

  it('пустой массив не фильтрует', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [] });
    expect(numbers(wrapper)).toEqual(['А1', 'А2', 'А3']);
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });

  it('несколько организаций дают объединение', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [1, 3] });
    expect(numbers(wrapper)).toEqual(['А1', 'А3']);
    expect(wrapper.vm.hasActiveFilters).toBe(true);
  });

  it('разные фильтры пересекаются по AND', async () => {
    const wrapper = await mountWithItems({
      selectedOrganizationIds: [1, 3],
      selectedCompanyIds: [10],
    });
    expect(numbers(wrapper)).toEqual(['А1', 'А3']);

    await wrapper.setProps({ selectedCompanyIds: [11] });
    expect(numbers(wrapper)).toEqual([]);
  });

  // Строки таблицы и справочник приходят разными запросами, типы id совпадать не
  // обязаны - без String() в предикате этот кейс краснеет.
  it('строковые id из дропдауна матчат числовые из строк таблицы', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: ['1'] });
    expect(numbers(wrapper)).toEqual(['А1']);
  });

  it('место разгрузки: машина проходит, если есть хотя бы одно из выбранных', async () => {
    const wrapper = await mountWithItems({ selectedUnloadingPlaceIds: [200, 300] });
    // У А2 есть Ворота 2, у А3 - Ворота 3; у А1 только Ворота 1.
    expect(numbers(wrapper)).toEqual(['А2', 'А3']);
  });

  it('машина без записей в карте мест разгрузки отсеивается', async () => {
    const wrapper = await mountWithItems({ selectedUnloadingPlaceIds: [100] });
    // У А4 в карте carUnloadPlacesMap записей нет вовсе.
    await wrapper.setData({ itemsData: [...ITEMS, car({ id: 4, car_number: 'А4' })] });
    expect(numbers(wrapper)).toEqual(['А1', 'А2']);
  });

  it('организация с id=0 не проваливается как falsy', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [0] });
    await wrapper.setData({ itemsData: [car({ id: 9, car_number: 'А9', organization_id: 0 })] });
    expect(numbers(wrapper)).toEqual(['А9']);
    expect(wrapper.vm.hasActiveFilters).toBe(true);
  });
});
