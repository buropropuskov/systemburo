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

async function mountView() {
  getSettings.mockResolvedValue([
    { key: 'password.min_length', value: '10', type: 'int' },
    { key: 'password.require_special', value: 'true', type: 'bool' },
  ]);
  updateSetting.mockResolvedValue({});
  const wrapper = shallowMount(AdminSettings);
  await flushPromises();
  return wrapper;
}

describe('AdminSettings - раздел безопасности', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getSettings.mockReset();
    updateSetting.mockReset();
  });

  it('маппит ключи password.* из ответа', async () => {
    const wrapper = await mountView();
    expect(wrapper.vm.settings.password_min_length).toBe(10);
    expect(wrapper.vm.settings.password_require_special).toBe(true);
  });

  it('saveSecuritySettings шлёт все ключи политики', async () => {
    const wrapper = await mountView();
    await wrapper.vm.saveSecuritySettings();
    const keys = updateSetting.mock.calls.map((c) => c[0]);
    expect(keys).toEqual(expect.arrayContaining([
      'password.min_length',
      'password.require_letter',
      'password.require_uppercase',
      'password.require_lowercase',
      'password.require_digit',
      'password.require_special',
    ]));
  });
});
