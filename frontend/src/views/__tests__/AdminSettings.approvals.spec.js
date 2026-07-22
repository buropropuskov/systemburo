import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const getSettings = vi.fn();
const updateSetting = vi.fn();
vi.mock('@/api/settings', () => ({
  getSettings: (...a) => getSettings(...a),
  updateSetting: (...a) => updateSetting(...a),
}));

import AdminSettings from '../AdminSettings.vue';
import { useDeletionsStore } from '@/stores/deletions';

async function mountView() {
  getSettings.mockResolvedValue([]);
  updateSetting.mockResolvedValue({});
  const wrapper = shallowMount(AdminSettings);
  await flushPromises();
  return wrapper;
}

function setApproval(vm, { enabled = true, first = 3, repeat = 3 } = {}) {
  vm.settings.approval_reminder_enabled = enabled;
  vm.settings.approval_reminder_first_days = first;
  vm.settings.approval_reminder_repeat_days = repeat;
}

describe('AdminSettings - напоминания согласующим', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getSettings.mockReset();
    updateSetting.mockReset();
  });

  it('валидное сохранение пишет три ключа approval.reminder_*', async () => {
    const wrapper = await mountView();

    setApproval(wrapper.vm, { enabled: false, first: 5, repeat: 7 });
    await wrapper.vm.saveApprovalSettings();

    expect(updateSetting).toHaveBeenCalledWith('approval.reminder_enabled', 'false');
    expect(updateSetting).toHaveBeenCalledWith('approval.reminder_first_days', '5');
    expect(updateSetting).toHaveBeenCalledWith('approval.reminder_repeat_days', '7');
  });

  it('граничные значения 1 и 30 валидны: пишет ключи и даёт success-notify', async () => {
    const wrapper = await mountView();
    const store = useDeletionsStore();
    const notifySpy = vi.spyOn(store, 'notify');

    setApproval(wrapper.vm, { enabled: true, first: 1, repeat: 30 });
    await wrapper.vm.saveApprovalSettings();

    expect(updateSetting).toHaveBeenCalledWith('approval.reminder_enabled', 'true');
    expect(updateSetting).toHaveBeenCalledWith('approval.reminder_first_days', '1');
    expect(updateSetting).toHaveBeenCalledWith('approval.reminder_repeat_days', '30');
    expect(notifySpy).toHaveBeenCalledWith(
      expect.objectContaining({ prefix: 'Настройки напоминаний сохранены' }),
    );
  });

  it('первое напоминание вне 1-30 блокирует сохранение', async () => {
    const wrapper = await mountView();
    const store = useDeletionsStore();
    const notifySpy = vi.spyOn(store, 'notify');

    setApproval(wrapper.vm, { first: 0, repeat: 3 });
    await wrapper.vm.saveApprovalSettings();

    expect(updateSetting).not.toHaveBeenCalled();
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('повтор напоминания вне 1-30 блокирует сохранение', async () => {
    const wrapper = await mountView();
    const store = useDeletionsStore();
    const notifySpy = vi.spyOn(store, 'notify');

    setApproval(wrapper.vm, { first: 3, repeat: 31 });
    await wrapper.vm.saveApprovalSettings();

    expect(updateSetting).not.toHaveBeenCalled();
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('fetchSettings разбирает реальные ключи бэка approval.reminder_*', async () => {
    getSettings.mockResolvedValue([
      { key: 'approval.reminder_enabled', value: 'false', type: 'bool' },
      { key: 'approval.reminder_first_days', value: '10', type: 'int' },
      { key: 'approval.reminder_repeat_days', value: '4', type: 'int' },
    ]);
    updateSetting.mockResolvedValue({});
    const wrapper = shallowMount(AdminSettings);
    await flushPromises();

    expect(wrapper.vm.settings.approval_reminder_enabled).toBe(false);
    expect(wrapper.vm.settings.approval_reminder_first_days).toBe(10);
    expect(wrapper.vm.settings.approval_reminder_repeat_days).toBe(4);
  });
});
