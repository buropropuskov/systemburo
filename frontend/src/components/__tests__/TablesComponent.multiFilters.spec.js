import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { ref, nextTick } from 'vue';
import fs from 'node:fs';
import path from 'node:path';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

const isNarrowRef = ref(false);
vi.mock('@/composables/useNarrowScreen', () => ({
  useNarrowScreen: () => ({ isNarrow: isNarrowRef }),
}));

import TablesComponent from '../TablesComponent.vue';
import CarsTable from '../CarsTable.vue';
import PeopleTable from '../PeopleTable.vue';
import FactTable from '../FactTable.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const ORGANIZATIONS = [{ id: 1, name: 'Ромашка' }, { id: 2, name: 'Восток' }];
const COMPANIES = [{ id: 10, name: 'Компания А' }];
// Архивное место должно отсеиваться: фильтровать по выведенной площадке смысла нет.
const UNLOAD_PLACES = [
  { id: 100, name: 'Ворота 1', description: 'VR-01', is_active: true },
  { id: 200, name: 'Ворота 2', description: 'VR-02' },
  { id: 300, name: 'Старые ворота', description: 'VR-03', is_active: false },
];

function tableResponse(tableType) {
  return {
    table: {
      id: 42,
      name: 'kpp_4',
      display_name: 'КПП №4',
      table_type: tableType,
      show_fact_table: true,
    },
  };
}

function mockApi(tableType = 'cars') {
  apiRequest.mockReset();
  apiRequest.mockImplementation((url) => {
    if (url.startsWith('/system-tables/name/')) return Promise.resolve(okResponse(tableResponse(tableType)));
    if (url.startsWith('/users/me')) return Promise.resolve(okResponse({ id: 1, username: 'test' }));
    if (url === '/organizations') return Promise.resolve(okResponse(ORGANIZATIONS));
    if (url === '/companies') return Promise.resolve(okResponse(COMPANIES));
    if (url === '/unload-places') return Promise.resolve(okResponse(UNLOAD_PLACES));
    return Promise.resolve(okResponse([]));
  });
}

function mountPage() {
  return mount(TablesComponent, {
    global: {
      stubs: {
        teleport: true,
        transition: false,
        'transition-group': false,
        RouterLink: true,
        CarsTable: true,
        PeopleTable: true,
        FactTable: true,
        ApplicationDetail: true,
        TableExportModal: true,
        ManualAddModal: true,
        PassReportModal: true,
        DateFilter: true,
        // BaseDropdown живой: проверяется проводка конфиг -> дропдаун -> setMultiFilter.
      },
      mocks: {
        $route: { params: { tableName: 'kpp_4' } },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
      },
    },
  });
}

function testids(wrapper) {
  return wrapper.findAllComponents(BaseDropdown).map(d => d.attributes('data-testid'));
}

