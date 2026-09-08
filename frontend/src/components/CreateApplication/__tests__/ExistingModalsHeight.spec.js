import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Высоты списков подобраны под конкретное число строк (строка - 42px).
 *
 * Окно «Добавить существующую(-ие)»: сплошной список с прокруткой, видно 15 строк.
 * Пейджер оттуда убран сознательно - он резал Shift-диапазон границей страницы, и
 * чтобы выбрать всех, приходилось повторять жест на каждой странице.
 *
 * Список участников в самой заявке: видно 10 строк, дальше прокрутка. Без потолка он
 * тянулся во всю страницу и выталкивал кнопку отправки далеко вниз.
 */
const стиль = (файл) => readFileSync(
  resolve(__dirname, '..', файл), 'utf8',
).split('<style')[1] || '';

const правило = (текст, селектор) => {
  const m = текст.match(new RegExp(`${селектор}\\s*\\{[^}]*\\}`));
  return m ? m[0] : '';
};

describe.each([
  ['сотрудники', 'ExistingEmployeesModal.vue', 'employees-table-container'],
  ['машины', 'ExistingCarsModal.vue', 'cars-table-container'],
])('окно выбора (%s)', (_имя, файл, контейнер) => {
  it('показывает пятнадцать строк и клампится по высоте экрана', () => {
    const п = правило(стиль(файл), `\\.${контейнер}`);
    expect(п, `правило .${контейнер} не найдено`).not.toBe('');
    const мера = п.match(/max-height:\s*min\((\d+)px/);
    expect(мера, 'высота должна клампиться через min(), иначе на низком экране окно уедет').not.toBeNull();
    expect(Number(мера[1]), 'пятнадцать строк по 42px').toBeGreaterThanOrEqual(600);
  });

  it('пейджера в окне нет', () => {
    const исходник = readFileSync(resolve(__dirname, '..', файл), 'utf8');
    expect(исходник, 'пейджер режет Shift-диапазон границей страницы').not.toContain('<Pager');
    expect(исходник).not.toContain('РАЗМЕР_СТРАНИЦЫ');
  });
});

describe.each([
  ['сотрудники', 'EmployeesList.vue'],
  ['машины', 'VehiclesList.vue'],
])('список участников заявки (%s)', (_имя, файл) => {
  it('высота списка - десять строк, дальше прокрутка', () => {
    const п = правило(стиль(файл), '\\.table-body');
    expect(п, 'правило .table-body не найдено').not.toBe('');
    const мера = п.match(/max-height:\s*(\d+)px/);
    expect(мера, 'без потолка список тянется во всю страницу').not.toBeNull();
    expect(Number(мера[1]), 'десять строк по 42px').toBe(420);
    expect(п, 'переполнение обязано уходить в прокрутку').toMatch(/overflow-y:\s*auto/);
  });
});
