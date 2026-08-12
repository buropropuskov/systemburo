import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import VehiclesList from '../VehiclesList.vue';
import EmployeesList from '../EmployeesList.vue';

// Пустое состояние - часть таблицы: шапка колонок остаётся, сообщение стоит в её теле
// строкой на всю ширину. Раньше оно висело отдельным блоком ПОД таблицей и читалось как
// чужой текст рядом со списком.

function mountList(component, props) {
    return mount(component, {
        props,
        global: { stubs: { VehicleDetailsModal: true, EmployeeDetailsModal: true, ConfirmationModal: true } },
    });
}

// Поиск включается только когда список длиннее страницы (useListSearchPagination,
// 50 строк), поэтому «ничего не найдено» проверяем на таком наполнении.
function manyVehicles(count) {
    return Array.from({ length: count }, (_, i) => ({
        id: i + 1, plateNumber: `А${String(i + 1).padStart(3, '0')}АА777`, mark: 'Volvo',
    }));
}

function manyEmployees(count) {
    return Array.from({ length: count }, (_, i) => ({
        id: i + 1, lastName: `Фамилия${i + 1}`, firstName: 'Иван',
    }));
}

describe('Пустое состояние живёт внутри таблицы', () => {
    it('машины: сообщение внутри таблицы, шапка колонок на месте', () => {
        const w = mountList(VehiclesList, { vehicles: [] });

        const table = w.find('.vehicles-cols');
        expect(table.exists()).toBe(true);
        expect(table.findAll('.vcol__head').length).toBeGreaterThan(0);

        const empty = w.find('[data-testid="vehicles-empty"]');
        expect(empty.exists()).toBe(true);
        expect(empty.text()).toBe('Нет добавленных транспортных средств');
        // Именно внутри таблицы, а не соседним блоком под ней.
        expect(table.find('[data-testid="vehicles-empty"]').exists()).toBe(true);
    });

    it('машины: карточная ветка держит сообщение в теле таблицы', async () => {
        const w = mountList(VehiclesList, { vehicles: [] });
        w.vm.isNarrow = true;
        await w.vm.$nextTick();

        const body = w.find('.vehicles-table .table-body');
        expect(body.exists()).toBe(true);
        expect(body.find('[data-testid="vehicles-empty"]').exists()).toBe(true);
        expect(w.find('.vehicles-table .rt-head-row').exists()).toBe(true);
    });

    it('машины: поиск без совпадений объясняет причину там же', async () => {
        const w = mountList(VehiclesList, { vehicles: manyVehicles(60) });

        await w.find('[data-testid="vehicles-search"]').setValue('такого номера нет');

        const empty = w.find('.vehicles-cols [data-testid="vehicles-empty"]');
        expect(empty.exists()).toBe(true);
        expect(empty.text()).toContain('Ничего не найдено по запросу');
    });

    it('сотрудники: сообщение внутри тела таблицы, шапка колонок на месте', () => {
        const w = mountList(EmployeesList, { employees: [] });

        const body = w.find('.employees-table .table-body');
        expect(body.exists()).toBe(true);
        expect(body.find('[data-testid="employees-empty"]').exists()).toBe(true);
        expect(body.find('[data-testid="employees-empty"]').text()).toBe('Нет добавленных сотрудников');
        expect(w.find('.employees-table .rt-head-row').exists()).toBe(true);
    });

    it('сотрудники: поиск без совпадений объясняет причину там же', async () => {
        const w = mountList(EmployeesList, { employees: manyEmployees(60) });

        await w.find('[data-testid="employees-search"]').setValue('никого такого');

        const empty = w.find('.employees-table .table-body [data-testid="employees-empty"]');
        expect(empty.exists()).toBe(true);
        expect(empty.text()).toContain('Ничего не найдено по запросу');
    });

    it('строка из бланка тоже считается наполнением - пустого сообщения нет', () => {
        const vehicles = mountList(VehiclesList, {
            vehicles: [{ id: 1, plateNumber: 'В002ВВ777', mark: 'Kia', isPending: true }],
        });
        expect(vehicles.find('[data-testid="vehicles-empty"]').exists()).toBe(false);

        const employees = mountList(EmployeesList, {
            employees: [{ id: 1, lastName: 'Петров', firstName: 'Пётр', isPending: true }],
        });
        expect(employees.find('[data-testid="employees-empty"]').exists()).toBe(false);
    });
});

// Кнопка «Очистить» жила в отдельной строке-тулбаре под шапкой (.list-toolbar) -
// на коротком списке (нет поиска/пейджера, showToolbar=false) эта строка всё равно
// рисовалась ради одной кнопки и давала пустой пробел между шапкой и таблицей
// (претензия владельца). Теперь очистка живёт в самой строке шапки, а строка
// поиска/пейджера рисуется только когда в ней реально есть что показывать.
describe('«Очистить» стоит в шапке списка - тулбар не создаёт пустую строку', () => {
    it('машины: кнопка в строке шапки, отдельного тулбара под коротким списком нет', () => {
        const w = mountList(VehiclesList, {
            vehicles: [{ id: 1, plateNumber: 'А001АА777', mark: 'Volvo' }],
            canImport: true,
        });

        expect(w.find('.header-with-badge [data-testid="vehicles-clear-btn"]').exists()).toBe(true);
        expect(w.find('.list-toolbar').exists()).toBe(false);
        // Бейдж Experimental остаётся рядом с кнопкой импорта - он про импорт.
        expect(w.find('.header-with-badge .import-entry .badge').text()).toBe('Experimental');
    });

    it('сотрудники: то же расположение', () => {
        const w = mountList(EmployeesList, {
            employees: [{ id: 1, lastName: 'Иванов', firstName: 'Иван' }],
            canImport: true,
        });

        expect(w.find('.header-with-badge [data-testid="employees-clear-btn"]').exists()).toBe(true);
        expect(w.find('.list-toolbar').exists()).toBe(false);
        expect(w.find('.header-with-badge .import-entry .badge').text()).toBe('Experimental');
    });

    it('машины: без права на импорт кнопка всё равно остаётся в шапке', () => {
        const w = mountList(VehiclesList, {
            vehicles: [{ id: 1, plateNumber: 'А001АА777', mark: 'Volvo' }],
        });

        expect(w.find('.header-with-badge [data-testid="vehicles-clear-btn"]').exists()).toBe(true);
        expect(w.find('.list-toolbar').exists()).toBe(false);
    });

    it('машины: длинный список рисует тулбар поиска отдельно, очистка остаётся в шапке', () => {
        const w = mountList(VehiclesList, { vehicles: manyVehicles(60) });

        expect(w.find('.list-toolbar [data-testid="vehicles-search"]').exists()).toBe(true);
        expect(w.find('.list-toolbar [data-testid="vehicles-clear-btn"]').exists()).toBe(false);
        expect(w.find('.header-with-badge [data-testid="vehicles-clear-btn"]').exists()).toBe(true);
    });

    it('пустой список: ни очистки, ни тулбара, ни лишней строки в шапке', () => {
        const w = mountList(VehiclesList, { vehicles: [] });

        expect(w.find('[data-testid="vehicles-clear-btn"]').exists()).toBe(false);
        expect(w.find('.list-toolbar').exists()).toBe(false);
        expect(w.find('.header-actions').exists()).toBe(false);
    });
});
