import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({ id: 1 }) });
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import EmployeeEditModal from '../EmployeeEditModal.vue';

const CITIZENSHIPS = [{ id: 1, name: 'Россия', patent_required: false }];

const OWNERSHIP_INFO = {
  user_id: 1,
  has_organization: true,
  has_company: true,
  organization_id: 10,
  organization_name: 'ООО Ромашка',
  company_id: 20,
  company_name: 'Компания Лютик',
};

function mountModal() {
  return mount(EmployeeEditModal, {
    props: { visible: true, citizenships: CITIZENSHIPS, ownershipInfo: OWNERSHIP_INFO },
    global: { stubs: { teleport: true } },
  });
}

// #1097 w9, п.4: владелец хочет привязывать сотрудника И к организации, И к компании
// одновременно. У CarsView (машины) чекбоксы взаимоисключают друг друга через :disabled
// и watch-сброс - у сотрудника в разметке этого нет ни на инпутах, ни в script (нет
// :disabled, нет watch на bindToOrganization/bindToCompany), и бэк (unique_employee_service,
// модель UniqueEmployee) не проверяет XOR organization_id/company_id ни в create, ни в
// update. Ограничения не было ни на фронте, ни на бэке - замок фиксирует это как контракт.
describe('EmployeeEditModal — привязка к организации и компании не взаимоисключающая', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockClear();
  });

  it('чекбоксы привязки не несут disabled независимо от состояния друг друга', async () => {
    const wrapper = mountModal();
    const orgCheckbox = wrapper.findAll('.binding-option input[type="checkbox"]')[0];
    const companyCheckbox = wrapper.findAll('.binding-option input[type="checkbox"]')[1];

    await orgCheckbox.setValue(true);
    expect(companyCheckbox.attributes('disabled')).toBeUndefined();

    await companyCheckbox.setValue(true);
    expect(orgCheckbox.attributes('disabled')).toBeUndefined();
  });

  it('обе привязки остаются отмеченными одновременно - вторая не сбрасывает первую', async () => {
    const wrapper = mountModal();
    const orgCheckbox = wrapper.findAll('.binding-option input[type="checkbox"]')[0];
    const companyCheckbox = wrapper.findAll('.binding-option input[type="checkbox"]')[1];

    await orgCheckbox.setValue(true);
    await companyCheckbox.setValue(true);

    expect(wrapper.vm.bindToOrganization).toBe(true);
    expect(wrapper.vm.bindToCompany).toBe(true);
    expect(orgCheckbox.element.checked).toBe(true);
    expect(companyCheckbox.element.checked).toBe(true);
  });

  it('сохранение отправляет одновременно organization_id и company_id, когда отмечены обе привязки', async () => {
    const wrapper = mountModal();
    await wrapper.setData({
      lastName: 'Иванов',
      firstName: 'Иван',
      position: 'Монтажник',
      pdConsent: true,
      passportSeriesNumber: '1234 567890',
      selectedCitizenship: CITIZENSHIPS[0],
    });

    const orgCheckbox = wrapper.findAll('.binding-option input[type="checkbox"]')[0];
    const companyCheckbox = wrapper.findAll('.binding-option input[type="checkbox"]')[1];
    await orgCheckbox.setValue(true);
    await companyCheckbox.setValue(true);

    await wrapper.vm.saveEmployee();

    expect(apiRequest).toHaveBeenCalledWith('/unique-employees', expect.objectContaining({
      method: 'POST',
      body: expect.stringMatching(/"organization_id":10/),
    }));
    const [, options] = apiRequest.mock.calls[0];
    const body = JSON.parse(options.body);
    expect(body.organization_id).toBe(10);
    expect(body.company_id).toBe(20);
  });
});
