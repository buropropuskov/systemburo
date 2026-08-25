import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { ref, nextTick } from 'vue';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

// Тот же приём, что в TablesComponent.filterSheet.spec.js: setup() зовёт
// useNarrowScreen один раз, поэтому подменённый ref можно двигать и после mount.
const isNarrowRef = ref(false);
vi.mock('@/composables/useNarrowScreen', () => ({
  useNarrowScreen: () => ({ isNarrow: isNarrowRef }),
}));

import TablesComponent from '../TablesComponent.vue';

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
    attachTo: document.body,
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
        BaseDropdown: true,
        DateFilter: true,
      },
      mocks: {
        $route: { params: { tableName: 'kpp_4' } },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
      },
    },
  });
}

const DESKTOP_SEARCH = '.filters__fields > .field.search input.field__input';
const OVERLAY_SEARCH = '[data-testid="table-input-search"]';

// Оверлей снимается через <Transition>: до конца leave элемент остаётся в DOM.
// Длительности в jsdom нулевые, но снятие всё равно уезжает за пару кадров rAF.
const settleTransition = () => new Promise((resolve) => setTimeout(resolve, 50));

describe('TablesComponent - поиск иконкой с оверлеем на мобилке (#1097 S7)', () => {
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

  it('десктоп: поле поиска инлайн, иконки-тоггла и оверлея нет', async () => {
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find(DESKTOP_SEARCH).exists()).toBe(true);
    expect(wrapper.find('[data-testid="table-search-icon"]').exists()).toBe(false);
    expect(wrapper.find(OVERLAY_SEARCH).exists()).toBe(false);

    wrapper.unmount();
  });

  it('мобилка: инлайн-поля нет, есть иконка, поле появляется только после клика', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find(DESKTOP_SEARCH).exists()).toBe(false);
    const icon = wrapper.find('[data-testid="table-search-icon"]');
    expect(icon.exists()).toBe(true);
    expect(wrapper.find(OVERLAY_SEARCH).exists()).toBe(false);

    await icon.trigger('click');
    expect(wrapper.vm.showMobileSearch).toBe(true);
    expect(wrapper.find(OVERLAY_SEARCH).exists()).toBe(true);

    // Повторный клик сворачивает обратно.
    await icon.trigger('click');
    await settleTransition();
    expect(wrapper.find(OVERLAY_SEARCH).exists()).toBe(false);

    wrapper.unmount();
  });

  it('мобилка: фокус уходит в поле при раскрытии', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="table-search-icon"]').trigger('click');
    await nextTick();

    expect(document.activeElement).toBe(wrapper.find(OVERLAY_SEARCH).element);

    wrapper.unmount();
  });

  it('мобилка: иконка подсвечена, пока в поиске что-то введено, даже со свёрнутым полем', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('[data-testid="table-search-icon"]').classes())
      .not.toContain('search-icon-btn--active');

    await wrapper.setData({ searchQuery: 'абв' });
    expect(wrapper.find('[data-testid="table-search-icon"]').classes())
      .toContain('search-icon-btn--active');

    wrapper.unmount();
  });

  it('мобилка: крестик очищает запрос и закрывает оверлей', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="table-search-icon"]').trigger('click');
    await wrapper.setData({ searchQuery: 'абв' });

    await wrapper.find('.tables__search-clear').trigger('click');
    await settleTransition();

    expect(wrapper.vm.searchQuery).toBe('');
    expect(wrapper.vm.showMobileSearch).toBe(false);
    expect(wrapper.find(OVERLAY_SEARCH).exists()).toBe(false);

    wrapper.unmount();
  });

  it('крестика нет, пока поле пустое', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="table-search-icon"]').trigger('click');
    expect(wrapper.find('.tables__search-clear').exists()).toBe(false);

    wrapper.unmount();
  });

  it('возврат на десктоп гасит раскрытый оверлей и возвращает инлайн-поле', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="table-search-icon"]').trigger('click');
    expect(wrapper.vm.showMobileSearch).toBe(true);

    isNarrowRef.value = false;
    await nextTick();
    await settleTransition();

    expect(wrapper.vm.showMobileSearch).toBe(false);
    expect(wrapper.find(OVERLAY_SEARCH).exists()).toBe(false);
    expect(wrapper.find(DESKTOP_SEARCH).exists()).toBe(true);

    wrapper.unmount();
  });

  it('запрос переживает сворачивание поля и уходит в таблицы пропом', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    const icon = wrapper.find('[data-testid="table-search-icon"]');
    await icon.trigger('click');
    await wrapper.find(OVERLAY_SEARCH).setValue('вольво');
    await icon.trigger('click');

    expect(wrapper.vm.searchQuery).toBe('вольво');
    expect(wrapper.findComponent({ name: 'CarsTable' }).props('searchQuery')).toBe('вольво');

    wrapper.unmount();
  });
});
