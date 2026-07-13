import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import CarsFact from '../CarsFact.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const ITEMS = [
  { organization: 'Ромашка', unload_places: [], status: 'В работе', entry_time_from: '', entry_time_to: '', entry_date_to: '' },
  { organization: 'Восток', unload_places: [], status: 'В работе', entry_time_from: '', entry_time_to: '', entry_date_to: '' },
];

describe('CarsFact - поиск через общий util searchVariants (#1157)', () => {
  beforeEach(() => {
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.includes('/unload-places')) return Promise.resolve(okResponse([]));
      if (url.includes('/applications/active-cars')) return Promise.resolve(okResponse([]));
      return Promise.resolve(okResponse([]));
    });
  });

  it('поиск матчит по варианту раскладки - EN-ввод находит кириллицу организации', async () => {
    const wrapper = mount(CarsFact, { props: { searchQuery: '' } });
    await flushPromises();
    wrapper.vm.carsData = ITEMS;
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredCars).toHaveLength(2);

    // "hjvfirf" на EN-раскладке физически совпадает с "ромашка" на RU.
    await wrapper.setProps({ searchQuery: 'hjvfirf' });
    expect(wrapper.vm.filteredCars.map((c) => c.organization)).toEqual(['Ромашка']);

    wrapper.unmount();
  });

  it('пустой поисковый запрос снова показывает все строки', async () => {
    const wrapper = mount(CarsFact, { props: { searchQuery: '' } });
    await flushPromises();
    wrapper.vm.carsData = ITEMS;
    await wrapper.vm.$nextTick();

    await wrapper.setProps({ searchQuery: 'hjvfirf' });
    expect(wrapper.vm.filteredCars).toHaveLength(1);

    await wrapper.setProps({ searchQuery: '' });
    expect(wrapper.vm.filteredCars).toHaveLength(2);

    wrapper.unmount();
  });
});
