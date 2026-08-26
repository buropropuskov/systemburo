import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn().mockResolvedValue({ ok: true, json: async () => [] });
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
  createPersonBlacklist: vi.fn().mockResolvedValue({}),
  checkVehicleBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
  createVehicleBlacklist: vi.fn().mockResolvedValue({}),
}));
vi.mock('exceljs', () => ({ default: {} }));

import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import VehicleDetailsModal from '../VehicleDetailsModal.vue';

const stubs = {
  teleport: true,
  TableInfoModal: true,
  EmployeeHistoryModal: true,
  CarHistoryModal: true,
  AddToBlacklistModal: true,
  UnloadPlaceInfoModal: true,
};

// «Запись закреплена за» в карточке видит только администратор, но роль здесь не
// проверяется: сервер отдаёт значение лишь ему (maskEmployeeOwners/maskCarOwners), и
// подпись гейтится наличием значения. В значении приходит ФИО владельца, а у не давшего
// согласия на обработку своих данных - его логин с собачкой. Карточка живёт в заявке, проходной, реестре и на
// странице чёрного списка - перечислять контексты пришлось бы заново при каждом новом.
describe('Карточки сотрудника и машины - строка владельца записи', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockClear();
  });

  it('сотрудник: логин пришёл - строка есть', () => {
    const wrapper = mount(EmployeeDetailsModal, {
      props: { show: true, employee: { id: 1, last_name: 'Пешков', first_name: 'Иван', user_name: 'megobari' }, source: 'employeesview' },
      global: { stubs },
    });
    const row = wrapper.find('[data-testid="employee-owner-login"]');
    expect(row.exists()).toBe(true);
    // Подпись под блоком, а не строка наравне с данными человека: сведения служебные.
    expect(row.text()).toBe('Запись закреплена за: megobari');
  });

  it('сотрудник: дата согласия печатается днём, а не «Invalid Date»', () => {
    // formatDate карточки рассчитан на «ГГГГ-ММ-ДД» из полей срока заявки и на полной
    // метке времени согласия ломался - живая проверка показала «получено Invalid Date».
    const wrapper = mount(EmployeeDetailsModal, {
      props: {
        show: true,
        employee: { id: 1, last_name: 'Пешков', first_name: 'Иван', pd_consent_at: '2026-08-17T20:30:00Z' },
        source: 'employeesview',
      },
      global: { stubs },
    });
    const row = wrapper.find('[data-testid="employee-pd-consent-date"]');
    expect(row.exists()).toBe(true);
    expect(row.text()).toContain('17.08.2026');
    expect(row.text()).not.toContain('Invalid');
  });

  it('сотрудник: логина нет - строки нет', () => {
    const wrapper = mount(EmployeeDetailsModal, {
      props: { show: true, employee: { id: 1, last_name: 'Пешков', first_name: 'Иван' }, source: 'employeesview' },
      global: { stubs },
    });
    expect(wrapper.find('[data-testid="employee-owner-login"]').exists()).toBe(false);
  });

  it('машина: логин пришёл - строка есть, иначе нет', () => {
    const withOwner = mount(VehicleDetailsModal, {
      props: { show: true, vehicle: { id: 1, plateNumber: 'А111АА777', mark: 'Volvo', user_name: 'megobari' }, source: 'carsview' },
      global: { stubs },
    });
    const row = withOwner.find('[data-testid="vehicle-owner-login"]');
    expect(row.exists()).toBe(true);
    expect(row.text()).toBe('Запись закреплена за: megobari');

    const withoutOwner = mount(VehicleDetailsModal, {
      props: { show: true, vehicle: { id: 1, plateNumber: 'А111АА777', mark: 'Volvo' }, source: 'carsview' },
      global: { stubs },
    });
    expect(withoutOwner.find('[data-testid="vehicle-owner-login"]').exists()).toBe(false);
  });
});
