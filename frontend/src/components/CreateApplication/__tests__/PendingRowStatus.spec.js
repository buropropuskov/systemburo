import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import VehiclesList from '../VehiclesList.vue';
import EmployeesList from '../EmployeesList.vue';

// Строка, разобранная из бланка, но ещё не добавленная в заявку, называет своё состояние
// словами: приглушённого цвета владельцу было мало («серые строки надо сделать ещё серее,
// либо статус явно отображать»). Обе раскладки - колоночная и карточная - равноправны.

const VEHICLES = [
    { id: 1, plateNumber: 'A777AA777', mark: 'BMW' },
    { id: 2, plateNumber: 'В002ВВ777', mark: 'Kia', isPending: true },
];

const EMPLOYEES = [
    { id: 1, lastName: 'Иванов', firstName: 'Иван' },
    { id: 2, lastName: 'Петров', firstName: 'Пётр', isPending: true },
];

function mountList(component, props) {
    return mount(component, {
        props,
        global: { stubs: { VehicleDetailsModal: true, EmployeeDetailsModal: true, ConfirmationModal: true } },
    });
}

describe('Статус предварительной строки списка', () => {
    it('машины: колоночная раскладка показывает бейдж «В очереди» только у строки из бланка', () => {
        const w = mountList(VehiclesList, { vehicles: VEHICLES });

        const badges = w.findAll('.pending-badge');
        expect(badges).toHaveLength(1);
        expect(badges[0].text()).toBe('В очереди');
        // Бейдж стоит в ячейке номера предварительной машины, а не у соседней.
        expect(w.findAll('.vcol__cell--plate')[1].text()).toContain('В очереди');
        expect(w.findAll('.vcol__cell--plate')[0].text()).not.toContain('В очереди');
    });

    it('машины: карточная раскладка показывает тот же бейдж', async () => {
        const w = mountList(VehiclesList, { vehicles: VEHICLES });
        w.vm.isNarrow = true;
        await w.vm.$nextTick();

        const rows = w.findAll('[data-testid="vehicles-row"]');
        expect(rows[0].text()).not.toContain('В очереди');
        expect(rows[1].text()).toContain('В очереди');
        expect(rows[1].classes()).toContain('is-pending');
    });

    it('сотрудники: бейдж стоит у строки из бланка', () => {
        const w = mountList(EmployeesList, { employees: EMPLOYEES });

        const badges = w.findAll('.pending-badge');
        expect(badges).toHaveLength(1);
        expect(badges[0].text()).toBe('В очереди');

        const rows = w.findAll('[data-testid="employees-row"]');
        expect(rows[0].text()).not.toContain('В очереди');
        expect(rows[1].text()).toContain('В очереди');
    });

    it('список без строк из бланка бейджей не рисует', () => {
        const vehicles = mountList(VehiclesList, { vehicles: [VEHICLES[0]] });
        expect(vehicles.findAll('.pending-badge')).toHaveLength(0);

        const employees = mountList(EmployeesList, { employees: [EMPLOYEES[0]] });
        expect(employees.findAll('.pending-badge')).toHaveLength(0);
    });
});
