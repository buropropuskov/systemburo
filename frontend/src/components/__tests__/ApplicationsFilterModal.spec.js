import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ApplicationsFilterModal from '@/components/ApplicationsFilterModal.vue';

// #1097 W3.6: вторичные фильтры Центра вынесены в модалку. Dumb view - проверяем
// рендер секций по пропсам, проброс эмитов со значениями и guard clearDateFilter.

const BaseModalStub = {
  name: 'BaseModal',
  props: ['show', 'title', 'width', 'radius', 'contentClass', 'closable', 'closeOnOverlay', 'zIndex', 'sheetSwipe'],
  template: '<div v-if="show" class="base-modal-stub"><slot /><div class="actions"><slot name="actions" /></div></div>',
};

const clearSelection = vi.fn();
const DateFilterStub = {
  name: 'DateFilter',
  methods: { clearSelection },
  template: '<div class="date-filter-stub" />',
};

// Мультивыборные фильтры (#1398): клик по стабу отдаёт массив, как настоящий
// BaseDropdown multiple. Различаем инстансы по проброшенному data-testid.
const BaseDropdownStub = {
  name: 'BaseDropdown',
  props: ['modelValue', 'options', 'placeholder', 'summaryLabel', 'labelKey', 'valueKey', 'multiple', 'searchable', 'teleport'],
  emits: ['update:modelValue'],
  template: `<button
    class="dd-stub"
    :data-summary="summaryLabel"
    :data-selected="(modelValue || []).join(',')"
    @click="$emit('update:modelValue', [firstValue])"
  />`,
  computed: {
    firstValue() {
      const first = this.options[0] || {};
      return first[this.valueKey || 'id'];
    },
  },
};

// Конфиги фильтров приходят из ApplicationsCenter готовыми - модалка их только рендерит.
const DIRECTORY_FILTERS = [
  {
    field: 'selectedOrganizationIds', values: [], options: [{ id: 7, name: 'Орг' }], allLabel: 'Все организации', summaryLabel: 'Организация', testid: 'center-filter-organizations',
  },
  {
    field: 'selectedCompanyIds', values: [], options: [{ id: 5, name: 'Компания' }], allLabel: 'Все компании', summaryLabel: 'Компания', testid: 'center-filter-companies',
  },
  {
    field: 'selectedUnloadPlaceIds', values: [], options: [{ id: 3, name: 'Место' }], allLabel: 'Все места разгрузки', summaryLabel: 'Места разгрузки', testid: 'center-filter-unload-places',
  },
  {
    field: 'selectedPassageTableIds', values: [], options: [{ id: 9, name: 'КПП' }], allLabel: 'Все проходы', summaryLabel: 'Проход', testid: 'center-filter-passage-tables',
  },
];

const STATE_FILTERS = [
  {
    field: 'selectedConfirmations', values: [], options: [{ value: 'approved', label: 'Согласовано' }], allLabel: 'Всё подтверждение', summaryLabel: 'Подтверждение', testid: 'center-filter-confirmations',
  },
  {
    field: 'selectedApplicationStatuses', values: [], options: [{ value: 'inwork', label: 'В работе' }], allLabel: 'Все статусы', summaryLabel: 'Статус', testid: 'center-filter-statuses',
  },
  {
    field: 'selectedTags', values: [], options: [{ value: 'bl', label: 'ЧС' }], allLabel: 'Все теги', summaryLabel: 'Теги', testid: 'center-filter-tags',
  },
];

function mountModal(props = {}) {
  return mount(ApplicationsFilterModal, {
    props: {
      show: true,
      directoryFilters: DIRECTORY_FILTERS,
      stateFilters: STATE_FILTERS,
      activeToday: false,
      sortField: null,
      sortDirection: 'desc',
      hasActiveFilters: false,
      ...props,
    },
    global: {
      stubs: { BaseModal: BaseModalStub, DateFilter: DateFilterStub, BaseDropdown: BaseDropdownStub },
    },
  });
}

const td = (w, id) => w.find(`[data-testid="${id}"]`);

