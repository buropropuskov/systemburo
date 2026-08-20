import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { ref, nextTick } from 'vue';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

// Тот же приём, что в TablesComponent.filterSheet.spec.js: setup() зовёт
// useNarrowScreen один раз, поэтому ширину экрана можно менять и после mount.
const isNarrowRef = ref(false);
vi.mock('@/composables/useNarrowScreen', () => ({
  useNarrowScreen: () => ({ isNarrow: isNarrowRef }),
}));

import TablesComponent from '../TablesComponent.vue';
import { usePermissionsStore } from '@/stores/permissions';

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
        RouterLink: { props: ['to'], template: '<a><slot /></a>' },
        CarsTable: true,
        PeopleTable: true,
        FactTable: true,
        ApplicationDetail: true,
        TableExportModal: true,
        ManualAddModal: true,
        PassReportModal: true,
        DateFilter: true,
        // BaseModal живой: без него не проверить, что содержимое листа появляется
        // только при открытии.
      },
      mocks: {
        $route: { params: { tableName: 'kpp_4' } },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
      },
    },
  });
}

/**
 * Действия таблицы на мобилке (волна 5, мокап docs/mockups/mobile-ux.html):
 * «Версии», «Отчёт», «Экспорт», «Корзина» уезжают из шапки в лист «⋯», а главное
 * действие - в закреплённую снизу панель. В волне 6 туда же переехала «История»:
 * шапка самой таблицы стала рядом в 48px, и кнопке в ней места не осталось. Проверяем именно ПЕРЕЕЗД: кнопка обязана
 * существовать ровно в одном месте, иначе онбординг-тур и E2E находят два узла с
 * одним data-testid.
 */
describe('TablesComponent - действия таблицы на мобилке', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    // super проходит любой ключ прав - интересует раскладка, а не гейтинг.
    usePermissionsStore().mode = 'super';
    localStorage.clear();
    isNarrowRef.value = false;
    apiRequest.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return Promise.resolve(okResponse(TABLE_RESPONSE));
      if (url.startsWith('/users/me')) return Promise.resolve(okResponse({ id: 1, username: 'test' }));
      return Promise.resolve(okResponse([]));
    });
  });

  it('десктоп: действия инлайн в шапке, «⋯» и нижней панели нет', async () => {
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('.filters__options').exists()).toBe(true);
    expect(wrapper.find('[data-testid="table-versions-link"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="pass-report-button"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="manual-add-button"]').exists()).toBe(true);

    expect(wrapper.find('[data-testid="table-more-btn"]').exists()).toBe(false);
    expect(wrapper.find('.tables__fab-bar').exists()).toBe(false);
  });

  it('мобилка: ряд шапки без действий, «Добавить вручную» в нижней панели', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('.filters__options').exists()).toBe(false);
    // Лист закрыт - его содержимого в DOM нет, значит кнопок действий нет нигде.
    expect(wrapper.find('[data-testid="table-versions-link"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="pass-report-button"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="table-trash-link"]').exists()).toBe(false);

    expect(wrapper.find('[data-testid="table-more-btn"]').exists()).toBe(true);
    const add = wrapper.find('.tables__fab-bar [data-testid="manual-add-button"]');
    expect(add.exists()).toBe(true);
    expect(add.text()).toBe('Добавить вручную');
  });

  it('«⋯» открывает лист, и в нём все пять действий полными подписями', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.vm.showActionsSheet).toBe(false);
    await wrapper.find('[data-testid="table-more-btn"]').trigger('click');
    expect(wrapper.vm.showActionsSheet).toBe(true);

    const labels = wrapper.findAll('.actions-sheet__item').map((n) => n.text());
    expect(labels).toEqual([
      'Версии таблицы',
      'История изменений',
      'Отчёт по проездам',
      'Экспорт в Excel',
      'Корзина',
    ]);
  });

  it('действие из листа сначала закрывает лист, потом открывает своё окно', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="table-more-btn"]').trigger('click');
    await wrapper.find('[data-testid="pass-report-button"]').trigger('click');

    // Порядок важен: окно отчёта и лист лежат на одном слое модалок, и открытое
    // из-под листа окно оказывается не сверху, а рядом.
    expect(wrapper.vm.showActionsSheet).toBe(false);
    expect(wrapper.vm.showPassReport).toBe(true);
  });

  it('возврат на десктоп гасит лист - его кнопки снова в шапке', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="table-more-btn"]').trigger('click');
    expect(wrapper.vm.showActionsSheet).toBe(true);

    isNarrowRef.value = false;
    await nextTick();
    expect(wrapper.vm.showActionsSheet).toBe(false);
  });

  it('панель помечена признаком, по которому кнопка «наверх» уходит выше', async () => {
    isNarrowRef.value = true;
    const wrapper = mountPage();
    await flushPromises();

    // Глобальная кнопка «наверх» стоит в том же углу и без этого признака оказывается
    // под панелью. Само правило подъёма живёт в ScrollTopButton - здесь стережём, что
    // страница признак ставит: потеряется атрибут, и кнопка молча вернётся под панель.
    expect(wrapper.find('.tables__fab-bar').attributes('data-bottom-action-bar')).toBeDefined();
  });
});
