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

import { getOrganizations, getCompanies } from '@/api/organizations';
import { createManualCars } from '@/api/cars';
import { createManualEmployees } from '@/api/employees';
import { useDeletionsStore } from '@/stores/deletions';
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
