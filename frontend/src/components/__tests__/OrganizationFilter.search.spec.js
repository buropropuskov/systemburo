import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';

// Управляем «узким экраном»: на мобилке меню телепортируется в body (вне $el).
const isNarrowRef = ref(false);
vi.mock('@/composables/useNarrowScreen', () => ({
  useNarrowScreen: () => ({ isNarrow: isNarrowRef }),
}));

import OrganizationFilter from '../OrganizationFilter.vue';

const ORGS = [
  { id: 1, name: 'Ромашка' },
  { id: 2, name: 'Восток' },
];

function mountFilter(props = {}, opts = {}) {
  return mount(OrganizationFilter, {
    props: { organizations: ORGS, ...props },
    ...opts,
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

describe('OrganizationFilter - клик-вовне под Teleport (мобилка)', () => {
  it('клик по телепортированному меню не закрывает дропдаун, клик снаружи закрывает', async () => {
    isNarrowRef.value = true;
    const wrapper = mountFilter({}, { attachTo: document.body });
    wrapper.vm.isOpen = true;
    await nextTick();
    // watch(isOpen) ставит clickOutsideHandler через setTimeout(0)
    await new Promise((r) => setTimeout(r, 5));

    const menu = wrapper.vm.$refs.menuRef;
    expect(menu).toBeTruthy();
    expect(wrapper.vm.clickOutsideHandler).toBeTypeOf('function');

    // Меню вне $el (телепортировано) - клик по нему НЕ должен закрывать.
    wrapper.vm.clickOutsideHandler({ target: menu });
    expect(wrapper.vm.isOpen).toBe(true);

    // Клик по постороннему узлу - закрывает.
    wrapper.vm.clickOutsideHandler({ target: document.body });
    expect(wrapper.vm.isOpen).toBe(false);

    isNarrowRef.value = false;
    wrapper.unmount();
  });
});
