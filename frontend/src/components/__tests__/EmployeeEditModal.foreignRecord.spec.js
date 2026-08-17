import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({ id: 7 }) });
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import EmployeeEditModal from '../EmployeeEditModal.vue';

const CITIZENSHIPS = [{ id: 1, name: 'Россия', patent_required: false }];

// Администратор бюро: своя организация 10, своя компания 20.
const ADMIN_OWNERSHIP = {
  user_id: 1,
  has_organization: true,
  has_company: true,
  organization_id: 10,
  organization_name: 'Бюро пропусков',
  company_id: 20,
  company_name: 'Компания бюро',
};

// Запись контрагента: другой владелец, другая организация.
const FOREIGN_EMPLOYEE = {
  id: 7,
  last_name: 'Пешков',
  first_name: 'Иван',
  middle_name: '',
  position: 'Монтажник',
  citizenship_id: 1,
  passport_series_number: '4510 111222',
  patent_number: null,
  other_permission: null,
  organization_id: 55,
  organization_name: 'ООО Подрядчик',
  company_id: null,
  company_name: null,
  user_id: 42,
  user_name: 'megobari',
};

function mountModal(props = {}) {
  return mount(EmployeeEditModal, {
    props: {
      visible: true,
      citizenships: CITIZENSHIPS,
      ownershipInfo: ADMIN_OWNERSHIP,
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

// Администратор правит сотрудника чужой организации (вкладка «Все в системе»). Карточка
// строилась под правку СВОЕЙ записи: user_id и привязку она брала из ownership-info
// правящего, поэтому исправление ФИО перевело бы сотрудника контрагента на бюро вместе
// с его организацией. Замок держит два факта: привязку не отправляем свою и не подменяем
// владельца, а переключатели «привязать к моей организации» в этом режиме не показываем.
describe('EmployeeEditModal - правка записи чужой организации', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockClear();
  });

  it('сохранение не подменяет владельца и переносит привязку записи как есть', async () => {
    const wrapper = mountModal({ editingEmployee: FOREIGN_EMPLOYEE, foreignRecord: true });
    await wrapper.setData({ lastName: 'Пешков', firstName: 'Иоанн', selectedCitizenship: CITIZENSHIPS[0], pdConsent: true });

    await wrapper.vm.saveEmployee();

    expect(apiRequest).toHaveBeenCalledTimes(1);
    const [url, options] = apiRequest.mock.calls[0];
    expect(url).toBe('/unique-employees/7');
    expect(options.method).toBe('PUT');
    const body = JSON.parse(options.body);
    expect(body.first_name).toBe('Иоанн');
    expect(body).not.toHaveProperty('user_id');
    expect(body.organization_id).toBe(55);
    expect(body.company_id).toBeNull();
  });

  it('вместо переключателей привязки показывает, за кем закреплена запись', () => {
    const wrapper = mountModal({ editingEmployee: FOREIGN_EMPLOYEE, foreignRecord: true });

    const note = wrapper.find('[data-testid="employee-foreign-binding-note"]');
    expect(note.exists()).toBe(true);
    expect(note.text()).toContain('megobari');
    expect(note.text()).toContain('ООО Подрядчик');
    expect(wrapper.findAll('.binding-option input[type="checkbox"]')).toHaveLength(0);
  });

  it('своя запись правится по-прежнему: привязка из переключателей и свой user_id', async () => {
    const own = { ...FOREIGN_EMPLOYEE, id: 8, organization_id: 10, user_id: 1, user_name: 'testadmin' };
    const wrapper = mountModal({ editingEmployee: own, foreignRecord: false });
    await wrapper.setData({ lastName: 'Пешков', firstName: 'Иван', selectedCitizenship: CITIZENSHIPS[0], pdConsent: true });

    expect(wrapper.find('[data-testid="employee-foreign-binding-note"]').exists()).toBe(false);
    const orgCheckbox = wrapper.findAll('.binding-option input[type="checkbox"]')[0];
    await orgCheckbox.setValue(true);

    await wrapper.vm.saveEmployee();

    const [, options] = apiRequest.mock.calls[0];
    const body = JSON.parse(options.body);
    expect(body.user_id).toBe(1);
    expect(body.organization_id).toBe(10);
  });
});
