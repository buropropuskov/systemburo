import { describe, it, expect } from 'vitest';
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
});
