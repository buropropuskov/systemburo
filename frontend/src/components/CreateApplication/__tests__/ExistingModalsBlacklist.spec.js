import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';

// Модалки на visible=false не дёргают API (watcher не срабатывает) - тестируем чистую
// логику матчинга ЧС, выставляя blacklistKeys напрямую. Зеркалирование серверной
// нормализации (LOWER+TRIM) - ключевое, что проверяем.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/blacklist', () => ({
  listVehicleBlacklist: vi.fn().mockResolvedValue([]),
  listPersonBlacklist: vi.fn().mockResolvedValue([]),
}));

const stubs = { SearchComponent: true, LoaderSpinner: true };

describe('ExistingCarsModal - гард ЧС', () => {
  it('blacklistKey нормализует номер и марку (LOWER+TRIM)', () => {
    const wrapper = mount(ExistingCarsModal, { props: { visible: false }, global: { stubs } });
    expect(wrapper.vm.blacklistKey('  A777AA  ', '  BMW ')).toBe('a777aa|bmw');
  });

  it('isCarBlacklisted матчит по номеру+марке без учёта регистра/пробелов по краям', () => {
    const wrapper = mount(ExistingCarsModal, { props: { visible: false }, global: { stubs } });
    wrapper.vm.blacklistKeys = new Set(['a777aa777|bmw']);
    expect(wrapper.vm.isCarBlacklisted({ number: 'A777AA777', mark: 'BMW' })).toBe(true);
    expect(wrapper.vm.isCarBlacklisted({ number: ' a777aa777 ', mark: ' bmw ' })).toBe(true);
    expect(wrapper.vm.isCarBlacklisted({ number: 'A777AA777', mark: 'Toyota' })).toBe(false);
    expect(wrapper.vm.isCarBlacklisted({ number: 'B111BB111', mark: 'BMW' })).toBe(false);
  });

  it('isCarDisabled = true для машины из ЧС даже без already-added', () => {
    const wrapper = mount(ExistingCarsModal, { props: { visible: false }, global: { stubs } });
    wrapper.vm.blacklistKeys = new Set(['a777aa777|bmw']);
    expect(wrapper.vm.isCarDisabled({ id: 1, number: 'A777AA777', mark: 'BMW' })).toBe(true);
    expect(wrapper.vm.isCarDisabled({ id: 2, number: 'C999CC999', mark: 'Toyota' })).toBe(false);
  });
});

describe('ExistingEmployeesModal - гард ЧС', () => {
  it('blacklistKey по ФИО, пустое отчество нормализуется', () => {
    const wrapper = mount(ExistingEmployeesModal, { props: { visible: false }, global: { stubs } });
    expect(wrapper.vm.blacklistKey(' Иванов ', 'Иван', '')).toBe('иванов|иван|');
    expect(wrapper.vm.blacklistKey('Иванов', 'Иван', 'Иванович')).toBe('иванов|иван|иванович');
  });

  it('isEmployeeBlacklisted матчит по ФИО, пустое отчество матчит пустое', () => {
    const wrapper = mount(ExistingEmployeesModal, { props: { visible: false }, global: { stubs } });
    wrapper.vm.blacklistKeys = new Set(['иванов|иван|']);
    expect(wrapper.vm.isEmployeeBlacklisted({ last_name: 'Иванов', first_name: 'Иван', middle_name: '' })).toBe(true);
    expect(wrapper.vm.isEmployeeBlacklisted({ last_name: 'Иванов', first_name: 'Иван', middle_name: null })).toBe(true);
    expect(wrapper.vm.isEmployeeBlacklisted({ last_name: 'Иванов', first_name: 'Иван', middle_name: 'Петрович' })).toBe(false);
  });

  it('isEmployeeDisabled = true для человека из ЧС', () => {
    const wrapper = mount(ExistingEmployeesModal, { props: { visible: false }, global: { stubs } });
    wrapper.vm.blacklistKeys = new Set(['иванов|иван|иванович']);
    expect(wrapper.vm.isEmployeeDisabled({ id: 1, last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович' })).toBe(true);
    expect(wrapper.vm.isEmployeeDisabled({ id: 2, last_name: 'Сидоров', first_name: 'Сидр', middle_name: '' })).toBe(false);
  });
});
