import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { h, nextTick } from 'vue';
import BaseModal from '../BaseModal.vue';
import BaseDropdown from '../BaseDropdown.vue';
import { resetModalStack, setModalOpen, releaseModal } from '@/utils/modalStack';

const wrappers = [];
beforeEach(() => { vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([{}]); });
afterEach(() => { wrappers.reverse().forEach(w => w.unmount()); wrappers.length = 0; resetModalStack(); document.body.replaceChildren(); vi.restoreAllMocks(); });
function modal(props = {}, content = () => h('button', { id: 'inside' }, 'Внутри')) {
  const w = mount(BaseModal, { props: { show: true, ...props }, attachTo: document.body, slots: { default: content } });
  wrappers.push(w); return w;
}
function tab(shiftKey = false) { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey, cancelable: true })); }

describe('BaseModal focus integration', () => {
  it('focuses on opening, handles the first Tab and restores the trigger', async () => {
    const trigger = document.createElement('button'); document.body.append(trigger); trigger.focus();
    const w = modal(); await nextTick(); expect(document.activeElement).toBe(w.vm.modal);
    tab(); expect(document.activeElement).toBe(w.vm.modal.querySelector('.base-modal__close'));
    await w.setProps({ show: false }); expect(document.activeElement).toBe(trigger);
  });
  it('lets only the top modal handle Tab and restores the parent control', async () => {
    const parent = modal(); await nextTick(); const inside = parent.vm.modal.querySelector('#inside'); inside.focus();
    const child = modal({ zIndex: 2000 }, () => h('button', { id: 'child' }, 'Верхний')); await nextTick();
    tab(true); expect(document.activeElement.id).toBe('child');
    await child.setProps({ show: false }); expect(document.activeElement).toBe(inside);
  });
  it('restores the caller when an open modal is unmounted', async () => {
    const trigger = document.createElement('button'); document.body.append(trigger); trigger.focus();
    const w = modal(); await nextTick(); w.unmount(); wrappers.pop(); expect(document.activeElement).toBe(trigger);
  });
  it('does not move focus from the tour owner but handles a dialog above it', async () => {
    const tour = {}, target = document.createElement('button'); document.body.append(target); target.focus(); setModalOpen(tour, true, 12502);
    modal(); await nextTick(); tab(); expect(document.activeElement).toBe(target);
    const upper = modal({ zIndex: 13000 }); await nextTick(); expect(document.activeElement).toBe(upper.vm.modal);
    await upper.setProps({ show: false }); expect(document.activeElement).toBe(target); releaseModal(tour);
  });
  it('includes the real teleported dropdown search and keeps Escape local', async () => {
    const w = modal({}, () => [h(BaseDropdown, { teleport: true, searchable: true, options: [{ id: 1, name: 'Первый' }] }), h('button', { id: 'after' }, 'После')]);
    await nextTick(); const dropdown = w.getComponent(BaseDropdown); dropdown.vm.open(); await nextTick();
    tab(); tab(); tab(); expect(document.activeElement.classList.contains('base-dropdown__search-input')).toBe(true);
    tab(); expect(document.activeElement.id).toBe('after');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); await nextTick();
    expect(dropdown.vm.isOpen).toBe(false); expect(w.emitted('close')).toBeUndefined();
  });
  it('returns from a teleported menu to its trigger on Escape', async () => {
    const w = modal({}, () => h(BaseDropdown, { teleport: true, searchable: true, options: [{ id: 1, name: 'Первый' }] }));
    await nextTick(); const dropdown = w.getComponent(BaseDropdown); dropdown.vm.open(); await nextTick();
    document.querySelector('.base-dropdown__search-input').focus();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); await nextTick();
    expect(document.activeElement).toBe(dropdown.vm.$refs.triggerButton);
    expect(w.emitted('close')).toBeUndefined();
  });
});
