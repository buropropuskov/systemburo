import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import EmployeeForm from '../EmployeeForm.vue';

// Иное разрешение на работы: список длинный, выбранное значение блокирует поле патента.
// Без пункта "Не выбрано" выбор был необратим - патент оставался заблокированным навсегда.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
}));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue(null)
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' })
}));
vi.mock('@/components/CreateApplication/ExistingEmployeesModal.vue', () => ({
  default: { name: 'ExistingEmployeesModal', template: '<div />' }
}));

beforeEach(() => {
  setActivePinia(createPinia());
});

// patent.required=true форсирует обязательность патента независимо от гражданства,
// иначе блок разрешения отрисуется в неактивном состоянии.
const mountForm = () => mount(EmployeeForm, {
  props: {
    fieldConfig: {
      patent: { visible: true, required: true },
      work_permission: { visible: true, required: false }
    }
  }
});

const patentInput = (w) => w.find('.completion__patent input');

describe('EmployeeForm - иное разрешение на работы', () => {
  it('первым пунктом идёт "Не выбрано", дальше сами разрешения', async () => {
    const w = mountForm();
    await w.find('.permission__dropdown-button').trigger('click');

    const items = w.findAll('.permission__dropdown-item');
    expect(items).toHaveLength(w.vm.availablePermissions.length + 1);
    expect(items[0].text()).toBe('Не выбрано');
  });

  it('выбор разрешения блокирует номер патента, "Не выбрано" возвращает ввод', async () => {
    const w = mountForm();

    await w.find('.permission__dropdown-button').trigger('click');
    await w.findAll('.permission__dropdown-item')[1].trigger('click');
    expect(w.vm.selectedPermission).toBe(w.vm.availablePermissions[0]);
    expect(patentInput(w).attributes('disabled')).toBeDefined();

    await w.find('.permission__dropdown-button').trigger('click');
    await w.findAll('.permission__dropdown-item')[0].trigger('click');
    expect(w.vm.selectedPermission).toBe('');
    expect(patentInput(w).attributes('disabled')).toBeUndefined();

    await patentInput(w).setValue('77АА123456');
    expect(w.vm.patentNumber).toBe('77АА123456');
  });

  it('введённый патент закрывает выбор разрешения', async () => {
    const w = mountForm();

    await patentInput(w).setValue('77АА123456');
    expect(w.find('.permission__dropdown-button').attributes('disabled')).toBeDefined();

    await patentInput(w).setValue('');
    expect(w.find('.permission__dropdown-button').attributes('disabled')).toBeUndefined();
  });
});
