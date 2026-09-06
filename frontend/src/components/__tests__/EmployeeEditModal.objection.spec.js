import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import EmployeeEditModal from '../EmployeeEditModal.vue';

// Возражение субъекта (#2361). Сервер отклоняет правку персональных полей, а
// карточка обязана объяснить почему: кнопка, ведущая к отказу, - плохая кнопка,
// а форма без причины читается как сломанная.

const baseEmployee = {
  id: 7,
  last_name: 'Возражаев',
  first_name: 'Пётр',
  position: 'Слесарь',
  passport_series_number: '4501 111111',
  citizenship_id: 1,
  pd_consent_at: '2026-08-19T10:00:00Z',
};

function mountModal(employee) {
  return mount(EmployeeEditModal, {
    props: { visible: true, editingEmployee: employee, citizenships: [{ id: 1, name: 'РФ' }] },
    global: {
      stubs: {
        Teleport: true,
        BaseModal: { template: '<div><slot /></div>' },
        // Управление возражением живёт в отдельном компоненте и проверяется своим
        // тестом; здесь важно поведение самой карточки - запрет сохранения.
        EmployeeObjectionControl: true,
      },
    },
  });
}

describe('EmployeeEditModal — возражение субъекта', () => {
  it('без возражения карточка правится обычным порядком', () => {
    const wrapper = mountModal({ ...baseEmployee });
    expect(wrapper.vm.hasObjection).toBe(false);
    expect(wrapper.vm.canSaveEmployee).not.toBe(false);
  });

  it('с возражением показывает причину и гасит сохранение', () => {
    const wrapper = mountModal({ ...baseEmployee, pd_objection_at: '2026-09-06T09:00:00Z' });

    expect(wrapper.vm.hasObjection).toBe(true);

    expect(wrapper.vm.canSaveEmployee).toBe(false);
    expect(wrapper.vm.saveEmployeeHint).toContain('правка запрещена');
    expect(wrapper.vm.saveEmployeeHint).toContain('администратор бюро');
  });

  it('возражение важнее незаполненных полей: причина названа именно оно', async () => {
    const wrapper = mountModal({ ...baseEmployee, pd_objection_at: '2026-09-06T09:00:00Z' });
    await wrapper.setData({ lastName: '', position: '' });

    // Иначе человек кинется заполнять поля, которые всё равно не сохранятся.
    expect(wrapper.vm.saveEmployeeHint).not.toContain('Заполните');
    expect(wrapper.vm.saveEmployeeHint).toContain('правка запрещена');
  });
});
