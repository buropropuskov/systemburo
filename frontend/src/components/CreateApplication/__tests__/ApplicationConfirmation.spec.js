import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ApplicationConfirmation from '../ApplicationConfirmation.vue';
import { getConsentGate } from '@/api/pdConsent';

vi.mock('@/api/pdConsent', () => ({ getConsentGate: vi.fn() }));

const props = { label: 'Подтверждение', testId: 'confirmation' };

describe('ApplicationConfirmation', () => {
  beforeEach(() => getConsentGate.mockReset());

  it('передаёт редакцию показанного текста и сбрасывает прежнюю отметку', async () => {
    getConsentGate.mockResolvedValue({ version: 4, hash: 'new', text: '<p>Актуальный текст</p>' });
    const wrapper = mount(ApplicationConfirmation, {
      props: { ...props, modelValue: { version: 4, hash: 'old' } },
      global: { stubs: { BaseModal: true } },
    });
    await flushPromises();
    expect(wrapper.emitted('update:modelValue')).toEqual([[null]]);
    await wrapper.setProps({ modelValue: null });
    await wrapper.get('[data-testid="confirmation"]').setValue(true);
    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual([{ version: 4, hash: 'new' }]);
  });

  it('позволяет повторить загрузку после ошибки', async () => {
    getConsentGate.mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ version: 4, hash: 'new', text: '<p>Актуальный текст</p>' });
    const wrapper = mount(ApplicationConfirmation, {
      props,
      global: { stubs: { BaseModal: true } },
    });
    await flushPromises();
    expect(wrapper.find('[data-testid="confirmation"]').exists()).toBe(false);
    await wrapper.get('button').trigger('click');
    await flushPromises();
    expect(wrapper.find('[data-testid="confirmation"]').exists()).toBe(true);
  });
});
