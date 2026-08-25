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

describe('EmployeeForm - стрелка поля разрешения', () => {
  const arrowUp = (w) => w.find('.permission__button-arrow').classes('permission__button-arrow--up');

  it('закрытое поле показывает сторону раскрытия', async () => {
    const w = mountForm();

    await w.setData({ permissionMenuUp: false });
    expect(arrowUp(w)).toBe(false);

    await w.setData({ permissionMenuUp: true });
    expect(arrowUp(w)).toBe(true);
  });

  it('открытое поле показывает сторону сворачивания', async () => {
    const w = mountForm();

    // сторону раскрытия задаём после открытия: на открытии её пересчитывает
    // buildPermissionMenuStyle, а в jsdom у элементов нулевые размеры
    await w.setData({ isPermissionDropdownOpen: true });
    await w.vm.$nextTick();

    await w.setData({ permissionMenuUp: true });
    expect(arrowUp(w)).toBe(false);

    await w.setData({ permissionMenuUp: false });
    expect(arrowUp(w)).toBe(true);
  });
});

describe('EmployeeForm - поиск по разрешениям', () => {
  const openMenu = async (w) => {
    await w.find('.permission__dropdown-button').trigger('click');
    return w.find('.permission__search-input');
  };
  const itemTexts = (w) => w.findAll('.permission__dropdown-item').map((i) => i.text());

  it('фильтрует список по подстроке, "Не выбрано" остаётся доступным', async () => {
    const w = mountForm();
    const search = await openMenu(w);

    await search.setValue('студент');

    const texts = itemTexts(w);
    expect(texts[0]).toBe('Не выбрано');
    expect(texts.length).toBeGreaterThan(1);
    expect(texts.slice(1).every((t) => t.toLowerCase().includes('студент'))).toBe(true);
    expect(w.find('.permission__dropdown-empty').exists()).toBe(false);
  });

  it('терпит неверную раскладку клавиатуры', async () => {
    const w = mountForm();
    const search = await openMenu(w);

    // "cnelty" на QWERTY - это "студен" на ЙЦУКЕН
    await search.setValue('cnelty');

    expect(w.vm.filteredPermissions.length).toBeGreaterThan(0);
    expect(w.vm.filteredPermissions.every((p) => p.toLowerCase().includes('студен'))).toBe(true);
  });

  it('без совпадений показывает "Ничего не найдено"', async () => {
    const w = mountForm();
    const search = await openMenu(w);

    await search.setValue('чебурашка');

    expect(w.vm.filteredPermissions).toHaveLength(0);
    expect(w.find('.permission__dropdown-empty').text()).toBe('Ничего не найдено');
    // сброс выбора остаётся под рукой даже с пустой выдачей
    expect(itemTexts(w)).toEqual(['Не выбрано']);
  });

  it('Enter выбирает вариант, когда он остался один', async () => {
    const w = mountForm();
    const search = await openMenu(w);

    await search.setValue('журналист');
    expect(w.vm.filteredPermissions).toHaveLength(1);

    await search.trigger('keydown.enter');
    expect(w.vm.selectedPermission).toBe('Аккредитованные журналисты');
    expect(w.vm.isPermissionDropdownOpen).toBe(false);
  });

  it('закрытие меню сбрасывает запрос', async () => {
    const w = mountForm();
    const search = await openMenu(w);

    await search.setValue('посольств');
    expect(w.vm.permissionQuery).toBe('посольств');

    await w.setData({ isPermissionDropdownOpen: false });
    expect(w.vm.permissionQuery).toBe('');
  });
});
