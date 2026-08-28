import { describe, it, expect } from 'vitest';

import TablesComponent from '../TablesComponent.vue';

/**
 * Блок «по факту» бывает только у таблиц машин: строку с номером «по факту» заявитель
 * создаёт тумблером в форме транспорта, а в форме сотрудников такого тумблера нет.
 * У таблицы людей блок выходил пустым и читался охранником как «заявок нет», хотя
 * заявки были (#2019). Человека без заявки вносят ручным добавлением (#1049).
 *
 * Замок держит именно вычисление, а не разметку: флажок в конструкторе у таблиц людей
 * уже скрыт, но у заведённых раньше таблиц show_fact_table мог остаться включённым.
 */
describe('TablesComponent: блок «по факту»', () => {
    const showFactTable = TablesComponent.computed.showFactTable;

    it('показывается у таблицы машин с включённым флажком', () => {
        const vm = { tableData: { table: { table_type: 'cars', show_fact_table: true } } };
        expect(showFactTable.call(vm)).toBe(true);
    });

    it('не показывается у таблицы людей, даже когда флажок включён', () => {
        const vm = { tableData: { table: { table_type: 'people', show_fact_table: true } } };
        expect(showFactTable.call(vm)).toBe(false);
    });

    it('не показывается у таблицы машин с выключенным флажком', () => {
        const vm = { tableData: { table: { table_type: 'cars', show_fact_table: false } } };
        expect(showFactTable.call(vm)).toBe(false);
    });
});
