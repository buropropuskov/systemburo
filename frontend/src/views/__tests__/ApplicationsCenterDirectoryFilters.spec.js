import {
  describe, it, expect, beforeEach, afterEach, vi,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { apiRequest } from '@/api/client';
import { getApplicationsPaginated } from '@/api/applications';
import { useAuthStore } from '@/stores/auth';

// Справочники фильтров «Места разгрузки» и «Проход» (#1398). Форма ответа у этих
// эндпоинтов РАЗНАЯ, и это ловится только тестом на реальных фигурах данных:
// /unload-places отдаёт плоские записи, а /system-tables - SystemTableWithDetails,
// где сама таблица лежит во вложенном table. Регресс: без разворачивания пункты
// дропдауна «Проход» рендерились с пустыми подписями.

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({ getApplicationsPaginated: vi.fn(), getApplicationById: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn(), SOUND_PRESETS: [] }));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(() => () => {}), onStatus: vi.fn(() => () => {}),
  },
}));

const stubs = {
  teleport: true,
  RefreshButton: true,
  ApplicationDetail: true,
  DateFilter: true,
  FilterTabs: true,
  SkeletonTransition: { template: '<div><slot /></div>' },
  SkeletonTable: true,
  LoaderSpinner: true,
  DownloadBlanksModal: true,
  ApplicationsFilterModal: true,
  Badge: true,
  BaseDropdown: true,
};

const okJson = (data) => ({ ok: true, json: async () => data });

function mockDirectories() {
  apiRequest.mockImplementation((url) => {
    if (url === '/organizations') return Promise.resolve(okJson([{ id: 1, name: 'Орг' }]));
    if (url === '/companies') return Promise.resolve(okJson([{ id: 2, name: 'Компания' }]));
    if (url === '/unload-places') {
      return Promise.resolve(okJson([
        { id: 10, name: 'Ворота 1', is_active: true },
        { id: 11, name: 'Архивное место', is_active: false },
      ]));
    }
    if (url === '/system-tables') {
      return Promise.resolve(okJson([
        { table: { id: 20, name: 'post72', display_name: 'ПОСТ №72', is_active: true }, fields: [] },
        { table: { id: 21, name: 'kpp4', display_name: null, is_active: true }, fields: [] },
        { table: { id: 22, name: 'old', display_name: 'Архивная', is_active: false }, fields: [] },
      ]));
    }
    return Promise.resolve({ ok: false, json: async () => ({}), text: async () => '' });
  });
}

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

let wrapper;

describe('ApplicationsCenter: справочники фильтров (#1398)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    getApplicationsPaginated.mockReset();
    getApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
    mockDirectories();
    useAuthStore().token = 'test-token';
  });

  afterEach(() => wrapper?.unmount());

  it('таблицы проходной разворачиваются из вложенного table и получают читаемую подпись', async () => {
    wrapper = mountCenter();
    await flushPromises();

    expect(wrapper.vm.passageTables.map((t) => ({ id: t.id, name: t.name }))).toEqual([
      { id: 20, name: 'ПОСТ №72' },
      // display_name пустой - падаем на служебное name, но НЕ на undefined
      { id: 21, name: 'kpp4' },
    ]);
  });

  it('места разгрузки читаются плоскими, архивные отсеиваются', async () => {
    wrapper = mountCenter();
    await flushPromises();

    expect(wrapper.vm.unloadPlaces.map((p) => p.id)).toEqual([10]);
  });

  it('конфиг фильтров отдаёт загруженные справочники в дропдауны', async () => {
    wrapper = mountCenter();
    await flushPromises();

    const byField = Object.fromEntries(wrapper.vm.directoryFilters.map((f) => [f.field, f]));
    expect(byField.selectedUnloadPlaceIds.options).toHaveLength(1);
    expect(byField.selectedPassageTableIds.options).toHaveLength(2);
    expect(byField.selectedPassageTableIds.summaryLabel).toBe('Проход');
  });

  it('setMultiFilter пишет выбор и перезапрашивает список с бэка', async () => {
    wrapper = mountCenter();
    await flushPromises();
    getApplicationsPaginated.mockClear();

    wrapper.vm.setMultiFilter('selectedPassageTableIds', [20, 21]);
    await flushPromises();

    expect(wrapper.vm.selectedPassageTableIds).toEqual([20, 21]);
    expect(getApplicationsPaginated).toHaveBeenCalled();
    expect(getApplicationsPaginated.mock.calls[0][0].passage_table_ids).toBe('20,21');
  });

  it('сброс фильтров очищает все четыре справочных набора', async () => {
    wrapper = mountCenter();
    await flushPromises();
    wrapper.vm.selectedOrganizationIds = [1];
    wrapper.vm.selectedCompanyIds = [2];
    wrapper.vm.selectedUnloadPlaceIds = [10];
    wrapper.vm.selectedPassageTableIds = [20];
    expect(wrapper.vm.hasActiveFilters).toBe(true);

    wrapper.vm.resetFilters();
    await flushPromises();

    expect(wrapper.vm.selectedOrganizationIds).toEqual([]);
    expect(wrapper.vm.selectedCompanyIds).toEqual([]);
    expect(wrapper.vm.selectedUnloadPlaceIds).toEqual([]);
    expect(wrapper.vm.selectedPassageTableIds).toEqual([]);
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });
});
