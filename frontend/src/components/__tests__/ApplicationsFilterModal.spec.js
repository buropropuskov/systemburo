import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ApplicationsFilterModal from '@/components/ApplicationsFilterModal.vue';

// #1097 W3.6: вторичные фильтры Центра вынесены в модалку. Dumb view - проверяем
// рендер секций по пропсам, проброс эмитов со значениями и guard clearDateFilter.

const BaseModalStub = {
  name: 'BaseModal',
  props: ['show', 'title', 'width', 'radius', 'contentClass', 'closable', 'closeOnOverlay', 'zIndex'],
  template: '<div v-if="show" class="base-modal-stub"><slot /><div class="actions"><slot name="actions" /></div></div>',
};

const clearSelection = vi.fn();
const DateFilterStub = {
  name: 'DateFilter',
  methods: { clearSelection },
  template: '<div class="date-filter-stub" />',
};

function mountModal(props = {}) {
  return mount(ApplicationsFilterModal, {
    props: {
      show: true,
      confirmations: [{ value: 'approved', label: 'Согласовано' }],
      selectedConfirmations: [],
      applicationStatuses: [{ value: 'inwork', label: 'В работе' }],
      selectedApplicationStatuses: [],
      tags: [{ value: 'bl', label: 'ЧС' }],
      selectedTags: [],
      activeToday: false,
      sortField: null,
      hasActiveFilters: false,
      ...props,
    },
    global: { stubs: { BaseModal: BaseModalStub, DateFilter: DateFilterStub } },
  });
}

const td = (w, id) => w.find(`[data-testid="${id}"]`);

describe('ApplicationsFilterModal', () => {
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
