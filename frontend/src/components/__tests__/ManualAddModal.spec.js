import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/organizations', () => ({
  getOrganizations: vi.fn(),
  getCompanies: vi.fn(),
}));
vi.mock('@/api/cars', () => ({
  createManualCars: vi.fn(),
}));
vi.mock('@/api/employees', () => ({
  createManualEmployees: vi.fn(),
}));
vi.mock('@/api/applications', () => ({
  getAttachableApplications: vi.fn(),
  getApplicationAttachments: vi.fn(),
}));
vi.mock('@/api/attachments', () => ({
  attachToApplication: vi.fn(),
}));

import { getOrganizations, getCompanies } from '@/api/organizations';
import { createManualCars } from '@/api/cars';
import { createManualEmployees } from '@/api/employees';
import { getAttachableApplications, getApplicationAttachments } from '@/api/applications';
import { attachToApplication } from '@/api/attachments';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import ManualAddModal from '../ManualAddModal.vue';

const stubs = {
  VehicleForm: true,
  EmployeeForm: true,
  DateRangeSection: true,
  BaseDropdown: true,
  teleport: true,
  transition: false,
};

function mountModal(props = {}) {
  return mount(ManualAddModal, {
    props: { show: true, tableId: 42, tableName: 'КПП 1', ...props },
    global: { stubs },
  });
}

describe('ManualAddModal (#1049 S8)', () => {
  let notifySpy;

  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    getOrganizations.mockResolvedValue([{ id: 7, name: 'ООО Ромашка' }]);
    getCompanies.mockResolvedValue([{ id: 3, name: 'Компания А' }]);
    createManualCars.mockResolvedValue({ success: true, attachment_id: 100, car_ids: [1, 2] });
    createManualEmployees.mockResolvedValue({ success: true, attachment_id: 200, employee_ids: [10, 11] });
    notifySpy = vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    document.body.style.overflow = '';
  });

  it('маппит payload VehicleForm -> ManualCarRequest DTO (passage_tables -> target_tables)', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await wrapper.setData({
      selectedOrgId: 7,
      selectedCompanyId: 3,
      dateData: {
        isOneDay: false,
        startDate: '01.06.2026',
        endDate: '05.06.2026',
        singleDate: '',
        startTime: '08:00',
        endTime: '18:00',
        roofAccess: true,
        freeParking: false,
      },
      addedVehicles: [{
        id: 1,
        plateNumber: 'А123ВС77',
        mark: 'BMW',
        markId: 9,
        markName: 'BMW',
        unloadingPlace: 'Склад 1',
        unloadPlaces: [11, 12],
        passage_tables: [21],
      }],
    });

    await wrapper.vm.submit();

    expect(createManualCars).toHaveBeenCalledTimes(1);
    const payload = createManualCars.mock.calls[0][0];
    expect(payload).toMatchObject({
      organization_id: 7,
      company_id: 3,
      table_id: 42,
      entry_date_from: '2026-06-01',
      entry_date_to: '2026-06-05',
      entry_time_from: '08:00:00',
      entry_time_to: '18:00:00',
      roof_access: true,
      free_parking: false,
    });
    expect(payload.vehicles).toHaveLength(1);
    expect(payload.vehicles[0]).toEqual({
      car_number: 'А123ВС77',
      car_brand: 'BMW',
      mark_id: 9,
      mark_name: 'BMW',
      unload_place: 'Склад 1',
      unload_places: [11, 12],
      target_tables: [21],
    });
    expect(wrapper.emitted('added')).toBeTruthy();
    expect(wrapper.emitted('added')[0][0]).toMatchObject({ attachment_id: 100 });
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('single-day: singleDate уходит в entry_date_from и entry_date_to', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await wrapper.setData({
      selectedOrgId: 7,
      dateData: {
        isOneDay: true,
        startDate: '',
        endDate: '',
        singleDate: '10.06.2026',
        startTime: '09:00',
        endTime: '17:00',
        roofAccess: false,
        freeParking: false,
      },
      addedVehicles: [{ id: 1, plateNumber: 'А1', mark: 'X', unloadPlaces: [], passage_tables: [] }],
    });

    await wrapper.vm.submit();

    const payload = createManualCars.mock.calls[0][0];
    expect(payload.entry_date_from).toBe('2026-06-10');
    expect(payload.entry_date_to).toBe('2026-06-10');
    expect(payload.company_id).toBeNull();
  });

  it('без организации не отправляет и показывает ошибку', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await wrapper.setData({
      selectedOrgId: null,
      addedVehicles: [{ id: 1, plateNumber: 'А1', mark: 'X', unloadPlaces: [], passage_tables: [] }],
      dateData: { isOneDay: true, singleDate: '10.06.2026', startTime: '09:00', endTime: '17:00' },
    });

    await wrapper.vm.submit();

    expect(createManualCars).not.toHaveBeenCalled();
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('без даты/времени не отправляет', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await wrapper.setData({
      selectedOrgId: 7,
      addedVehicles: [{ id: 1, plateNumber: 'А1', mark: 'X', unloadPlaces: [], passage_tables: [] }],
      dateData: { isOneDay: false, startDate: '', endDate: '', startTime: '', endTime: '' },
    });

    await wrapper.vm.submit();

    expect(createManualCars).not.toHaveBeenCalled();
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('ошибка бэка показывается юзеру, added не эмитится', async () => {
    createManualCars.mockRejectedValue(new Error('Организация не найдена'));
    const wrapper = mountModal();
    await flushPromises();
    await wrapper.setData({
      selectedOrgId: 7,
      dateData: { isOneDay: true, singleDate: '10.06.2026', startTime: '09:00', endTime: '17:00' },
      addedVehicles: [{ id: 1, plateNumber: 'А1', mark: 'X', unloadPlaces: [], passage_tables: [] }],
    });

    await wrapper.vm.submit();

    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Организация не найдена', type: 'error' }));
    expect(wrapper.emitted('added')).toBeFalsy();
  });

  it('форма машины заблокирована с видимой подсказкой до выбора организации', async () => {
    const wrapper = mountModal();
    await flushPromises();
    // без организации VehicleForm inert (disabled) - показываем подсказку, иначе форма
    // выглядит активной, но не реагирует (баг «курсор не реагирует»)
    const lock = wrapper.find('[data-testid="manual-form-lock"]');
    expect(lock.exists()).toBe(true);
    expect(lock.text()).toMatch(/организаци/i);
    // после выбора организации подсказка исчезает, форма разблокирована
    await wrapper.setData({ selectedOrgId: 7 });
    expect(wrapper.find('[data-testid="manual-form-lock"]').exists()).toBe(false);
  });

  it('vehicle-added накапливает список, удаление убирает', async () => {
    const wrapper = mountModal();
    await flushPromises();
    wrapper.vm.handleVehicleAdded({ plateNumber: 'А1', mark: 'BMW', unloadPlaces: [], passage_tables: [] });
    wrapper.vm.handleVehicleAdded({ plateNumber: 'В2', mark: 'Audi', unloadPlaces: [], passage_tables: [] });
    expect(wrapper.vm.addedVehicles).toHaveLength(2);
    wrapper.vm.removeVehicle(wrapper.vm.addedVehicles[0]);
    expect(wrapper.vm.addedVehicles).toHaveLength(1);
    expect(wrapper.vm.addedVehicles[0].plateNumber).toBe('В2');
  });
});

