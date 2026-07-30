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

import FactTable from '../FactTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function factRow(overrides) {
  return {
    id: 1,
    organization_id: 1,
    organization_name: 'Ромашка',
    company_id: 10,
    company: 'Компания А',
    car_brand: 'Toyota',
    status: 'В работе',
    entry_time_from: '',
    entry_time_to: '',
    pass_time: '',
    entry_date_to: '',
    ...overrides,
  };
}

const ITEMS = [
  factRow({ id: 1, organization_id: 1, company_id: 10 }),
  factRow({ id: 2, organization_id: 2, company_id: 11 }),
  factRow({ id: 3, organization_id: 3, company_id: 10 }),
];

const PLACES_MAP = {
  1: [{ id: 100, name: 'Ворота 1' }],
  2: [{ id: 100, name: 'Ворота 1' }, { id: 200, name: 'Ворота 2' }],
  3: [{ id: 300, name: 'Ворота 3' }],
};

function mountTable(props = {}) {
  return mount(FactTable, {
    props: { tableType: 'cars', tableId: 42, currentUserId: 7, currentUserName: 'Охр', ...props },
    global: {
      stubs: { teleport: true, transition: false, 'transition-group': false, FactPassModal: true },
    },
  });
}

async function mountWithItems(props = {}) {
  const wrapper = mountTable(props);
  await flushPromises();
  await wrapper.setData({ factData: ITEMS, factCarUnloadPlacesMap: PLACES_MAP });
  return wrapper;
}

function ids(wrapper) {
  return wrapper.vm.filteredData.map(i => i.id);
}

describe('FactTable - мультивыбор фильтров (#1398)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (/\/(unload-places|organizations|companies|marks)/.test(url)) return Promise.resolve(okResponse([]));
      if (url.includes('/cars/fact-for-table')) return Promise.resolve(okResponse([]));
      if (url.includes('/license-plate-formats')) return Promise.resolve(okResponse([]));
      return Promise.resolve(okResponse({}));
    });
  });

  it('пропсы не переданы - фильтры не режут строки', async () => {
    const wrapper = await mountWithItems();
    expect(ids(wrapper)).toEqual([1, 2, 3]);
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });

  it('несколько организаций дают объединение', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [1, 3] });
    expect(ids(wrapper)).toEqual([1, 3]);
    expect(wrapper.vm.hasActiveFilters).toBe(true);
  });

  it('организация и компания пересекаются по AND', async () => {
    const wrapper = await mountWithItems({
      selectedOrganizationIds: [1, 2],
      selectedCompanyIds: [11],
    });
    expect(ids(wrapper)).toEqual([2]);
  });

  it('строковые id из дропдауна матчат числовые из строк таблицы', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: ['2'] });
    expect(ids(wrapper)).toEqual([2]);
  });

  // Регресс-замок дефекта, который жил до #1398 S3: родитель отдавал ИМЯ места в
  // необъявленный проп `selected-unloading-place`, а filteredData читала
  // `selectedUnloadingPlaceId`, который снаружи не приходил никогда. Фильтр места
  // разгрузки в таблице «по факту» не работал ни разу.
  it('место разгрузки реально сужает строки таблицы по факту', async () => {
    const wrapper = await mountWithItems({ selectedUnloadingPlaceIds: [300] });
    expect(ids(wrapper)).toEqual([3]);
  });

  it('место разгрузки: строка проходит, если есть хотя бы одно из выбранных', async () => {
    const wrapper = await mountWithItems({ selectedUnloadingPlaceIds: [200, 300] });
    expect(ids(wrapper)).toEqual([2, 3]);
  });

  it('для people-таблицы фильтр места разгрузки игнорируется', async () => {
    const wrapper = await mountWithItems({
      tableType: 'people',
      selectedUnloadingPlaceIds: [300],
    });
    expect(ids(wrapper)).toEqual([1, 2, 3]);
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });

  it('организация с id=0 не проваливается как falsy', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [0] });
    await wrapper.setData({ factData: [factRow({ id: 9, organization_id: 0 })] });
    expect(ids(wrapper)).toEqual([9]);
    expect(wrapper.vm.hasActiveFilters).toBe(true);
  });
});
