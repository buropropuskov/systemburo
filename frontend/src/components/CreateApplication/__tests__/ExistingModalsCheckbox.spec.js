import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })) }));

const stubs = { SearchComponent: true, LoaderSpinner: true };

/**
 * Клик по самому флажку обязан ставить галочку, а не только подсвечивать строку.
 *
 * Ловушка, на которой это сломалось (#2399): обработчик с `preventDefault` отменял
 * нативное переключение, чтобы состояние целиком шло из данных. Браузер переключает
 * флажок ДО обработчика, Vue патчит `:checked` тем же кадром, а откат `preventDefault`
 * происходит после - строка выделена, счётчик растёт, галочки нет.
 *
 * ВАЖНО про природу замка: саму эту гонку jsdom не воспроизводит - на сломанном
 * варианте функциональные проверки ниже остаются зелёными (проверено). Поэтому от
 * возврата дефекта стережёт структурная проверка разметки в конце файла, а поведение
 * подтверждается на стенде. Не заменять её «более честной» функциональной: она молчит.
 */
const ЛЮДИ = [
  { id: 1, last_name: 'Абрамов', first_name: 'Алексей' },
  { id: 2, last_name: 'Богданов', first_name: 'Борис' },
];
const МАШИНЫ = [
  { id: 1, number: 'А001АА777', mark: 'Volvo' },
  { id: 2, number: 'В002ВВ777', mark: 'MAN' },
];

describe.each([
  ['сотрудники', ExistingEmployeesModal, ЛЮДИ, 'displayedEmployees', 'tempSelectedEmployees'],
  ['машины', ExistingCarsModal, МАШИНЫ, 'displayedCars', 'tempSelectedCars'],
])('%s - флажок в окне выбора', (_имя, Компонент, строки, спис, темп) => {
  // Модалка уходит в Teleport, поэтому её строки живут в document, а не в wrapper.
  const открыть = async () => {
    const w = mount(Компонент, { props: { visible: true }, global: { stubs }, attachTo: document.body });
    w.vm[спис] = [...строки];
    await w.vm.$nextTick();
    return w;
  };
  const флажки = () => [...document.querySelectorAll('.table-row input[type="checkbox"]')];
  const клик = async (w, i, доп = {}) => {
    флажки()[i].dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, ...доп }));
    await w.vm.$nextTick();
  };

  it('клик по флажку выбирает запись и ставит галочку', async () => {
    const w = await открыть();
    await клик(w, 0);

    expect(w.vm[темп].map((i) => i.id), 'запись должна попасть в выбор').toEqual([1]);
    expect(флажки()[0].checked, 'галочка обязана стоять, а не только подсветка строки').toBe(true);
    w.unmount();
  });

  it('повторный клик по флажку снимает выбор и галочку', async () => {
    const w = await открыть();
    await клик(w, 0);
    await клик(w, 0);

    expect(w.vm[темп]).toEqual([]);
    expect(флажки()[0].checked).toBe(false);
    w.unmount();
  });

  it('Shift+клик по флажку берёт диапазон, как и по строке', async () => {
    const w = await открыть();
    await клик(w, 0);
    await клик(w, 1, { shiftKey: true });

    expect(w.vm[темп].map((i) => i.id)).toEqual([1, 2]);
    expect(флажки()[1].checked).toBe(true);
    w.unmount();
  });
});

describe.each([
  ['сотрудники', 'ExistingEmployeesModal.vue'],
  ['машины', 'ExistingCarsModal.vue'],
])('%s - разметка флажка', (_имя, файл) => {
  const разметка = () => readFileSync(resolve(__dirname, '..', файл), 'utf8').split('<script')[0];

  it('клик ловит ячейка, а не сам флажок', () => {
    const шаблон = разметка();
    const ячейка = шаблон.match(/<div[^>]*class="table-cell select-cell"[\s\S]{0,400}?<\/div>/);
    expect(ячейка, 'ячейка выбора не найдена').not.toBeNull();
    expect(ячейка[0], 'обработчик должен висеть на ячейке').toMatch(/select-cell"[\s\S]*?@click\.stop="handleRowClick/);
    expect(ячейка[0], 'на самом флажке обработчика быть не должно')
      .not.toMatch(/<input[\s\S]*?@click/);
  });

  it('флажок выключен из событий общим правилом, а не своим стилем', () => {
    // pointer-events живёт в responsive-tables.css рядом с user-select: правило одно
    // на оба окна, копия в каждом разъехалась бы на первой правке.
    expect(
      readFileSync(resolve(__dirname, '../../../assets/responsive-tables.css'), 'utf8'),
    ).toMatch(/\.range-select-row input\[type="checkbox"\]\s*\{[^}]*pointer-events:\s*none/);
  });
});
