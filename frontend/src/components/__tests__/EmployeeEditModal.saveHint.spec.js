import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}));

import EmployeeEditModal from '../EmployeeEditModal.vue';

const CITIZENSHIPS = [
  { id: 1, name: 'Россия', patent_required: false },
  { id: 2, name: 'Узбекистан', patent_required: true },
];

function mountModal() {
  return mount(EmployeeEditModal, {
    props: { visible: true, citizenships: CITIZENSHIPS },
    global: { stubs: { teleport: true } },
  });
}

describe('EmployeeEditModal — подсказка на кнопке сохранения', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  // Гражданство форма подставляет сама (resetForm берёт дефолтное), поэтому
  // на чистой форме в подсказке остаются только руками заполняемые поля.
  it('на пустой форме перечисляет незаполненные поля', () => {
    const wrapper = mountModal();
    expect(wrapper.vm.saveEmployeeHint).toBe(
      'Заполните: фамилию, имя, должность, серию и номер паспорта. '
      + 'Отметьте согласие работника на обработку персональных данных'
    );
  });

  it('гражданство с патентом объясняется отдельной причиной', async () => {
    const wrapper = mountModal();
    await wrapper.setData({
      selectedCitizenship: CITIZENSHIPS[1],
      lastName: 'Иванов',
      firstName: 'Иван',
      position: 'Монтажник',
      passportSeriesNumber: '1234 567890',
      // Согласие субъекта отмечено: иначе в подсказке было бы две причины, а кейс
      // проверяет именно формулировку про патент.
      pdConsent: true,
    });

    expect(wrapper.vm.canSaveEmployee).toBe(false);
    expect(wrapper.vm.saveEmployeeHint).toBe(
      'Для этого гражданства нужен номер патента или иное разрешение на работы'
    );

    await wrapper.setData({ patentNumber: '77-123' });
    expect(wrapper.vm.canSaveEmployee).toBe(true);
    expect(wrapper.vm.saveEmployeeHint).toBe('');
  });

  it('data-hint висит на обёртке вокруг заблокированной кнопки', () => {
    const wrapper = mountModal();
    const anchor = wrapper.find('.hint-anchor');

    expect(anchor.attributes('data-hint')).toContain('Заполните:');
    expect(anchor.find('.add-button').attributes('disabled')).toBeDefined();
  });
});
