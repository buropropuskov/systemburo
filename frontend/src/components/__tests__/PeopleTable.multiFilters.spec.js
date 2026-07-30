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

import PeopleTable from '../PeopleTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function person(overrides) {
  return {
    id: 1,
    last_name: 'Иванов',
    first_name: 'Иван',
    middle_name: 'Иванович',
    position: 'Водитель',
    citizenship_name: 'РФ',
    organization_id: 1,
    organization_name: 'Ромашка',
    company_id: 10,
    status: 'В работе',
    entry_date_to: '2026-06-05',
    pass_time: '08:00',
    ...overrides,
  };
}

const ITEMS = [
  person({ id: 1, last_name: 'Иванов', organization_id: 1, company_id: 10 }),
  person({ id: 2, last_name: 'Петров', organization_id: 2, company_id: 11 }),
  person({ id: 3, last_name: 'Сидоров', organization_id: 3, company_id: 10 }),
];

function mountTable(props = {}) {
  return mount(PeopleTable, {
    props: {
      tableName: 'КПП-72',
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
  await wrapper.setData({ itemsData: ITEMS });
  return wrapper;
}

function names(wrapper) {
  return wrapper.vm.displayItems.map(i => i.last_name);
}

describe('PeopleTable - мультивыбор фильтров (#1398)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return okResponse({ table: { id: 7 } });
      if (url.startsWith('/employees/active-for-table/')) return okResponse([]);
      if (url.startsWith('/employees/history/current-status')) return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/citizenships')) return okResponse([]);
      if (url.startsWith('/notifications/deletion-settings')) return okResponse({});
      return okResponse({});
    });
  });

  // Замок на удалённый мёртвый проп: родитель биндил `-id`, компонент объявлял String,
  // данных о местах разгрузки у сотрудников нет вовсе (#1398 S3).
  it('в пропсах нет никакого selectedUnloadingPlace*', () => {
    const propNames = Object.keys(PeopleTable.props);
    expect(propNames.filter(n => n.startsWith('selectedUnloadingPlace'))).toEqual([]);
  });

  it('пропсы не переданы (preview-монтирование) - фильтры не режут строки', async () => {
    const wrapper = await mountWithItems();
    expect(names(wrapper)).toEqual(['Иванов', 'Петров', 'Сидоров']);
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });

  it('пустой массив не фильтрует', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [], selectedCompanyIds: [] });
    expect(names(wrapper)).toEqual(['Иванов', 'Петров', 'Сидоров']);
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });

  it('несколько организаций дают объединение', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [1, 3] });
    expect(names(wrapper)).toEqual(['Иванов', 'Сидоров']);
    expect(wrapper.vm.hasActiveFilters).toBe(true);
  });

  it('организация и компания пересекаются по AND', async () => {
    const wrapper = await mountWithItems({
      selectedOrganizationIds: [1, 2],
      selectedCompanyIds: [11],
    });
    expect(names(wrapper)).toEqual(['Петров']);
  });

  it('строковые id из дропдауна матчат числовые из строк таблицы', async () => {
    const wrapper = await mountWithItems({ selectedCompanyIds: ['10'] });
    expect(names(wrapper)).toEqual(['Иванов', 'Сидоров']);
  });

  it('организация с id=0 не проваливается как falsy', async () => {
    const wrapper = await mountWithItems({ selectedOrganizationIds: [0] });
    await wrapper.setData({ itemsData: [person({ id: 9, last_name: 'Нулевой', organization_id: 0 })] });
    expect(names(wrapper)).toEqual(['Нулевой']);
    expect(wrapper.vm.hasActiveFilters).toBe(true);
  });
});
