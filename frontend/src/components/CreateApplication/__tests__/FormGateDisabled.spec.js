import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import VehicleForm from '../VehicleForm.vue';
import EmployeeForm from '../EmployeeForm.vue';
import ItemsForm from '../ItemsForm.vue';

// п.36: формы добавления (авто/сотрудник/ТМЦ) недоступны, пока не заполнены
// обязательные поля вложения. CreateApplication отдаёт это через prop `disabled`:
// форма получает класс-замок и атрибут inert (барьер взаимодействия). Сам серый
// оверлей с замком и подсказкой живёт уровнем выше - на form__data в CreateApplication.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/blacklist', () => ({
  checkVehicleBlacklist: vi.fn().mockResolvedValue(null),
  checkPersonBlacklist: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/api/marks', () => ({
  listMarks: vi.fn().mockResolvedValue([]),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
vi.mock('@/components/CreateApplication/ExistingCarsModal.vue', () => ({
  default: { name: 'ExistingCarsModal', template: '<div />' },
}));
vi.mock('@/components/CreateApplication/ExistingEmployeesModal.vue', () => ({
  default: { name: 'ExistingEmployeesModal', template: '<div />' },
}));

const FORMS = [
  ['VehicleForm', VehicleForm],
  ['EmployeeForm', EmployeeForm],
  ['ItemsForm', ItemsForm],
];

beforeEach(() => {
  setActivePinia(createPinia());
});

describe('Гейт форм вложения (п.36)', () => {
  it.each(FORMS)('%s: по умолчанию доступна - нет класса-замка и inert', (_name, Form) => {
    const w = mount(Form);
    const root = w.find('.data__completion');
    expect(root.classes()).not.toContain('data__completion--locked');
    expect(root.attributes('inert')).toBeUndefined();
  });

  it.each(FORMS)('%s: disabled=true - класс-замок и inert (барьер взаимодействия)', (_name, Form) => {
    const w = mount(Form, { props: { disabled: true } });
    const root = w.find('.data__completion');
    expect(root.classes()).toContain('data__completion--locked');
    expect(root.attributes('inert')).toBeDefined();
    // Визуальный замок и подсказка перенесены на form__data (CreateApplication) -
    // внутри самой формы их больше нет.
    expect(w.find('.completion__lock').exists()).toBe(false);
  });
});
