import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { useUiStore } from '@/stores/ui';
import EntityPeriodModal from '../EntityPeriodModal.vue';
import ManualAttachExistingModal from '../ManualAttachExistingModal.vue';
import AddToBlacklistModal from '../admin/blacklist/AddToBlacklistModal.vue';

beforeEach(() => setActivePinia(createPinia()));

function context(phone = false) {
  return { drawerContext: { desktop: ref(!phone) }, saving: false, $emit: vi.fn() };
}

describe('entity drawer forms retain unsaved data', () => {
  it('asks for a changed period even with an empty reason, and keeps it on cancel', async () => {
    const form = { entry_date_to: '2026-11-06' };
    const vm = { ...context(), snapshot: {}, reason: '', mode: 'individual', form,
      initialPeriod: JSON.stringify(['individual', { entry_date_to: '2026-11-05' }]) };
    const pending = EntityPeriodModal.methods.close.call(vm);
    expect(useUiStore().confirmState).not.toBeNull();
    useUiStore().resolveConfirm(false); await pending;
    expect(vm.$emit).not.toHaveBeenCalled();
    expect(vm.form).toEqual(form);
  });

  it('closes an unchanged period without asking', async () => {
    const form = { entry_date_to: '2026-11-05' };
    const vm = { ...context(), snapshot: {}, reason: '', mode: 'inherit', form,
      initialPeriod: JSON.stringify(['inherit', form]) };
    await EntityPeriodModal.methods.close.call(vm);
    expect(useUiStore().confirmState).toBeNull();
    expect(vm.$emit).toHaveBeenCalledWith('close');
  });

  it('protects a selected binding period before an application was chosen', async () => {
    const vm = { ...context(), reason: '', applicationID: null, targetID: null, sourceID: null,
      periodChoice: 'individual', invalidate: vi.fn() };
    const pending = ManualAttachExistingModal.methods.close.call(vm);
    expect(useUiStore().confirmState).not.toBeNull();
    useUiStore().resolveConfirm(false); await pending;
    expect(vm.invalidate).not.toHaveBeenCalled();
    expect(vm.$emit).not.toHaveBeenCalled();
  });

  it('discards a binding only after confirmation', async () => {
    const vm = { ...context(), reason: 'Учебная причина', applicationID: 1, invalidate: vi.fn() };
    const pending = ManualAttachExistingModal.methods.close.call(vm);
    useUiStore().resolveConfirm(true); await pending;
    expect(vm.invalidate).toHaveBeenCalledOnce();
    expect(vm.$emit).toHaveBeenCalledWith('close');
  });

  it('retains a blacklist reason after cancellation', async () => {
    const vm = { ...context(), reason: 'Учебная причина', initialReason: '' };
    const pending = AddToBlacklistModal.methods.close.call(vm);
    useUiStore().resolveConfirm(false); await pending;
    expect(vm.$emit).not.toHaveBeenCalled();
    expect(vm.reason).toBe('Учебная причина');
  });

  it('does not change phone closing behavior', async () => {
    const vm = { ...context(true), reason: 'Учебная причина', initialReason: '' };
    await AddToBlacklistModal.methods.close.call(vm);
    expect(useUiStore().confirmState).toBeNull();
    expect(vm.$emit).toHaveBeenCalledWith('close');
  });

  it('keeps a saving form open without presenting a confirmation', async () => {
    const vm = { ...context(), saving: true, reason: 'Учебная причина', initialReason: '' };
    await AddToBlacklistModal.methods.close.call(vm);
    expect(useUiStore().confirmState).toBeNull();
    expect(vm.$emit).not.toHaveBeenCalled();
  });
});
