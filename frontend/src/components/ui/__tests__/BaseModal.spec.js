import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import BaseModal from '../BaseModal.vue';

function mountModal(props = {}, slots = {}) {
  return mount(BaseModal, {
    props: { show: true, ...props },
    slots: { default: '<p>Modal content</p>', ...slots },
    global: {
      stubs: { teleport: true },
    },
    attachTo: document.body,
  });
}

describe('BaseModal', () => {
  it('renders content when show=true', () => {
    const wrapper = mountModal();
    expect(wrapper.find('.base-modal__body').exists()).toBe(true);
    expect(wrapper.html()).toContain('Modal content');
  });

  it('hidden when show=false', () => {
    const wrapper = mountModal({ show: false });
    expect(wrapper.find('.base-modal-overlay').exists()).toBe(false);
  });

  it('renders title in header', () => {
    const wrapper = mountModal({ title: 'Тестовый заголовок' });
    expect(wrapper.find('.base-modal__title').text()).toBe('Тестовый заголовок');
  });

  it('emits close on close button click', async () => {
    const wrapper = mountModal({ title: 'Test' });
    await wrapper.find('.base-modal__close').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('emits close on overlay click when closeOnOverlay=true', async () => {
    const wrapper = mountModal({ closeOnOverlay: true });
    const overlay = wrapper.find('.base-modal-overlay');
    await overlay.trigger('mousedown');
    await overlay.trigger('mouseup');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('does not emit close on overlay click when closeOnOverlay=false', async () => {
    const wrapper = mountModal({ closeOnOverlay: false });
    const overlay = wrapper.find('.base-modal-overlay');
    await overlay.trigger('mousedown');
    await overlay.trigger('mouseup');
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('does not emit close when mousedown starts inside modal and mouseup lands on overlay (drag-out)', async () => {
    const wrapper = mountModal({ closeOnOverlay: true });
    const overlay = wrapper.find('.base-modal-overlay');
    await wrapper.find('.base-modal').trigger('mousedown');
    await overlay.trigger('mouseup');
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('emits close on Escape key', async () => {
    const wrapper = mountModal();
    await document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('does not emit close on Escape when closable=false', async () => {
    const wrapper = mountModal({ closable: false });
    await document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('does not render close button when closable=false', () => {
    const wrapper = mountModal({ closable: false, title: 'Test' });
    expect(wrapper.find('.base-modal__close').exists()).toBe(false);
  });

  // Bottom-sheet теперь дефолт на мобилке (#1097 p2): без флага ползунок ЕСТЬ.
  it('sheetSwipe по умолчанию включён: ползунок рендерится без флага', () => {
    const wrapper = mountModal();
    expect(wrapper.find('.sheet-handle').exists()).toBe(true);
  });

  // Отключение через :sheet-swipe="false" - ползунка нет и жест не действует.
  it('sheetSwipe=false (opt-out): ползунок не рендерится, свайп-вниз не эмитит close', async () => {
    const wrapper = mountModal({ sheetSwipe: false });
    expect(wrapper.find('.sheet-handle').exists()).toBe(false);
    const modal = wrapper.find('.base-modal');
    await modal.trigger('touchstart', { touches: [{ clientY: 100 }] });
    await modal.trigger('touchmove', { touches: [{ clientY: 300 }] });
    await modal.trigger('touchend');
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('sheetSwipe=true: ползунок рендерится, протяжка вниз за порог эмитит close', async () => {
    vi.useFakeTimers();
    const wrapper = mountModal({ sheetSwipe: true });
    expect(wrapper.find('.sheet-handle').exists()).toBe(true);
    const modal = wrapper.find('.base-modal');
    // dy = 300-100 = 200 > threshold 90 (useSwipeDismiss)
    await modal.trigger('touchstart', { touches: [{ clientY: 100 }] });
    await modal.trigger('touchmove', { touches: [{ clientY: 300 }] });
    await modal.trigger('touchend');
    // Закрытие после слайда-вниз (setTimeout ~260мс в useSwipeDismiss).
    vi.advanceTimersByTime(300);
    expect(wrapper.emitted('close')).toHaveLength(1);
    vi.useRealTimers();
  });

  it('sheetSwipe=true: короткая протяжка ниже порога НЕ эмитит close', async () => {
    const wrapper = mountModal({ sheetSwipe: true });
    const modal = wrapper.find('.base-modal');
    // dy = 40 < threshold 90 - лист вернётся на место, без закрытия
    await modal.trigger('touchstart', { touches: [{ clientY: 100 }] });
    await modal.trigger('touchmove', { touches: [{ clientY: 140 }] });
    await modal.trigger('touchend');
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  // Регресс (#1097 p2): контент реально прокручен (активный скроллер scrollTop>0) ->
  // свайп вниз = обычная прокрутка, НЕ закрытие. На >768px скроллер - само окно
  // (body.scrollTop всегда 0), поэтому getScrollTop должен брать max, не body ??.
  it('sheetSwipe: свайп при прокрученном окне (modal.scrollTop>0) НЕ эмитит close', async () => {
    const wrapper = mountModal();
    const modalEl = wrapper.find('.base-modal').element;
    const bodyEl = wrapper.find('.base-modal__body').element;
    Object.defineProperty(bodyEl, 'scrollTop', { value: 0, configurable: true });
    Object.defineProperty(modalEl, 'scrollTop', { value: 120, configurable: true });
    const modal = wrapper.find('.base-modal');
    await modal.trigger('touchstart', { touches: [{ clientY: 100 }] });
    await modal.trigger('touchmove', { touches: [{ clientY: 300 }] });
    await modal.trigger('touchend');
    expect(wrapper.emitted('close')).toBeUndefined();
  });
});
