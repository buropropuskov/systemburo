import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NumberFormat from '../NumberFormat.vue';
import DirtyConfirm from '../DirtyConfirmModal.vue';
import BaseModal from '../ui/BaseModal.vue';
import { confirmIfAnyDirty, registerDirtyTracker, resolveDirtyConfirm, confirmState } from '@/utils/dirtyTracker';
import { resetModalStack } from '@/utils/modalStack';
import { resetBodyScrollLock } from '@/utils/bodyScrollLock';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(async () => ({ ok: true, json: async () => [] })) }));
const wrappers = [], stops = [];
beforeEach(() => { setActivePinia(createPinia()); vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([{}]); });
afterEach(async () => {
  resolveDirtyConfirm(false); await flushPromises();
  wrappers.reverse().forEach(w => w.unmount()); wrappers.length = 0;
  stops.forEach(stop => stop()); stops.length = 0;
  resetModalStack(); resetBodyScrollLock(); document.body.replaceChildren(); vi.restoreAllMocks();
});
const add = (Component, props = {}) => { const w = mount(Component, { props, attachTo: document.body }); wrappers.push(w); return w; };
const escape = () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
const tab = shiftKey => document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey, bubbles: true, cancelable: true }));
describe('Dirty confirmation in the real modal stack', () => {
  it('takes focus above the editor, cancels only the question and preserves the form', async () => {
    add(DirtyConfirm); const form = add(NumberFormat); await flushPromises();
    const open = form.get('[data-testid=nf-add-btn]'); open.element.focus(); await open.trigger('click'); await flushPromises();
    const input = document.querySelector('[data-testid=nf-input-name]');
    input.value = 'Альфа'; input.dispatchEvent(new Event('input', { bubbles: true })); await flushPromises();
    const cancel = document.querySelector('[data-testid=nf-modal-cancel]'); cancel.focus(); cancel.click(); await flushPromises();
    const question = document.querySelector('.dirty-modal');
    expect(question.getAttribute('role')).toBe('dialog');
    expect(document.activeElement).toBe(question);
    tab(true); expect(question.contains(document.activeElement)).toBe(true);
    tab(); expect(document.activeElement).toBe(question.querySelector('.dirty-modal__content'));
    escape(); await flushPromises();
    expect(confirmState.show).toBe(false);
    expect(form.vm.showAddModal).toBe(true);
    expect(input.value).toBe('Альфа');
    expect(document.activeElement).toBe(cancel);
    expect(document.body.style.overflow).toBe('hidden');
  });
  it('returns to a page control when a standalone question is cancelled', async () => {
    add(DirtyConfirm); stops.push(registerDirtyTracker(() => true));
    const trigger = document.createElement('button'); document.body.append(trigger); trigger.focus();
    const answer = confirmIfAnyDirty(); await flushPromises();
    expect(document.activeElement).toBe(document.querySelector('.dirty-modal'));
    escape(); expect(await answer).toBe(false); await flushPromises();
    expect(document.activeElement).toBe(trigger);
    expect(document.body.style.overflow).toBe('');
  });
  it('leaves its question open when Escape belongs to a higher modal', async () => {
    add(DirtyConfirm); stops.push(registerDirtyTracker(() => true));
    const answer = confirmIfAnyDirty(); await flushPromises();
    const upper = add(BaseModal, { show: true, zIndex: 22000 }); await flushPromises();
    escape(); await flushPromises();
    expect(upper.emitted('close')).toHaveLength(1);
    expect(confirmState.show).toBe(true);
    resolveDirtyConfirm(false); await answer;
  });
});
