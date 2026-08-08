import { describe, it, expect } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import EmployeesList from '../EmployeesList.vue';

// blank-import E1: список должен пережить импорт бланком (до 2000 строк) без рендера
// всего массива v-for'ом. Проверяем на реальном наполнении 2000 строк, не на пустом
// состоянии - иначе тест не ловит регресс к "рендерим всё" (component-рендерится сам
// по себе ничего не доказывает).

function makeEmployees(n) {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    lastName: `Фамилия${i + 1}`,
    firstName: `Имя${i + 1}`,
    middleName: `Отчество${i + 1}`,
  }));
}

describe('EmployeesList - постраничный показ на большом списке (blank-import E1)', () => {
  it('2000 строк: в DOM попадает страница, а не весь массив', () => {
    const employees = makeEmployees(2000);
    const w = shallowMount(EmployeesList, { props: { employees } });

    const rows = w.findAll('[data-testid="employees-row"]');
    // PAGE_SIZE в компоненте - 50: страница держит фиксированный размер вне
    // зависимости от объёма исходного массива.
    expect(rows.length).toBe(50);

    const totalNodes = w.element.querySelectorAll('*').length;
    expect(totalNodes).toBeLessThan(employees.length);
  });

  it('переход в конец списка показывает последнюю строку, окно рендера не растёт', async () => {
    const employees = makeEmployees(2000);
    const w = shallowMount(EmployeesList, { props: { employees } });

    await w.get('[data-testid="employees-last-page"]').trigger('click');

    const rows = w.findAll('[data-testid="employees-row"]');
    expect(rows.length).toBe(50);
    expect(rows[rows.length - 1].text()).toContain('Фамилия2000');
    expect(w.get('[data-testid="employees-page-info"]').text()).toContain('40 из 40');
  });

  it('удаление строки на большом списке эмитит delete-employee с корректным id', async () => {
    const employees = makeEmployees(2000);
    const w = shallowMount(EmployeesList, { props: { employees } });

    const firstRow = w.findAll('[data-testid="employees-row"]')[0];
    await firstRow.find('.delete-btn').trigger('click');

    expect(w.emitted('delete-employee')).toEqual([[1]]);
  });

  it('поиск фильтрует строки и сбрасывает страницу на первую', async () => {
    const employees = makeEmployees(2000);
    const w = shallowMount(EmployeesList, { props: { employees } });

    await w.get('[data-testid="employees-last-page"]').trigger('click');
    await w.get('[data-testid="employees-search"]').setValue('Фамилия1999');

    const rows = w.findAll('[data-testid="employees-row"]');
    expect(rows.length).toBe(1);
    expect(rows[0].text()).toContain('Фамилия1999');
  });

  it('небольшой список (как раньше) не показывает поиск и пагинацию', () => {
    const employees = makeEmployees(3);
    const w = shallowMount(EmployeesList, { props: { employees } });

    expect(w.find('[data-testid="employees-search"]').exists()).toBe(false);
    expect(w.findAll('[data-testid="employees-row"]').length).toBe(3);
  });

  it('пустой список сохраняет исходное пустое состояние', () => {
    const w = shallowMount(EmployeesList, { props: { employees: [] } });

    expect(w.find('.no-employees').text()).toContain('Нет добавленных сотрудников');
    expect(w.find('[data-testid="employees-search"]').exists()).toBe(false);
  });
});
