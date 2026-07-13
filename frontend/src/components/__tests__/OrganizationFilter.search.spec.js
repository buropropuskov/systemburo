import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import OrganizationFilter from '../OrganizationFilter.vue';

const ORGS = [
  { id: 1, name: 'Ромашка' },
  { id: 2, name: 'Восток' },
];

function mountFilter(props = {}) {
  return mount(OrganizationFilter, {
    props: { organizations: ORGS, ...props },
  });
}

describe('OrganizationFilter - поиск через общий util searchVariants (#1157)', () => {
  it('без запроса показывает все организации', () => {
    const wrapper = mountFilter();
    expect(wrapper.vm.filteredOrganizations).toHaveLength(2);
  });

  it('поиск матчит по варианту раскладки - EN-ввод находит кириллицу', async () => {
    const wrapper = mountFilter();
    // "hjvfirf" на EN-раскладке физически совпадает с "ромашка" на RU.
    wrapper.vm.searchQuery = 'hjvfirf';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredOrganizations.map((o) => o.id)).toEqual([1]);
  });

  it('пустой поисковый запрос снова показывает все организации', async () => {
    const wrapper = mountFilter();
    wrapper.vm.searchQuery = 'hjvfirf';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredOrganizations).toHaveLength(1);

    wrapper.vm.searchQuery = '';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredOrganizations).toHaveLength(2);
  });
});
