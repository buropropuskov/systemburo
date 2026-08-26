import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';

// Пометку ЧС теперь считает сервер и отдаёт в самих строках реестра полем is_blacklisted
// (нормализация ФИО/номера ушла на бэкенд, покрыта go-тестом). Модалка список ЧС в браузер
// не грузит — проверяем, что она читает готовый флаг и блокирует выбор такой строки.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

const stubs = { SearchComponent: true, LoaderSpinner: true };

describe('ExistingCarsModal - гард ЧС', () => {
  it('isCarBlacklisted читает серверный флаг is_blacklisted', () => {
    const wrapper = mount(ExistingCarsModal, { props: { visible: false }, global: { stubs } });
    expect(wrapper.vm.isCarBlacklisted({ number: 'A777AA777', mark: 'BMW', is_blacklisted: true })).toBe(true);
    expect(wrapper.vm.isCarBlacklisted({ number: 'B111BB111', mark: 'BMW', is_blacklisted: false })).toBe(false);
    expect(wrapper.vm.isCarBlacklisted({ number: 'C999CC999', mark: 'Toyota' })).toBe(false);
  });

  it('isCarDisabled = true для машины из ЧС даже без already-added', () => {
    const wrapper = mount(ExistingCarsModal, { props: { visible: false }, global: { stubs } });
    expect(wrapper.vm.isCarDisabled({ id: 1, number: 'A777AA777', mark: 'BMW', is_blacklisted: true })).toBe(true);
    expect(wrapper.vm.isCarDisabled({ id: 2, number: 'C999CC999', mark: 'Toyota', is_blacklisted: false })).toBe(false);
  });
});

describe('ExistingEmployeesModal - гард ЧС', () => {
  it('isEmployeeBlacklisted читает серверный флаг is_blacklisted', () => {
    const wrapper = mount(ExistingEmployeesModal, { props: { visible: false }, global: { stubs } });
    expect(wrapper.vm.isEmployeeBlacklisted({ last_name: 'Иванов', first_name: 'Иван', is_blacklisted: true })).toBe(true);
    expect(wrapper.vm.isEmployeeBlacklisted({ last_name: 'Сидоров', first_name: 'Сидр', is_blacklisted: false })).toBe(false);
    expect(wrapper.vm.isEmployeeBlacklisted({ last_name: 'Петров', first_name: 'Пётр' })).toBe(false);
  });

  it('isEmployeeDisabled = true для человека из ЧС', () => {
    const wrapper = mount(ExistingEmployeesModal, { props: { visible: false }, global: { stubs } });
    expect(wrapper.vm.isEmployeeDisabled({ id: 1, last_name: 'Иванов', first_name: 'Иван', is_blacklisted: true })).toBe(true);
    expect(wrapper.vm.isEmployeeDisabled({ id: 2, last_name: 'Сидоров', first_name: 'Сидр', is_blacklisted: false })).toBe(false);
  });
});
