import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue({ success: false, error: 'x' }) }),
}));
vi.mock('@/api/blacklist', () => ({
  listPersonBlacklist: vi.fn().mockResolvedValue([]),
  listVehicleBlacklist: vi.fn().mockResolvedValue([]),
}));
vi.mock('@/api/employees', () => ({
  getUniqueEmployeesPaginated: vi.fn().mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } }),
}));
vi.mock('@/api/cars', () => ({
  getUniqueCarsPaginated: vi.fn().mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } }),
  getCarUnloadPlaces: vi.fn().mockResolvedValue([]),
}));
// Права реестра выданы: проверяем именно признак администратора, а не гранты.
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true }),
}));

import EmployeeView from '../EmployeeView.vue';
import CarsView from '../CarsView.vue';

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  EmployeeEditModal: true,
  EmployeeDetailsModal: true,
  VehicleDetailsModal: true,
  ConfirmationModal: true,
  BaseModal: true,
  ApplicationDetail: true,
};

const mocks = { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } };

// Пользователь бюро: организация 10. Запись контрагента живёт в организации 55 у
// пользователя 42 - ни один из признаков принадлежности не совпадает.
function ownership(canManageAll) {
  return { user_id: 1, has_organization: true, has_company: false, organization_id: 10, company_id: null, can_manage_all: canManageAll };
}
const FOREIGN_EMPLOYEE = { id: 7, user_id: 42, organization_id: 55, company_id: null };
const FOREIGN_CAR = { id: 9, user_id: 42, organization_id: 55, company_id: null };

let wrapper;

// Вкладка «Все в системе» была read-only для всех (PR #198). Бюро обязано чинить и
// убирать записи контрагентов, поэтому администратору правка и удаление открыты. Признак
// берётся из ownership-info - того же ответа, по которому решает сервер: разъехавшись,
// FE и BE дали бы «кнопка есть, а в ответ 403».
describe('Реестры сотрудников и машин - администратор правит любую запись', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  afterEach(() => {
    wrapper?.unmount();
  });

  it('сотрудники: администратору кнопки правки и удаления доступны на вкладке «Все в системе»', async () => {
    wrapper = mount(EmployeeView, { global: { stubs, mocks } });
    await wrapper.setData({ ownershipInfo: ownership(true), currentFilter: 'all_system' });

    expect(wrapper.vm.canEditEmployee(FOREIGN_EMPLOYEE)).toBe(true);
    expect(wrapper.vm.showEditEmployee(FOREIGN_EMPLOYEE)).toBe(true);
    expect(wrapper.vm.showDeleteEmployee(FOREIGN_EMPLOYEE)).toBe(true);
    expect(wrapper.vm.employeeBelongsToUser(FOREIGN_EMPLOYEE)).toBe(false);
  });

  it('сотрудники: без признака администратора вкладка остаётся только для просмотра', async () => {
    wrapper = mount(EmployeeView, { global: { stubs, mocks } });
    await wrapper.setData({ ownershipInfo: ownership(false), currentFilter: 'all_system' });

    expect(wrapper.vm.canEditEmployee(FOREIGN_EMPLOYEE)).toBe(false);
    expect(wrapper.vm.showEditEmployee(FOREIGN_EMPLOYEE)).toBe(false);
    expect(wrapper.vm.canEditTooltip(FOREIGN_EMPLOYEE)).toContain('только администратору');
  });

  it('машины: администратору кнопки правки и удаления доступны, привязка считается отдельно', async () => {
    wrapper = mount(CarsView, { global: { stubs, mocks } });
    await wrapper.setData({ ownershipInfo: ownership(true), currentFilter: 'all_system' });

    expect(wrapper.vm.canEditCar(FOREIGN_CAR)).toBe(true);
    expect(wrapper.vm.showEditCar(FOREIGN_CAR)).toBe(true);
    expect(wrapper.vm.showDeleteCar(FOREIGN_CAR)).toBe(true);
    expect(wrapper.vm.carBelongsToUser(FOREIGN_CAR)).toBe(false);
  });

  it('машины: без признака администратора правка закрыта', async () => {
    wrapper = mount(CarsView, { global: { stubs, mocks } });
    await wrapper.setData({ ownershipInfo: ownership(false), currentFilter: 'all_system' });

    expect(wrapper.vm.canEditCar(FOREIGN_CAR)).toBe(false);
    expect(wrapper.vm.canEditTooltip(FOREIGN_CAR)).toContain('только администратору');
  });

  it('машины: форма правки чужой записи не подставляет привязку правящего', async () => {
    wrapper = mount(CarsView, { global: { stubs, mocks } });
    await wrapper.setData({
      ownershipInfo: ownership(true),
      currentFilter: 'all_system',
      editingCar: { ...FOREIGN_CAR, number: 'А111АА777', mark: 'Volvo' },
    });

    expect(wrapper.vm.editingForeignRecord).toBe(true);
    // Своя запись - режим обычной правки с переключателями привязки.
    await wrapper.setData({ editingCar: { id: 3, user_id: 1, organization_id: 10, company_id: null } });
    expect(wrapper.vm.editingForeignRecord).toBe(false);
  });
});
