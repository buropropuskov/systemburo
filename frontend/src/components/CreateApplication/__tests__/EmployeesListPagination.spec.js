import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import EmployeesList from '../EmployeesList.vue';

// blank-import E1: список должен пережить импорт бланком (до 2000 строк) без рендера
// всего массива v-for'ом. Проверяем на реальном наполнении 2000 строк, не на пустом
// состоянии - иначе тест не ловит регресс к "рендерим всё" (component-рендерится сам
// по себе ничего не доказывает). mount (не shallowMount) - пагинация теперь живёт
// внутри реального Pager, клик по data-testid у стаба ничего бы не сделал.

function makeEmployees(n) {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    lastName: `Фамилия${i + 1}`,
    firstName: `Имя${i + 1}`,
    middleName: `Отчество${i + 1}`,
  }));
}

// В Pager есть только Назад/Вперёд (без прыжка в начало/конец) - .pager__btn[1].
function clickNext(w) {
  return w.findAll('.pager__btn')[1].trigger('click');
}

describe('EmployeesList - постраничный показ на большом списке (blank-import E1)', () => {
  it('2000 строк: в DOM попадает страница, а не весь массив', () => {
    const employees = makeEmployees(2000);
    const w = mount(EmployeesList, { props: { employees } });

    const rows = w.findAll('[data-testid="employees-row"]');
    // Размер страницы в composable - 50: страница держит фиксированный размер вне
    // зависимости от объёма исходного массива.
    expect(rows.length).toBe(50);

    const totalNodes = w.element.querySelectorAll('*').length;
    expect(totalNodes).toBeLessThan(employees.length);
  });

  it('переход на следующую страницу показывает следующий блок строк', async () => {
    const employees = makeEmployees(2000);
    const w = mount(EmployeesList, { props: { employees } });

    await clickNext(w);

    const rows = w.findAll('[data-testid="employees-row"]');
    expect(rows.length).toBe(50);
    expect(rows[0].text()).toContain('Фамилия51');
    expect(w.get('.pager__page').text()).toBe('Стр. 2 / 40');
  });

  it('удаление строки на большом списке эмитит delete-employee с корректным id', async () => {
    const employees = makeEmployees(2000);
    const w = mount(EmployeesList, { props: { employees } });

    const firstRow = w.findAll('[data-testid="employees-row"]')[0];
    await firstRow.find('.delete-btn').trigger('click');

    expect(w.emitted('delete-employee')).toEqual([[1]]);
  });

  it('поиск фильтрует строки и сбрасывает страницу на первую', async () => {
    const employees = makeEmployees(2000);
    const w = mount(EmployeesList, { props: { employees } });

    await clickNext(w);
    await w.get('[data-testid="employees-search"]').setValue('Фамилия1999');

    const rows = w.findAll('[data-testid="employees-row"]');
    expect(rows.length).toBe(1);
    expect(rows[0].text()).toContain('Фамилия1999');
  });

  it('небольшой список (как раньше) не показывает поиск и пагинацию', () => {
    const employees = makeEmployees(3);
    const w = mount(EmployeesList, { props: { employees } });

    expect(w.find('[data-testid="employees-search"]').exists()).toBe(false);
    expect(w.findAll('[data-testid="employees-row"]').length).toBe(3);
  });

  it('пустой список сохраняет исходное пустое состояние', () => {
    const w = mount(EmployeesList, { props: { employees: [] } });

    expect(w.find('[data-testid="employees-empty"]').text()).toContain('Нет добавленных сотрудников');
    expect(w.find('[data-testid="employees-search"]').exists()).toBe(false);
  });

  // Регресс, ради которого список переводили на composable: активный поиск сузил
  // видимую часть, а список ЦЕЛИКОМ упал ниже порога тулбара (50) - например,
  // после импорта бланком лишние строки удалили. Тулбар прячется, и вместе с ним
  // обязан сброситься поиск - иначе фильтр молча остаётся действующим, а инпут,
  // которым его можно снять, уже скрыт.
  it('список уменьшился ниже порога тулбара во время активного поиска - сбрасывает поиск, показывает все строки', async () => {
    const employees = makeEmployees(2000);
    const w = mount(EmployeesList, { props: { employees } });

    await w.get('[data-testid="employees-search"]').setValue('Фамилия1999');
    expect(w.findAll('[data-testid="employees-row"]').length).toBe(1);

    await w.setProps({ employees: makeEmployees(10) });

    expect(w.find('[data-testid="employees-search"]').exists()).toBe(false);
    expect(w.findAll('[data-testid="employees-row"]').length).toBe(10);
  });
});
