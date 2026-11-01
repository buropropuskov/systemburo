import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import UserProfileHeader from '../UserProfileHeader.vue';

vi.mock('@/api/pdConsent', () => ({
  listMyConsents: vi.fn().mockResolvedValue([]),
  revokeMyConsent: vi.fn(),
}));
vi.mock('@/api/settings', () => ({
  getPublicContacts: vi.fn().mockResolvedValue({}),
  getPasswordPolicy: vi.fn().mockResolvedValue({ min_length: 8 }),
}));
let wrapper;
const writeText = vi.fn().mockResolvedValue(undefined);
beforeEach(() => {
  setActivePinia(createPinia());
  writeText.mockClear();
  vi.stubGlobal('navigator', { clipboard: { writeText } });
});
afterEach(() => { wrapper?.unmount(); vi.unstubAllGlobals(); });

describe('UserProfileHeader contact actions', () => {
  it('uses non-submit native buttons and copies each contact once', async () => {
    wrapper = mount(UserProfileHeader, {
      props: { email: 'office@example.invalid', phone: '+7 (000) 123-45-67', userType: 'user' },
      global: { stubs: { ChangePasswordModal: true } },
    });
    await flushPromises();
    const email = wrapper.get('.email-badge');
    const phone = wrapper.get('.phone-badge');
    for (const button of [email, phone]) {
      expect(button.element.tagName).toBe('BUTTON');
      expect(button.attributes('type')).toBe('button');
      expect(button.element.tabIndex).toBe(0);
    }
    await email.trigger('click');
    await flushPromises();
    expect(writeText).toHaveBeenNthCalledWith(1, 'office@example.invalid');
    expect(email.attributes('aria-label')).toBe('Почта скопирована');
    await phone.trigger('click');
    await flushPromises();
    expect(writeText).toHaveBeenNthCalledWith(2, '70001234567');
    expect(writeText).toHaveBeenCalledTimes(2);
    expect(phone.attributes('aria-label')).toBe('Телефон скопирован');
  });

  it('does not add focus stops when contacts are absent', async () => {
    wrapper = mount(UserProfileHeader, {
      props: { userType: 'user' },
      global: { stubs: { ChangePasswordModal: true } },
    });
    await flushPromises();
    expect(wrapper.find('.email-badge').exists()).toBe(false);
    expect(wrapper.find('.phone-badge').exists()).toBe(false);
    expect(wrapper.get('[data-testid="cabinet-change-password"]').element.tagName).toBe('BUTTON');
  });
});
