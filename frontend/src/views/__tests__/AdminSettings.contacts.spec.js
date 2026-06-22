import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const getSettings = vi.fn();
const updateSetting = vi.fn();
const getPublicContacts = vi.fn();
vi.mock('@/api/settings', () => ({
  getSettings: (...a) => getSettings(...a),
  updateSetting: (...a) => updateSetting(...a),
  getPublicContacts: (...a) => getPublicContacts(...a),
}));

import AdminSettings from '../AdminSettings.vue';

async function mountView() {
  getSettings.mockResolvedValue([
    { key: 'contacts.bureau_phone', value: '+7 (495) 111-22-33', type: 'string' },
    { key: 'contacts.bureau_email', value: 'bureau@example.com', type: 'string' },
  ]);
  updateSetting.mockResolvedValue({});
  getPublicContacts.mockResolvedValue({ phone: '', email: '' });
  const wrapper = shallowMount(AdminSettings);
  await flushPromises();
  return wrapper;
}

describe('AdminSettings - контакты Бюро', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getSettings.mockReset();
    updateSetting.mockReset();
    getPublicContacts.mockReset();
  });

  it('маппит ключи contacts.* из ответа', async () => {
    const wrapper = await mountView();
    expect(wrapper.vm.settings.bureau_phone).toBe('+7 (495) 111-22-33');
    expect(wrapper.vm.settings.bureau_email).toBe('bureau@example.com');
  });

  it('saveContactsSettings шлёт телефон и почту', async () => {
    const wrapper = await mountView();
    await wrapper.vm.saveContactsSettings();
    const keys = updateSetting.mock.calls.map((c) => c[0]);
    expect(keys).toEqual(expect.arrayContaining(['contacts.bureau_phone', 'contacts.bureau_email']));
  });

  it('saveContactsSettings отклоняет некорректный email без запроса', async () => {
    const wrapper = await mountView();
    wrapper.vm.settings.bureau_email = 'not-an-email';
    wrapper.vm.settings.bureau_phone = '';
    updateSetting.mockClear();
    await wrapper.vm.saveContactsSettings();
    expect(updateSetting).not.toHaveBeenCalled();
  });
});
