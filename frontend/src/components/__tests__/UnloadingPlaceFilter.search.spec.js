import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import UnloadingPlaceFilter from '../UnloadingPlaceFilter.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

const PLACES = [
  { id: 1, name: 'Ромашка', description: 'склад №1' },
  { id: 2, name: 'Восток', description: 'ворота 2' },
];

describe('UnloadingPlaceFilter - поиск через общий util searchVariants (#1157)', () => {
  beforeEach(() => {
    apiRequest.mockReset();
    apiRequest.mockImplementation(() => Promise.resolve(okResponse([])));
  });

  it('без запроса показывает все места разгрузки', async () => {
    const wrapper = mount(UnloadingPlaceFilter);
    await flushPromises();
    wrapper.vm.unloadingPlaces = PLACES;
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredPlaces).toHaveLength(2);
  });

  it('поиск матчит по варианту раскладки - EN-ввод находит кириллицу названия', async () => {
    const wrapper = mount(UnloadingPlaceFilter);
    await flushPromises();
    wrapper.vm.unloadingPlaces = PLACES;
    await wrapper.vm.$nextTick();

    // "hjvfirf" на EN-раскладке физически совпадает с "ромашка" на RU.
    wrapper.vm.searchQuery = 'hjvfirf';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredPlaces.map((p) => p.id)).toEqual([1]);
  });

  it('пустой поисковый запрос снова показывает все места разгрузки', async () => {
    const wrapper = mount(UnloadingPlaceFilter);
    await flushPromises();
    wrapper.vm.unloadingPlaces = PLACES;
    await wrapper.vm.$nextTick();

    wrapper.vm.searchQuery = 'hjvfirf';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredPlaces).toHaveLength(1);

    wrapper.vm.searchQuery = '';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredPlaces).toHaveLength(2);
  });
});
