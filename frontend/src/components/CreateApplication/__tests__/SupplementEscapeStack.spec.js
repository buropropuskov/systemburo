import { afterEach, describe, expect, it, vi } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { nextTick } from 'vue';
import { setModalOpen, releaseModal, isTopModal, isEscapeHandled, markEscapeHandled, resetModalStack } from '@/utils/modalStack';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false, json: async () => [] }) }));
vi.mock('@/api/applications', () => ({ createSupplement: vi.fn() }));
vi.mock('@/api/attachment-templates', () => ({ getFieldConfig: vi.fn().mockResolvedValue({ base: [] }) }));
import SupplementModal from '../SupplementModal.vue';
import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';

let wrapper, parentListener;
afterEach(() => {
  wrapper?.unmount(); wrapper = null;
  if (parentListener) document.removeEventListener('keydown', parentListener);
  resetModalStack();
});
const cases = [
  [SupplementModal, 'show', { application: { id: 1 }, attachments: [] }, 10010],
  [ExistingCarsModal, 'visible', { zIndex: 10012 }, 10012],
  [ExistingEmployeesModal, 'visible', { zIndex: 10012 }, 10012],
];

describe.each(cases)('$0.name Escape ownership', (Component, flag, props, layer) => {
  it.each(['before', 'after'])('closes only the top window with parent listener registered %s', async order => {
    setActivePinia(createPinia());
    const parent = {}, above = {}, parentClose = vi.fn();
    setModalOpen(parent, true, 10002);
    parentListener = event => {
      if (event.key !== 'Escape' || isEscapeHandled(event) || !isTopModal(parent)) return;
      markEscapeHandled(event); parentClose();
    };
    if (order === 'before') document.addEventListener('keydown', parentListener);
    wrapper = shallowMount(Component, { props: { ...props, [flag]: true }, global: { stubs: { teleport: true } } });
    if (order === 'after') document.addEventListener('keydown', parentListener);
    await nextTick();
    setModalOpen(above, true, layer + 1);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toBeUndefined(); expect(parentClose).not.toHaveBeenCalled();
    releaseModal(above);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toHaveLength(1); expect(parentClose).not.toHaveBeenCalled();
    await wrapper.setProps({ [flag]: false });
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(parentClose).toHaveBeenCalledTimes(1);
    await wrapper.setProps({ [flag]: true });
    expect(isTopModal(parent)).toBe(false);
    wrapper.unmount(); wrapper = null;
    expect(isTopModal(parent)).toBe(true);
  });
});
