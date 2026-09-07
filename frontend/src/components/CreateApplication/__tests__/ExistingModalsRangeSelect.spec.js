import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

const stubs = { SearchComponent: true, LoaderSpinner: true };

/**
 * Выбор диапазона с Shift в окнах «Добавить существующую(-ие)» (#2399). До этого
 * набрать три десятка человек можно было только тридцатью кликами.
 */
const ЛЮДИ = [
  { id: 1, last_name: 'Абрамов', first_name: 'Алексей' },
  { id: 2, last_name: 'Богданов', first_name: 'Борис' },
  { id: 3, last_name: 'Веселов', first_name: 'Виктор' },
  { id: 4, last_name: 'Гаврилов', first_name: 'Геннадий' },
];
const МАШИНЫ = [
  { id: 1, number: 'А001АА777', mark: 'Volvo' },
  { id: 2, number: 'В002ВВ777', mark: 'MAN' },
  { id: 3, number: 'С003СС777', mark: 'Scania' },
  { id: 4, number: 'Е004ЕЕ777', mark: 'Ford' },
];

const shift = { shiftKey: true };

describe.each([
  ['сотрудники', ExistingEmployeesModal, ЛЮДИ, 'displayedEmployees', 'tempSelectedEmployees'],
  ['машины', ExistingCarsModal, МАШИНЫ, 'displayedCars', 'tempSelectedCars'],
])('%s - выбор диапазона с Shift (#2399)', (_имя, Компонент, строки, спис, темп) => {
  const смонтировать = () => {
    const w = mount(Компонент, { props: { visible: false }, global: { stubs } });
    w.vm[спис] = [...строки];
    return w;
  };
  const выбранные = (w) => w.vm[темп].map((i) => i.id).sort();

  it('обычный клик выбирает одну строку и ставит якорь', () => {
    const w = смонтировать();
    w.vm.handleRowClick(строки[1], {});
    expect(выбранные(w)).toEqual([2]);
    expect(w.vm.selectionAnchorId).toBe(2);
    w.unmount();
  });

  it('Shift+клик берёт всё от якоря до цели', () => {
    const w = смонтировать();
    w.vm.handleRowClick(строки[0], {});
    w.vm.handleRowClick(строки[2], shift);
    expect(выбранные(w)).toEqual([1, 2, 3]);
    w.unmount();
  });

  it('повторный Shift переопределяет диапазон, а не копит его', () => {
    // Промах на пару строк иначе было бы не отменить: лишние остались бы выбранными.
    const w = смонтировать();
    w.vm.handleRowClick(строки[0], {});
    w.vm.handleRowClick(строки[3], shift);
    expect(выбранные(w)).toEqual([1, 2, 3, 4]);
    w.vm.handleRowClick(строки[1], shift);
    expect(выбранные(w), 'диапазон сузился до якоря и новой цели').toEqual([1, 2]);
    w.unmount();
  });

  it('Shift без якоря ведёт себя как обычный клик', () => {
    const w = смонтировать();
    w.vm.handleRowClick(строки[2], shift);
    expect(выбранные(w)).toEqual([3]);
    w.unmount();
  });

  it('диапазон считается по видимому списку, а не по исходному набору', () => {
    // Поиск сузил выдачу: между якорем и целью на экране лежат другие записи.
    const w = смонтировать();
    w.vm[спис] = [строки[3], строки[0], строки[2]];
    w.vm.handleRowClick(строки[3], {});
    w.vm.handleRowClick(строки[2], shift);
    expect(выбранные(w)).toEqual([1, 3, 4]);
    w.unmount();
  });

  it('выбор, сделанный до якоря, диапазон не теряет', () => {
    const w = смонтировать();
    w.vm.handleRowClick(строки[3], {});
    w.vm.handleRowClick(строки[0], {});
    w.vm.handleRowClick(строки[1], shift);
    expect(выбранные(w)).toEqual([1, 2, 4]);
    w.unmount();
  });

  it('строки списка не выделяются текстом при Shift', () => {
    // Браузер тем же жестом тянет выделение от прошлого клика - список синеет
    // поверх выбора. Правило лежит в общем responsive-tables.css, класс - на строке.
    const файл = Компонент.__file || '';
    const разметка = readFileSync(resolve(__dirname, '..', файл.split('/').pop()), 'utf8').split('<script')[0];
    expect(разметка, 'на строке нет класса, гасящего выделение').toContain('range-select-row');
    expect(
      readFileSync(resolve(__dirname, '../../../assets/responsive-tables.css'), 'utf8'),
      'правило должно жить в общем CSS, а не в двух копиях',
    ).toMatch(/\.range-select-row\s*\{[^}]*user-select:\s*none/);
  });
});
