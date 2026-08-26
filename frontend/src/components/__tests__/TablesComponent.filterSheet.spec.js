import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { ref, nextTick } from 'vue';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

// Управляем «узким экраном» из тестов: setup() зовёт useNarrowScreen один раз и
// возвращает этот же ref, так что менять isNarrowRef.value можно и после mount.
const isNarrowRef = ref(false);
vi.mock('@/composables/useNarrowScreen', () => ({
  useNarrowScreen: () => ({ isNarrow: isNarrowRef }),
}));

import TablesComponent from '../TablesComponent.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import FilterSheet from '@/components/ui/FilterSheet.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const TABLE_RESPONSE = {
  table: {
    id: 42,
    name: 'kpp_4',
    display_name: 'КПП №4',
    table_type: 'cars',
    show_fact_table: true,
  },
};

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
        // BaseDropdown НЕ стабим: он нужен живым, чтобы проверялась проводка
        // directoryFilters -> дропдаун -> setMultiFilter.
        DateFilter: true,
      },
      mocks: {
        $route: { params: { tableName: 'kpp_4' } },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
      },
    },
  });
}

describe('TablesComponent - сворачивание фильтров в кнопку «Фильтр» на мобилке (#1097 S2)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    isNarrowRef.value = false;
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return Promise.resolve(okResponse(TABLE_RESPONSE));
      if (url.startsWith('/users/me')) return Promise.resolve(okResponse({ id: 1, username: 'test' }));
      return Promise.resolve(okResponse([]));
    });
  });

  it('десктоп: вторичные фильтры инлайн, кнопки «Фильтр» и sheet нет', async () => {
    const wrapper = mountPage();
    await flushPromises();

    // cars-таблица: организация + компания + место разгрузки инлайн.
    expect(wrapper.findAllComponents(BaseDropdown).length).toBe(3);
    expect(wrapper.findComponent(FilterButton).exists()).toBe(false);
    expect(wrapper.findComponent(FilterSheet).exists()).toBe(false);
  });

  it('мобилка: инлайн-фильтры свёрнуты, видна кнопка «Фильтр», sheet смонтирован', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    // Инлайн вторичные фильтры не рендерятся (sheet закрыт - контент BaseModal под v-if show).
    expect(wrapper.findAllComponents(BaseDropdown).length).toBe(0);
    expect(wrapper.find('[data-testid="table-filter-btn"]').exists()).toBe(true);
    expect(wrapper.findComponent(FilterSheet).exists()).toBe(true);
    // Поиск остаётся снаружи sheet - с #1097 S7 он свёрнут в иконку-тоггл рядом
    // с «Фильтром», а поле раскрывается оверлеем (см. TablesComponent.searchOverlay.spec.js).
    expect(wrapper.find('[data-testid="table-search-icon"]').exists()).toBe(true);
  });

  it('клик по кнопке «Фильтр» открывает sheet', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.vm.showFilterSheet).toBe(false);
    await wrapper.find('[data-testid="table-filter-btn"]').trigger('click');
    expect(wrapper.vm.showFilterSheet).toBe(true);
  });

  it('точка-индикатор на кнопке зажигается только по вторичным фильтрам, не по поиску', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.findComponent(FilterButton).props('active')).toBe(false);

    // Поиск не зажигает точку (он снаружи, виден отдельно).
    await wrapper.setData({ searchQuery: 'абв' });
    expect(wrapper.findComponent(FilterButton).props('active')).toBe(false);

    // Вторичный фильтр зажигает.
    await wrapper.setData({ selectedOrganizationIds: [7] });
    expect(wrapper.findComponent(FilterButton).props('active')).toBe(true);
  });

  it('reset из sheet сбрасывает и поиск, и вторичные фильтры', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    await wrapper.setData({
      showFilterSheet: true,
      searchQuery: 'абв',
      selectedOrganizationIds: [7],
      selectedCompanyIds: [2],
      selectedUnloadingPlaceIds: [3],
      selectedDate: '2026-07-26',
    });

    wrapper.findComponent(FilterSheet).vm.$emit('reset');
    await nextTick();

    expect(wrapper.vm.searchQuery).toBe('');
    expect(wrapper.vm.selectedOrganizationIds).toEqual([]);
    expect(wrapper.vm.selectedCompanyIds).toEqual([]);
    expect(wrapper.vm.selectedUnloadingPlaceIds).toEqual([]);
    expect(wrapper.vm.selectedDate).toBeNull();
    expect(wrapper.vm.hasActiveFilters).toBe(false);
  });

  // Сжатие «Инструкции» в иконку - только мобилка по замеру ширины (jsdom layout
  // не считает, поэтому проверяем инвариант десктопа: там сжатия быть не должно).
  it('measureHeader: на десктопе инструкция не сжимается в иконку', async () => {
    isNarrowRef.value = false;
    const wrapper = mountPage();
    await flushPromises();

    wrapper.vm.instructionCompact = true;
    wrapper.vm.measureHeader();
    await nextTick();

    expect(wrapper.vm.instructionCompact).toBe(false);
  });
});