describe('ManualAddModal - people-режим (#1049 S9)', () => {
  let notifySpy;

  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    getOrganizations.mockResolvedValue([{ id: 7, name: 'ООО Ромашка' }]);
    getCompanies.mockResolvedValue([{ id: 3, name: 'Компания А' }]);
    createManualEmployees.mockResolvedValue({ success: true, attachment_id: 200, employee_ids: [10, 11] });
    notifySpy = vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    document.body.style.overflow = '';
  });

  function mountPeople(props = {}) {
    return mount(ManualAddModal, {
      props: { show: true, mode: 'people', tableId: 55, tableName: 'КПП людей', ...props },
      global: { stubs },
    });
  }

  it('маппит payload EmployeeForm -> ManualEmployeeRequest DTO (camelCase -> snake_case), зовёт createManualEmployees', async () => {
    const wrapper = mountPeople();
    await flushPromises();
    await wrapper.setData({
      selectedOrgId: 7,
      selectedCompanyId: 3,
      dateData: {
        isOneDay: false,
        startDate: '01.06.2026',
        endDate: '05.06.2026',
        singleDate: '',
        startTime: '08:00',
        endTime: '18:00',
        roofAccess: false,
        freeParking: false,
      },
      addedEmployees: [{
        id: 1,
        lastName: 'Иванов',
        firstName: 'Иван',
        middleName: 'Иванович',
        position: 'Водитель',
        citizenshipId: 4,
        passportSeriesNumber: '1234 567890',
        patentNumber: null,
        otherPermission: null,
        targetTables: [21, 22],
      }],
    });

    await wrapper.vm.submit();

    expect(createManualCars).not.toHaveBeenCalled();
    expect(createManualEmployees).toHaveBeenCalledTimes(1);
    const payload = createManualEmployees.mock.calls[0][0];
    expect(payload).toMatchObject({
      organization_id: 7,
      company_id: 3,
      table_id: 55,
      entry_date_from: '2026-06-01',
      entry_date_to: '2026-06-05',
      entry_time_from: '08:00:00',
      entry_time_to: '18:00:00',
    });
    // roof_access/free_parking не уходят для people (нет в DTO)
    expect(payload.roof_access).toBeUndefined();
    expect(payload.free_parking).toBeUndefined();
    expect(payload.employees).toHaveLength(1);
    expect(payload.employees[0]).toEqual({
      last_name: 'Иванов',
      first_name: 'Иван',
      middle_name: 'Иванович',
      citizenship_id: 4,
      position: 'Водитель',
      passport_series_number: '1234 567890',
      patent_number: null,
      other_permission: null,
      target_tables: [21, 22],
    });
    expect(wrapper.emitted('added')[0][0]).toMatchObject({ attachment_id: 200 });
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('EmployeeForm получает allow-existing-search=false (дубль без existing_employee_id)', async () => {
    const wrapper = mountPeople();
    await flushPromises();
    await wrapper.setData({ selectedOrgId: 7 });
    const form = wrapper.findComponent({ name: 'EmployeeForm' });
    expect(form.exists()).toBe(true);
    expect(form.props('allowExistingSearch')).toBe(false);
  });

  it('без сотрудников не отправляет, показывает ошибку', async () => {
    const wrapper = mountPeople();
    await flushPromises();
    await wrapper.setData({
      selectedOrgId: 7,
      addedEmployees: [],
      dateData: { isOneDay: true, singleDate: '10.06.2026', startTime: '09:00', endTime: '17:00' },
    });

    await wrapper.vm.submit();

    expect(createManualEmployees).not.toHaveBeenCalled();
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('employee-added накапливает список, удаление убирает', async () => {
    const wrapper = mountPeople();
    await flushPromises();
    wrapper.vm.handleEmployeeAdded({ lastName: 'Иванов', firstName: 'Иван', targetTables: [] });
    wrapper.vm.handleEmployeeAdded({ lastName: 'Петров', firstName: 'Пётр', targetTables: [] });
    expect(wrapper.vm.addedEmployees).toHaveLength(2);
    wrapper.vm.removeEmployee(wrapper.vm.addedEmployees[0]);
    expect(wrapper.vm.addedEmployees).toHaveLength(1);
    expect(wrapper.vm.addedEmployees[0].lastName).toBe('Петров');
  });
});

describe('ManualAddModal - режим-2 привязка к заявке (#1049 S10)', () => {
  let notifySpy;

  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    getOrganizations.mockResolvedValue([{ id: 7, name: 'ООО Ромашка' }]);
    getCompanies.mockResolvedValue([{ id: 3, name: 'Компания А' }]);
    createManualCars.mockResolvedValue({ success: true, attachment_id: 100, car_ids: [1] });
    createManualEmployees.mockResolvedValue({ success: true, attachment_id: 200, employee_ids: [10] });
    getAttachableApplications.mockResolvedValue([
      { id: 5, application_number: '№ 5/2026', organization_name: 'ООО Ромашка', confirmation: 'Согласовано', status: 'В работе' },
      { id: 6, application_number: '№ 6/2026', organization_name: 'ООО Прочее', confirmation: 'На согласовании', status: 'В работе' },
    ]);
    getApplicationAttachments.mockResolvedValue([
      { id: 51, attachment_type: 'cars', attachment_display_name: 'Автозаявка', entry_date_from: '2026-06-01', entry_date_to: '2026-06-30' },
      { id: 52, attachment_type: 'people', attachment_display_name: 'Люди', entry_date_from: '2026-06-01', entry_date_to: '2026-06-30' },
    ]);
    attachToApplication.mockResolvedValue({ success: true, application_id: 5, attachment_id: 100 });
    notifySpy = vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    usePermissionsStore().mode = 'super'; // canAttach=true, переключатель виден
    document.body.style.overflow = '';
  });

  function fillValidCar(wrapper) {
    return wrapper.setData({
      selectedOrgId: 7,
      dateData: { isOneDay: true, singleDate: '10.06.2026', startTime: '09:00', endTime: '17:00' },
      addedVehicles: [{ id: 1, plateNumber: 'А1', mark: 'X', unloadPlaces: [], passage_tables: [] }],
    });
  }

  it('переключатель скрыт без права page.admin (normal mode)', async () => {
    usePermissionsStore().mode = 'normal';
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.canAttach).toBe(false);
    expect(wrapper.find('[data-testid="manual-add-mode"]').exists()).toBe(false);
  });

  it('super видит переключатель; bindMode=application грузит заявки через attachable-эндпоинт', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.canAttach).toBe(true);
    expect(wrapper.find('[data-testid="manual-add-mode"]').exists()).toBe(true);
    await wrapper.setData({ bindMode: 'application' });
    await flushPromises();
    // грузит через /applications/attachable (super/admin, без скоупа по участию),
    // сервер сам форсит активные согласованные
    expect(getAttachableApplications).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.applicationOptions.map(o => o.id)).toEqual([5]); // клиентский фильтр-страховка
    // лейбл берёт organization_name (реальное поле ответа), а не organization,
    // и показывает application_number как есть - без двойного «№» (номер уже с «№»)
    expect(wrapper.vm.applicationOptions[0].label).toBe('№ 5/2026 - ООО Ромашка');
    expect(wrapper.vm.applicationOptions[0].label).not.toContain('№ №');
  });

  it('adopt (новое вложение): create -> attachToApplication {applicationId}', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await fillValidCar(wrapper);
    await wrapper.setData({ bindMode: 'application' });
    await flushPromises();
    await wrapper.setData({ selectedApplicationId: 5 });

    await wrapper.vm.submit();

    expect(createManualCars).toHaveBeenCalledTimes(1);
    expect(attachToApplication).toHaveBeenCalledWith(100, { applicationId: 5 });
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
    expect(wrapper.emitted('added')).toBeTruthy();
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('reattach (существующее): фильтр вложений по типу cars, attach {targetAttachmentId}', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await fillValidCar(wrapper);
    await wrapper.setData({ bindMode: 'application' });
    await flushPromises();
    wrapper.vm.onApplicationChange(5);
    await flushPromises();
    expect(wrapper.vm.attachmentOptions.map(o => o.id)).toEqual([51]); // только cars
    await wrapper.setData({ attachTarget: 'existing', selectedAttachmentId: 51 });

    await wrapper.vm.submit();

    expect(attachToApplication).toHaveBeenCalledWith(100, { targetAttachmentId: 51 });
  });

  it('провал привязки: записи созданы (added эмитится), ошибка привязки показана', async () => {
    attachToApplication.mockRejectedValue(new Error('Период машины вне окна вложения'));
    const wrapper = mountModal();
    await flushPromises();
    await fillValidCar(wrapper);
    await wrapper.setData({ bindMode: 'application', selectedApplicationId: 5 });

    await wrapper.vm.submit();

    expect(createManualCars).toHaveBeenCalledTimes(1);
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Период машины вне окна вложения',
      type: 'error',
    }));
    expect(wrapper.emitted('added')).toBeTruthy(); // созданные записи не теряем
  });

  it('canSubmit требует заявку (и вложение при existing) в режиме привязки', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await fillValidCar(wrapper);
    await wrapper.setData({ bindMode: 'application', selectedApplicationId: null });
    expect(wrapper.vm.canSubmit).toBe(false);
    await wrapper.setData({ selectedApplicationId: 5 });
    expect(wrapper.vm.canSubmit).toBe(true);
    await wrapper.setData({ attachTarget: 'existing', selectedAttachmentId: null });
    expect(wrapper.vm.canSubmit).toBe(false);
    await wrapper.setData({ selectedAttachmentId: 51 });
    expect(wrapper.vm.canSubmit).toBe(true);
  });

  it('подсказка на кнопке называет причину блокировки', async () => {
    const wrapper = mountModal();
    await flushPromises();
    expect(wrapper.vm.submitHint).toBe(
      'Заполните: организацию, даты и время. Добавьте хотя бы одну машину'
    );

    await fillValidCar(wrapper);
    expect(wrapper.vm.submitHint).toBe('');

    await wrapper.setData({ bindMode: 'application', selectedApplicationId: null });
    expect(wrapper.vm.submitHint).toBe('Заполните: заявку');

    // Якорь подсказки - обёртка: у disabled-кнопки :hover не срабатывает.
    const anchor = wrapper.find('.hint-anchor');
    expect(anchor.attributes('data-hint')).toBe('Заполните: заявку');
    expect(anchor.find('[data-testid="manual-add-submit"]').exists()).toBe(true);
  });

  it('people-режим: attachmentOptions фильтрует вложения по типу people', async () => {
    const wrapper = mount(ManualAddModal, {
      props: { show: true, mode: 'people', tableId: 55, tableName: 'КПП людей' },
      global: { stubs },
    });
    await flushPromises();
    wrapper.vm.onApplicationChange(5);
    await flushPromises();
    expect(wrapper.vm.attachmentOptions.map(o => o.id)).toEqual([52]);
  });
});
