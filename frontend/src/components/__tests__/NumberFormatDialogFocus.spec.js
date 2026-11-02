import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NumberFormat from '../NumberFormat.vue';
import BaseModal from '../ui/BaseModal.vue';
import { resetModalStack } from '@/utils/modalStack';
import { resetBodyScrollLock } from '@/utils/bodyScrollLock';
import { confirmIfAnyDirty } from '@/utils/dirtyTracker';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(async () => ({ ok: true, json: async () => [] })) }));
vi.mock('@/utils/dirtyTracker', () => ({ registerDirtyTracker: () => () => {}, confirmIfAnyDirty: vi.fn(async () => true) }));
const wrappers = [];
beforeEach(() => { setActivePinia(createPinia()); vi.mocked(confirmIfAnyDirty).mockResolvedValue(true); vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([{}]); });
afterEach(() => { wrappers.reverse().forEach(w => w.unmount()); wrappers.length = 0; resetModalStack(); resetBodyScrollLock(); document.body.replaceChildren(); vi.restoreAllMocks(); });
const tab = shiftKey => document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey, bubbles: true, cancelable: true }));
const escape = () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
async function setup(cell = false) {
  const w = mount(NumberFormat, { attachTo: document.body }); wrappers.push(w); await flushPromises();
  if (cell) {
    await w.setData({ selectedFormat: { format: { id: 1, name: 'Альфа', is_active: true }, cells: [{ cell_order: 0, cell_type: 'letters', min_length: 1, max_length: 1, alphabet_type: 'cyrillic', allowed_letters: '', padding_side: 'left' }] } });
  }
  const trigger = w.get(cell ? '.cell-card' : '[data-testid=nf-add-btn]');
  trigger.element.focus(); await trigger.trigger('click'); await flushPromises();
  const root = document.querySelector(cell ? '[data-testid=nf-cell-modal] .nf-modal' : '[data-testid=nf-modal] .nf-modal');
  return { w, trigger: trigger.element, root };
}
describe.each([false, true])('Number format dialog, cell=%s', cell => {
  it('owns focus, wraps Tab and restores the caller on Escape', async () => {
    const { trigger, root } = await setup(cell);
    expect(root.getAttribute('role')).toBe('dialog');
    expect(document.activeElement).toBe(root);
    expect(document.body.style.overflow).toBe('hidden');
    tab(true); expect(root.contains(document.activeElement)).toBe(true);
    tab(); expect(root.contains(document.activeElement)).toBe(true);
    escape(); await flushPromises();
    expect(document.activeElement).toBe(trigger);
    expect(document.body.style.overflow).toBe('');
  });
  it('leaves the underlying editor open when Escape closes a higher dialog', async () => {
    const { root } = await setup(cell);
    const upper = mount(BaseModal, { props: { show: true, zIndex: 10000 }, attachTo: document.body });
    wrappers.push(upper); await flushPromises();
    escape(); await flushPromises();
    expect(upper.emitted('close')).toHaveLength(1);
    expect(root.isConnected).toBe(true);
  });
});
it('keeps the edited form open when the existing dirty guard refuses closing', async () => {
  const { w, root } = await setup();
  const input = document.querySelector('[data-testid=nf-input-name]');
  input.value = 'Альфа'; input.dispatchEvent(new Event('input', { bubbles: true }));
  await flushPromises();
  vi.mocked(confirmIfAnyDirty).mockResolvedValue(false);
  escape(); await flushPromises();
  expect(confirmIfAnyDirty).toHaveBeenCalled();
  expect(root.isConnected).toBe(true);
  expect(w.vm.showAddModal).toBe(true);
  expect(document.body.style.overflow).toBe('hidden');
});