describe('ApplicationsFilterModal', () => {
  it('рендерит все справочные фильтры мультивыбором (#1398)', () => {
    const w = mountModal();
    expect(td(w, 'center-filter-organizations').exists()).toBe(true);
    expect(td(w, 'center-filter-companies').exists()).toBe(true);
    expect(td(w, 'center-filter-unload-places').exists()).toBe(true);
    expect(td(w, 'center-filter-passage-tables').exists()).toBe(true);
  });

  it('выбор в справочном фильтре эмитит set-filter с именем поля и массивом', async () => {
    const w = mountModal();
    await td(w, 'center-filter-unload-places').trigger('click');
    expect(w.emitted('set-filter')[0]).toEqual(['selectedUnloadPlaceIds', [3]]);
  });

  it('выбор в фильтре состояния эмитит set-filter со значением опции, а не id', async () => {
    const w = mountModal();
    await td(w, 'center-filter-statuses').trigger('click');
    expect(w.emitted('set-filter')[0]).toEqual(['selectedApplicationStatuses', ['inwork']]);
  });

  it('выбранные значения уходят в дропдаун (модалка отражает состояние родителя)', () => {
    const withSelection = DIRECTORY_FILTERS.map((f) => (
      f.field === 'selectedOrganizationIds' ? { ...f, values: [7] } : f
    ));
    const w = mountModal({ directoryFilters: withSelection });
    expect(td(w, 'center-filter-organizations').attributes('data-selected')).toBe('7');
    expect(td(w, 'center-filter-companies').attributes('data-selected')).toBe('');
  });

  it('секция сортировки: клик по полю эмитит sort-by, активное поле показывает направление', async () => {
    const w = mountModal({ sortField: 'date', sortDirection: 'asc' });
    const dateBtn = td(w, 'center-button-sort-date');
    const numberBtn = td(w, 'center-button-sort-number');
    expect(dateBtn.exists()).toBe(true);
    // активное поле date -> подсветка + стрелка вверх (asc)
    expect(dateBtn.classes()).toContain('status-btn--active');
    expect(dateBtn.find('.sort-btn__dir').text()).toBe('↑');
    // неактивное поле - без подсветки и без стрелки
    expect(numberBtn.classes()).not.toContain('status-btn--active');
    expect(numberBtn.find('.sort-btn__dir').exists()).toBe(false);
    await numberBtn.trigger('click');
    expect(w.emitted('sort-by')[0]).toEqual(['number']);
  });

  it('рендерит секции фильтров по пропсам', () => {
    const w = mountModal();
    expect(td(w, 'center-button-today').exists()).toBe(true);
    expect(td(w, 'center-filter-confirmations').exists()).toBe(true);
    expect(td(w, 'center-filter-statuses').exists()).toBe(true);
    expect(td(w, 'center-filter-tags').exists()).toBe(true);
    expect(td(w, 'center-button-reset-filters').exists()).toBe(true);
    expect(w.find('.date-filter-stub').exists()).toBe(true);
  });

  it('клики по бинарным чипам и сбросу эмитят свои события', async () => {
    // hasActiveFilters:true - иначе кнопка «Сбросить фильтры» disabled и клик не эмитит
    const w = mountModal({ hasActiveFilters: true });
    await td(w, 'center-button-today').trigger('click');
    expect(w.emitted('toggle-today')).toBeTruthy();
    await td(w, 'center-button-reset-filters').trigger('click');
    expect(w.emitted('reset-filters')).toBeTruthy();
  });

  it('active-класс бинарного чипа отражает проп', () => {
    const w = mountModal({ activeToday: true });
    expect(td(w, 'center-button-today').classes()).toContain('status-btn--active');
  });

  // Чип «Обновления» жил здесь с #1349, а теперь переключатель стоит в шапке Центра и
  // виден на мобилке тоже. Замок держит его отсюда убранным: копия дала бы два контрола
  // на один флаг в одном вьюпорте и два узла с одинаковым data-testid, по которому
  // ищут и тесты, и туры. Поведение самой кнопки проверяет
  // views/__tests__/ApplicationsCenterHeaderFilters.spec.js.
  it('чипа "Обновления" в модалке нет - переключатель живёт в шапке Центра', () => {
    const w = mountModal();
    expect(td(w, 'center-button-updates').exists()).toBe(false);
    expect(w.emitted('toggle-status-updated')).toBeFalsy();
  });

  it('reset-filters: disabled без активных фильтров, enabled с активными', () => {
    expect(td(mountModal({ hasActiveFilters: false }), 'center-button-reset-filters').attributes('disabled')).toBeDefined();
    expect(td(mountModal({ hasActiveFilters: true }), 'center-button-reset-filters').attributes('disabled')).toBeUndefined();
  });

  it('clearDateFilter зовёт clearSelection у DateFilter', () => {
    clearSelection.mockClear();
    const w = mountModal();
    w.vm.clearDateFilter();
    expect(clearSelection).toHaveBeenCalledTimes(1);
  });

  it('clearDateFilter не падает, когда DateFilter не отрендерен (модалка закрыта)', () => {
    const w = mountModal({ show: false });
    expect(() => w.vm.clearDateFilter()).not.toThrow();
  });
});
