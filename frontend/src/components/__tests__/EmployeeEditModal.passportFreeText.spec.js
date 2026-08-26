import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}));

import EmployeeEditModal from '../EmployeeEditModal.vue';

const CITIZENSHIPS = [
  { id: 1, name: 'Россия', patent_required: false },
];

function mountModal() {
  return mount(EmployeeEditModal, {
    props: { visible: true, citizenships: CITIZENSHIPS },
    global: { stubs: { teleport: true } },
  });
}

describe('EmployeeEditModal — серия и номер паспорта принимают произвольный текст', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('значение с буквами и дефисом проходит без обрезки и позволяет сохранить', async () => {
    const wrapper = mountModal();
    await wrapper.setData({
      lastName: 'Иванов',
      firstName: 'Иван',
      position: 'Монтажник',
      pdConsent: true,
    });

    const input = wrapper.find('input[placeholder="Введите серию и номер паспорта"]');
    await input.setValue('AB-123456 иностр.');

    expect(wrapper.vm.passportSeriesNumber).toBe('AB-123456 иностр.');
    expect(wrapper.vm.canSaveEmployee).toBe(true);
  });

  it('пустое значение по-прежнему блокирует сохранение', async () => {
    const wrapper = mountModal();
    await wrapper.setData({
      lastName: 'Иванов',
      firstName: 'Иван',
      position: 'Монтажник',
      pdConsent: true,
      passportSeriesNumber: '   ',
    });

    expect(wrapper.vm.canSaveEmployee).toBe(false);
  });
});
