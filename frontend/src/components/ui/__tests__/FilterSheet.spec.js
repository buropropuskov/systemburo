import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import FilterSheet from '@/components/ui/FilterSheet.vue';

// Свайп-закрытие живёт только на телефоне: на планшете и десктопе окно рисуется
// диалогом по центру, и `useSwipeDismiss` жест там не активирует. jsdom по умолчанию
// отдаёт ширину 1024, поэтому задаём телефонную явно - иначе тесты проверяли бы жест
// на ширине, где его и не должно быть.
const swipeWidth = window.innerWidth;
beforeEach(() => { window.innerWidth = 390; });
afterEach(() => { window.innerWidth = swipeWidth; });


// Стаб BaseModal: рендерит default + actions слоты и умеет эмитить close
// (реальный BaseModal телепортится в body и тянет useSwipeDismiss - не нужно тут).
const BaseModalStub = {
  name: 'BaseModal',
  props: ['show', 'title', 'width', 'radius', 'contentClass', 'sheetSwipe'],
  emits: ['close'],
  template: `
    <div class="base-modal-stub">
      <slot />
      <div class="stub-actions"><slot name="actions" /></div>
      <button class="stub-close" @click="$emit('close')" />
    </div>`,
};

function mountSheet(props = {}, slots = {}) {
  return mount(FilterSheet, {
    props: { show: true, ...props },
    slots: { default: '<div class="my-filter">FILTER</div>', ...slots },
    global: { stubs: { BaseModal: BaseModalStub } },
  });
}

describe('FilterSheet', () => {
  it('рендерит фильтры из default-слота', () => {
    const w = mountSheet();
    expect(w.find('.my-filter').text()).toBe('FILTER');
  });

  it('кнопка «Сбросить» disabled без активных фильтров', () => {
    const w = mountSheet({ hasActiveFilters: false });
    const btn = w.find('[data-testid="filter-sheet-reset"]');
    expect(btn.exists()).toBe(true);
    expect(btn.attributes('disabled')).toBeDefined();
  });

  it('при активных фильтрах «Сбросить» активна и эмитит reset', async () => {
    const w = mountSheet({ hasActiveFilters: true });
    const btn = w.find('[data-testid="filter-sheet-reset"]');
    expect(btn.attributes('disabled')).toBeUndefined();
    await btn.trigger('click');
    expect(w.emitted('reset')).toBeTruthy();
  });

  it('форвардит close из BaseModal', async () => {
    const w = mountSheet();
    await w.find('.stub-close').trigger('click');
    expect(w.emitted('close')).toBeTruthy();
  });

  it('actions-слот переопределяет дефолтную кнопку сброса', () => {
    const w = mountSheet({}, { actions: '<button class="custom-action">X</button>' });
    expect(w.find('.custom-action').exists()).toBe(true);
    expect(w.find('[data-testid="filter-sheet-reset"]').exists()).toBe(false);
  });
});
