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

// Один стаб на оба фильтра (Организация/Компания) - различаем по all-label.
const OrganizationFilterStub = {
  name: 'OrganizationFilter',
  props: ['value', 'organizations', 'allLabel', 'placeholderText'],
  emits: ['change'],
  template: '<button class="org-filter-stub" :data-all-label="allLabel" @click="$emit(\'change\', 7)" />',
};

function mountModal(props = {}) {
  return mount(ApplicationsFilterModal, {
    props: {
      show: true,
      organizations: [{ id: 7, name: 'Орг' }],
      selectedOrganizationId: null,
      companies: [{ id: 5, name: 'Компания' }],
      selectedCompanyId: null,
      confirmations: [{ value: 'approved', label: 'Согласовано' }],
      selectedConfirmations: [],
      applicationStatuses: [{ value: 'inwork', label: 'В работе' }],
      selectedApplicationStatuses: [],
      tags: [{ value: 'bl', label: 'ЧС' }],
      selectedTags: [],
      activeToday: false,
      sortField: null,
      sortDirection: 'desc',
      hasActiveFilters: false,
      ...props,
    },
    global: {
      stubs: { BaseModal: BaseModalStub, DateFilter: DateFilterStub, OrganizationFilter: OrganizationFilterStub },
    },
  });
}

const td = (w, id) => w.find(`[data-testid="${id}"]`);

describe('ApplicationsFilterModal', () => {
  it('рендерит секцию организации и пробрасывает change как organization-change', async () => {
    const w = mountModal();
    // Первый фильтр без all-label «Все компании» = организация.
    const org = w.findAll('.org-filter-stub').find((el) => el.attributes('data-all-label') !== 'Все компании');
    expect(org.exists()).toBe(true);
    await org.trigger('click');
    expect(w.emitted('organization-change')[0]).toEqual([7]);
  });

  it('рендерит секцию компании и пробрасывает change как company-change', async () => {
    const w = mountModal();
    const company = w.find('[data-all-label="Все компании"]');
    expect(company.exists()).toBe(true);
    await company.trigger('click');
    expect(w.emitted('company-change')[0]).toEqual([7]);
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
    expect(td(w, 'center-button-confirmation-approved').exists()).toBe(true);
    expect(td(w, 'center-button-status-inwork').exists()).toBe(true);
    expect(td(w, 'center-button-tag-bl').exists()).toBe(true);
    expect(td(w, 'center-button-reset-filters').exists()).toBe(true);
    expect(w.find('.date-filter-stub').exists()).toBe(true);
  });

  it('клики эмитят правильные события со значением', async () => {
    // hasActiveFilters:true - иначе кнопка «Сбросить фильтры» disabled и клик не эмитит
    const w = mountModal({ hasActiveFilters: true });
    await td(w, 'center-button-today').trigger('click');
    expect(w.emitted('toggle-today')).toBeTruthy();
    await td(w, 'center-button-confirmation-approved').trigger('click');
    expect(w.emitted('toggle-confirmation')[0]).toEqual(['approved']);
    await td(w, 'center-button-status-inwork').trigger('click');
    expect(w.emitted('toggle-status')[0]).toEqual(['inwork']);
    await td(w, 'center-button-tag-bl').trigger('click');
    expect(w.emitted('toggle-tag')[0]).toEqual(['bl']);
    await td(w, 'center-button-reset-filters').trigger('click');
    expect(w.emitted('reset-filters')).toBeTruthy();
  });

  it('active-класс отражает selected-пропсы', () => {
    const w = mountModal({ selectedApplicationStatuses: ['inwork'], activeToday: true });
    expect(td(w, 'center-button-status-inwork').classes()).toContain('status-btn--active');
    expect(td(w, 'center-button-today').classes()).toContain('status-btn--active');
    // невыбранное подтверждение - без active
    expect(td(w, 'center-button-confirmation-approved').classes()).not.toContain('status-btn--active');
  });

  it('чип "Обновления" эмитит toggle-status-updated и показывает счётчик (#1349)', async () => {
    const w = mountModal({ statusUpdatedOnly: true, statusUpdateCount: 4 });
    const btn = td(w, 'center-button-status-updated');
    expect(btn.exists()).toBe(true);
    expect(btn.classes()).toContain('status-btn--active');
    expect(btn.text()).toContain('Обновления: 4');
    await btn.trigger('click');
    expect(w.emitted('toggle-status-updated')).toBeTruthy();
  });

  it('чип "Обновления" без счётчика показывает только подпись', () => {
    const btn = td(mountModal({ statusUpdateCount: 0 }), 'center-button-status-updated');
    expect(btn.text()).toBe('Обновления');
    expect(btn.classes()).not.toContain('status-btn--active');
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