describe('TablesComponent - мультивыбор фильтров справочников (#1398)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    isNarrowRef.value = false;
    mockApi('cars');
  });

  it('cars: три фильтра с прежними data-testid, опции из справочников', async () => {
    const wrapper = mountPage();
    await flushPromises();

    expect(testids(wrapper)).toEqual(['table-sheet-org', 'table-sheet-company', 'table-sheet-place']);

    const [org, company, place] = wrapper.findAllComponents(BaseDropdown);
    expect(org.props('options')).toEqual(ORGANIZATIONS);
    expect(org.props('placeholder')).toBe('Все организации');
    expect(org.props('summaryLabel')).toBe('Организация');
    expect(org.props('multiple')).toBe(true);
    expect(company.props('options')).toEqual(COMPANIES);
    // Код площадки лежит в description - без searchKeys поиск по нему сузился бы.
    expect(place.props('searchKeys')).toEqual(['name', 'description']);
  });

  it('архивные места разгрузки в опции не попадают', async () => {
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.vm.unloadPlaces.map(p => p.id)).toEqual([100, 200]);
  });

  it('people: фильтра места разгрузки нет, справочник не запрашивается', async () => {
    mockApi('people');
    const wrapper = mountPage();
    await flushPromises();

    expect(testids(wrapper)).toEqual(['table-sheet-org', 'table-sheet-company']);
    expect(apiRequest.mock.calls.some(([url]) => url === '/unload-places')).toBe(false);
  });

  it('выбор в дропдауне пишет массив в state и уезжает в дочерние таблицы', async () => {
    const wrapper = mountPage();
    await flushPromises();

    const [org] = wrapper.findAllComponents(BaseDropdown);
    org.vm.$emit('update:modelValue', [1, 2]);
    await nextTick();

    expect(wrapper.vm.selectedOrganizationIds).toEqual([1, 2]);
    expect(wrapper.getComponent(CarsTable).props('selectedOrganizationIds')).toEqual([1, 2]);
    expect(wrapper.getComponent(FactTable).props('selectedOrganizationIds')).toEqual([1, 2]);
    // Дропдаун получает свой выбор обратно пропом - своего состояния у него нет.
    expect(wrapper.findAllComponents(BaseDropdown)[0].props('modelValue')).toEqual([1, 2]);
  });

  it('фильтр места разгрузки доезжает до fact-таблицы (до #1398 не работал никогда)', async () => {
    const wrapper = mountPage();
    await flushPromises();

    const place = wrapper.findAllComponents(BaseDropdown)[2];
    place.vm.$emit('update:modelValue', [100]);
    await nextTick();

    expect(wrapper.getComponent(FactTable).props('selectedUnloadingPlaceIds')).toEqual([100]);
    expect(wrapper.getComponent(CarsTable).props('selectedUnloadingPlaceIds')).toEqual([100]);
  });

  it('people-таблица места разгрузки не получает', async () => {
    mockApi('people');
    const wrapper = mountPage();
    await flushPromises();

    const people = wrapper.getComponent(PeopleTable);
    expect(Object.keys(people.props())).not.toContain('selectedUnloadingPlaceIds');
    expect(people.props('selectedOrganizationIds')).toEqual([]);
  });

  it('setMultiFilter на мусорном значении не роняет state в не-массив', async () => {
    const wrapper = mountPage();
    await flushPromises();

    wrapper.vm.setMultiFilter('selectedCompanyIds', null);
    expect(wrapper.vm.selectedCompanyIds).toEqual([]);
  });

  it('clearFilters обнуляет все три набора, справочник опций остаётся', async () => {
    const wrapper = mountPage();
    await flushPromises();
    await wrapper.setData({
      selectedOrganizationIds: [1],
      selectedCompanyIds: [10],
      selectedUnloadingPlaceIds: [100],
    });

    wrapper.vm.clearFilters();
    await nextTick();

    expect(wrapper.vm.selectedOrganizationIds).toEqual([]);
    expect(wrapper.vm.selectedCompanyIds).toEqual([]);
    expect(wrapper.vm.selectedUnloadingPlaceIds).toEqual([]);
    expect(wrapper.vm.unloadPlaces).toHaveLength(2);
    expect(wrapper.vm.organizations).toEqual(ORGANIZATIONS);
  });

  // Состав фильтров задаётся конфигом, поэтому десктопный ряд и мобильный лист
  // физически не могут разойтись.
  it('мобильный лист показывает тот же состав фильтров, что десктопный ряд', async () => {
    const desktop = mountPage();
    await flushPromises();
    const desktopIds = testids(desktop);

    isNarrowRef.value = true;
    const mobile = mountPage();
    await flushPromises();
    await mobile.setData({ showFilterSheet: true });

    expect(testids(mobile)).toEqual(desktopIds);
  });
});

// jsdom раскладку не считает, поэтому геометрию ряда охраняем чтением SFC. Замер в
// браузере на прод-сборке: с `min-width: 120px` на .filters__control ряд (он nowrap)
// переставал сжиматься и наезжал на правую группу кнопок - 21px на 1100 и 265px на 769.
describe('TablesComponent - CSS-контракт ряда фильтров (#1398)', () => {
  const sfc = fs.readFileSync(path.resolve(__dirname, '../TablesComponent.vue'), 'utf8');

  function ruleBody(selector) {
    const at = sfc.indexOf(selector + ' {');
    expect(at, `правило ${selector} не найдено`).toBeGreaterThan(-1);
    return sfc.slice(at, sfc.indexOf('}', at));
  }

  it('.filters__control сжимаем: min-width 0, без пиксельного пола', () => {
    const body = ruleBody('.filters__control');
    expect(body).toMatch(/min-width:\s*0\s*;/);
    expect(body).not.toMatch(/min-width:\s*\d*[1-9]\d*px/);
    expect(body).toMatch(/flex-shrink:\s*1\s*;/);
  });

  // Собственные 30px BaseDropdown перебивают низ clamp, если не занулить min-height.
  it('кнопка дропдауна приведена к контракту ряда через :deep', () => {
    const body = ruleBody('.filters__control :deep(.base-dropdown__button)');
    expect(body).toMatch(/min-height:\s*0\s*;/);
    expect(body).toMatch(/height:\s*clamp\(28px,\s*3vw,\s*35px\)/);
    expect(body).toMatch(/border-radius:\s*15px/);
  });

  it('мобильный лист: дропдаун 40px как календарь рядом', () => {
    const body = ruleBody('.filter-section :deep(.base-dropdown__button)');
    expect(body).toMatch(/height:\s*40px/);
    expect(body).toMatch(/min-height:\s*0\s*;/);
  });
});
