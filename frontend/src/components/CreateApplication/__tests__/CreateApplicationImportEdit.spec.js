import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import BlankImportPanel from '../BlankImportPanel.vue';

// Правка строки не должна убивать разбор бланка: форма правки открывается на месте
// панели, а сводка возвращается, когда правка завершена (сохранена или отменена).

const notifyMock = vi.fn();
vi.mock('@/stores/deletions', () => ({
    useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })),
}));

vi.mock('@/api/client', () => ({
    apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
    createExtendedTimeoutSignal: vi.fn(() => 'FAKE_SIGNAL'),
}));

vi.mock('@/stores/auth', () => ({
    useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));

vi.mock('@/stores/permissions', () => ({
    usePermissionsStore: () => ({ hasPermission: (key) => key === 'action.import.list' }),
}));

vi.mock('@/api/blankImport', () => ({
    downloadBlankTemplate: vi.fn(),
    uploadImportList: vi.fn(),
}));

// Формы подменяем заглушками С МЕТОДАМИ правки: родитель зовёт их через $refs, а
// shallowMount-заглушка без методов уронила бы вызов раньше проверяемого поведения.
const VehicleFormStub = {
    name: 'VehicleForm',
    template: '<div class="vehicle-form-stub" />',
    methods: { editVehicle: vi.fn() },
};
const EmployeeFormStub = {
    name: 'EmployeeForm',
    template: '<div class="employee-form-stub" />',
    methods: { editEmployee: vi.fn() },
};

const IMPORT_RESULT = {
    summary: { read: 2, accepted: 1, rejected: 1 },
    rows: [{ row_number: 5, vehicle: { car_number: 'А001АА777', car_brand: 'Volvo' }, errors: [], warnings: [] }],
};

async function mountApp() {
    const w = shallowMount(CreateApplication, {
        global: { stubs: { VehicleForm: VehicleFormStub, EmployeeForm: EmployeeFormStub } },
    });
    await flushPromises();
    return w;
}

function withCars(w, vehicles) {
    w.vm.attachments = [{ local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' }];
    w.vm.vehiclesByAttachment = { c1: vehicles };
    w.vm.selectedAttachment = w.vm.attachments[0];
    w.vm.importMode = true;
    w.vm.importResult = IMPORT_RESULT;
}

function withPeople(w, employees) {
    w.vm.attachments = [{ local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' }];
    w.vm.employeesByAttachment = { p1: employees };
    w.vm.selectedAttachment = w.vm.attachments[0];
    w.vm.importMode = true;
    w.vm.importResult = IMPORT_RESULT;
}

describe('Правка строки при открытом импорте', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        localStorage.clear();
        notifyMock.mockReset();
    });

    it('правка машины показывает форму, не теряя разбор, и сводка возвращается после сохранения', async () => {
        const w = await mountApp();
        const vehicle = { id: 1, plateNumber: 'А001АА777', mark: 'Volvo', isPending: true };
        withCars(w, [vehicle]);
        await flushPromises();

        w.vm.editVehicle(vehicle);
        await flushPromises();

        // Разбор жив: сводка никуда не делась, только уступила место форме.
        expect(w.vm.importResult).not.toBeNull();
        const panel = w.findComponent(BlankImportPanel);
        expect(panel.exists()).toBe(true);
        expect(panel.element.style.display).toBe('none');
        expect(w.find('.vehicle-form-stub').exists()).toBe(true);
        expect(notifyMock).not.toHaveBeenCalled();

        w.vm.handleVehicleUpdated({ ...vehicle, mark: 'Scania' });
        await flushPromises();

        expect(w.findComponent(BlankImportPanel).element.style.display).toBe('');
        expect(w.find('.vehicle-form-stub').exists()).toBe(false);
        expect(w.vm.importResult).not.toBeNull();
    });

    it('отмена правки машины тоже возвращает сводку', async () => {
        const w = await mountApp();
        const vehicle = { id: 1, plateNumber: 'А001АА777', mark: 'Volvo', isPending: true };
        withCars(w, [vehicle]);
        await flushPromises();

        w.vm.editVehicle(vehicle);
        await flushPromises();
        expect(w.findComponent(BlankImportPanel).element.style.display).toBe('none');

        w.vm.handleVehicleEditCancelled();
        await flushPromises();

        expect(w.findComponent(BlankImportPanel).element.style.display).toBe('');
        expect(w.vm.importResult).not.toBeNull();
    });

    it('правка сотрудника ведёт себя так же', async () => {
        const w = await mountApp();
        const employee = { id: 1, lastName: 'Иванов', firstName: 'Иван', isPending: true };
        withPeople(w, [employee]);
        await flushPromises();

        w.vm.editEmployee(employee);
        await flushPromises();

        expect(w.vm.importResult).not.toBeNull();
        expect(w.find('.employee-form-stub').exists()).toBe(true);
        expect(w.findComponent(BlankImportPanel).element.style.display).toBe('none');

        w.vm.handleEmployeeUpdated({ ...employee, lastName: 'Петров' });
        await flushPromises();

        expect(w.findComponent(BlankImportPanel).element.style.display).toBe('');
    });

    // Выход из режима - по-прежнему выход: сводка закрывается, форма возвращается.
    it('закрытие режима из панели убирает и сводку, и приостановку', async () => {
        const w = await mountApp();
        const vehicle = { id: 1, plateNumber: 'А001АА777', mark: 'Volvo', isPending: true };
        withCars(w, [vehicle]);
        await flushPromises();

        w.vm.editVehicle(vehicle);
        await flushPromises();
        w.vm.closeImportMode();
        await flushPromises();

        expect(w.vm.importMode).toBe(false);
        expect(w.vm.importResult).toBeNull();
        expect(w.findComponent(BlankImportPanel).exists()).toBe(false);
        expect(w.find('.vehicle-form-stub').exists()).toBe(true);
    });
});
