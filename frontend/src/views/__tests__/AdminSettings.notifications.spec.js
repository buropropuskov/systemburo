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

function setNotif(vm, { del = 10, res = 5, poll = 30 } = {}) {
  vm.settings.notifications_enabled = true;
  vm.settings.notifications_poll_interval = poll;
  vm.settings.notifications_delete_duration = del;
  vm.settings.notifications_restore_duration = res;
}

describe('AdminSettings - уведомления', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getSettings.mockReset();
    updateSetting.mockReset();
  });

  it('валидное сохранение пишет настройки и сразу применяет к стору (setDurations)', async () => {
    const wrapper = await mountView();
    const store = useDeletionsStore();
    const spy = vi.spyOn(store, 'setDurations');

    setNotif(wrapper.vm, { del: 15, res: 3, poll: 30 });
    await wrapper.vm.saveNotificationSettings();

    expect(updateSetting).toHaveBeenCalledWith('notifications.delete_duration', '15');
    expect(updateSetting).toHaveBeenCalledWith('notifications.restore_duration', '3');
    expect(spy).toHaveBeenCalledWith(15, 3);
  });

  it('длительность вне 3-60 блокирует сохранение (нет запросов и setDurations)', async () => {
    const wrapper = await mountView();
    const store = useDeletionsStore();
    const spy = vi.spyOn(store, 'setDurations');
    const notifySpy = vi.spyOn(store, 'notify');

    setNotif(wrapper.vm, { del: 2, res: 5, poll: 30 });
    await wrapper.vm.saveNotificationSettings();

    expect(updateSetting).not.toHaveBeenCalled();
    expect(spy).not.toHaveBeenCalled();
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('интервал опроса вне 10-120 блокирует сохранение', async () => {
    const wrapper = await mountView();
    const store = useDeletionsStore();
    const spy = vi.spyOn(store, 'setDurations');
    const notifySpy = vi.spyOn(store, 'notify');

    setNotif(wrapper.vm, { del: 10, res: 5, poll: 5 });
    await wrapper.vm.saveNotificationSettings();

    expect(updateSetting).not.toHaveBeenCalled();
    expect(spy).not.toHaveBeenCalled();
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });
});
