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

function mountTable(props = {}) {
  return mount(FactTable, {
    props: { tableType: 'cars', tableId: 42, currentUserId: 7, currentUserName: 'Охр', ...props },
    global: {
      stubs: { teleport: true, transition: false, 'transition-group': false, FactPassModal: true },
    },
  });
}

const ITEMS = [
  { id: 1, organization_name: 'Ромашка', car_brand: 'Toyota', company: '', status: 'В работе', entry_time_from: '', entry_time_to: '', pass_time: '', entry_date_to: '' },
  { id: 2, organization_name: 'Восток', car_brand: 'BMW', company: '', status: 'В работе', entry_time_from: '', entry_time_to: '', pass_time: '', entry_date_to: '' },
];

describe('FactTable - поиск через общий util searchVariants (#1157)', () => {
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

  it('поиск матчит по варианту раскладки - EN-ввод находит кириллицу организации', async () => {
    const wrapper = mountTable();
    await flushPromises();
    wrapper.vm.factData = ITEMS;
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredData).toHaveLength(2);

    // "hjvfirf" на EN-раскладке физически совпадает с "ромашка" на RU.
    await wrapper.setProps({ searchQuery: 'hjvfirf' });
    expect(wrapper.vm.filteredData.map((i) => i.id)).toEqual([1]);
  });

  it('пустой поисковый запрос снова показывает все строки', async () => {
    const wrapper = mountTable();
    await flushPromises();
    wrapper.vm.factData = ITEMS;
    await wrapper.vm.$nextTick();

    await wrapper.setProps({ searchQuery: 'hjvfirf' });
    expect(wrapper.vm.filteredData).toHaveLength(1);

    await wrapper.setProps({ searchQuery: '' });
    expect(wrapper.vm.filteredData).toHaveLength(2);
  });
});
