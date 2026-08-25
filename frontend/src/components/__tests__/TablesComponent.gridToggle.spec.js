import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import TablesComponent from '../TablesComponent.vue';
import CarsTable from '../CarsTable.vue';
import FactTable from '../FactTable.vue';

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

function mountPage(tableName = 'kpp_4') {
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
        BaseDropdown: true,
        DateFilter: true,
      },
      mocks: {
        $route: { params: { tableName } },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
      },
    },
  });
}

describe('TablesComponent - тумблер "Сетка" (#1289)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return Promise.resolve(okResponse(TABLE_RESPONSE));
      if (url.startsWith('/users/me')) return Promise.resolve(okResponse({ id: 1, username: 'test' }));
      return Promise.resolve(okResponse([]));
    });
  });

  it('по умолчанию выключен, таблицы получают grid=false', async () => {
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.getComponent(CarsTable).props('grid')).toBe(false);
    expect(wrapper.getComponent(FactTable).props('grid')).toBe(false);
  });

  it('тумблер таблицы включает сетку и в таблице по факту, состояние в localStorage', async () => {
    const wrapper = mountPage();
    await flushPromises();

    // Тумблер живёт в шапке основной таблицы и поднимает состояние на страницу.
    wrapper.getComponent(CarsTable).vm.$emit('update:grid', true);
    await flushPromises();

    expect(wrapper.getComponent(CarsTable).props('grid')).toBe(true);
    expect(wrapper.getComponent(FactTable).props('grid')).toBe(true);
    expect(localStorage.getItem('grid-mode:kpp_4')).toBe('1');
  });

  it('выключение сбрасывает флаг в localStorage', async () => {
    localStorage.setItem('grid-mode:kpp_4', '1');
    const wrapper = mountPage();
    await flushPromises();

    wrapper.getComponent(CarsTable).vm.$emit('update:grid', false);
    await flushPromises();

    expect(wrapper.getComponent(CarsTable).props('grid')).toBe(false);
    expect(wrapper.getComponent(FactTable).props('grid')).toBe(false);
    expect(localStorage.getItem('grid-mode:kpp_4')).toBe('0');
  });

  it('состояние восстанавливается из localStorage при открытии страницы', async () => {
    localStorage.setItem('grid-mode:kpp_4', '1');
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.vm.gridMode).toBe(true);
    expect(wrapper.getComponent(CarsTable).props('grid')).toBe(true);
  });

  it('состояние хранится отдельно по имени таблицы', async () => {
    localStorage.setItem('grid-mode:kpp_4', '1');
    const wrapper = mountPage('post_72');
    await flushPromises();

    expect(wrapper.vm.gridMode).toBe(false);
  });
});
