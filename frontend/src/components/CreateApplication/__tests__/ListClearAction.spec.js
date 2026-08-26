import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import BlankImportPanel from '../BlankImportPanel.vue';
import VehiclesList from '../VehiclesList.vue';
import EmployeesList from '../EmployeesList.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';

// Эпик blank-import-ux, срез U6: «Очистить» в шапке списка убирает ВЕСЬ список текущего
// вложения - и заведённое руками, и предварительные строки из бланка. Отмены нет, поэтому
// кнопка сначала спрашивает подтверждение общей ConfirmationModal, а не чистит по клику.

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

let hasImportPermission = true;
vi.mock('@/stores/permissions', () => ({
    usePermissionsStore: () => ({
        hasPermission: (key) => hasImportPermission && key === 'action.import.list',
    }),
}));

vi.mock('@/api/blankImport', () => ({
    downloadBlankTemplate: vi.fn(),
    uploadImportList: vi.fn(),
}));

const VEHICLE = { id: 1, plateNumber: 'А001АА777', mark: 'Volvo' };
const PENDING_VEHICLE = { id: 2, plateNumber: 'В002ВВ777', mark: 'Scania', isPending: true };
const EMPLOYEE = { id: 1, lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович' };
const PENDING_EMPLOYEE = { id: 2, lastName: 'Петров', firstName: 'Пётр', isPending: true };

beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    notifyMock.mockReset();
    hasImportPermission = true;
});

afterEach(() => {
    hasImportPermission = false;
});

function mountVehicles(vehicles) {
    return shallowMount(VehiclesList, { props: { vehicles } });
}

function mountEmployees(employees) {
    return shallowMount(EmployeesList, { props: { employees } });
}

describe.each([
    ['список машин', mountVehicles, 'vehicles-clear-btn', [VEHICLE], [VEHICLE, PENDING_VEHICLE]],
    ['список сотрудников', mountEmployees, 'employees-clear-btn', [EMPLOYEE], [EMPLOYEE, PENDING_EMPLOYEE]],
])('%s - кнопка «Очистить» (U6)', (_name, mountList, testid, oneRow, withPending) => {
    it('на пустом списке кнопки нет, со строками - появляется', () => {
        expect(mountList([]).find(`[data-testid="${testid}"]`).exists()).toBe(false);
        expect(mountList(oneRow).find(`[data-testid="${testid}"]`).exists()).toBe(true);
    });

    it('клик открывает подтверждение, а не чистит сразу', async () => {
        const wrapper = mountList(oneRow);

        await wrapper.find(`[data-testid="${testid}"]`).trigger('click');

        expect(wrapper.findComponent(ConfirmationModal).props('show')).toBe(true);
        expect(wrapper.emitted('clear-list')).toBeUndefined();
    });

    it('отказ закрывает окно и оставляет список родителю нетронутым', async () => {
        const wrapper = mountList(oneRow);
        await wrapper.find(`[data-testid="${testid}"]`).trigger('click');

        await wrapper.findComponent(ConfirmationModal).vm.$emit('cancel');

        expect(wrapper.findComponent(ConfirmationModal).props('show')).toBe(false);
        expect(wrapper.emitted('clear-list')).toBeUndefined();
        expect(wrapper.find(`[data-testid="${testid}"]`).exists()).toBe(true);
    });

    it('подтверждение просит родителя очистить и закрывает окно', async () => {
        const wrapper = mountList(oneRow);
        await wrapper.find(`[data-testid="${testid}"]`).trigger('click');

        await wrapper.findComponent(ConfirmationModal).vm.$emit('confirm');

        expect(wrapper.emitted('clear-list')).toHaveLength(1);
        expect(wrapper.findComponent(ConfirmationModal).props('show')).toBe(false);
    });

    it('вопрос называет число строк и отдельно предварительные из бланка', () => {
        const wrapper = mountList(withPending);

        const message = wrapper.findComponent(ConfirmationModal).props('message');
        expect(message).toContain('Будет убрано строк: 2');
        expect(message).toContain('предварительных из бланка: 1');
        expect(message).toContain('Отменить это действие нельзя');
    });
});

async function mountApp() {
    const w = shallowMount(CreateApplication);
    await flushPromises();
    return w;
}

function withCarsAttachment(w, vehicles) {
    w.vm.attachments = [{ local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' }];
    w.vm.vehiclesByAttachment = { c1: vehicles };
    w.vm.selectedAttachment = w.vm.attachments[0];
}

function withPeopleAttachment(w, employees) {
    w.vm.attachments = [{ local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' }];
    w.vm.employeesByAttachment = { p1: employees };
    w.vm.selectedAttachment = w.vm.attachments[0];
}

describe('U6: очистка списка в CreateApplication', () => {
    it('машины: уходят все строки, включая предварительные, сводка импорта показывает ноль', async () => {
        const w = await mountApp();
        withCarsAttachment(w, [VEHICLE, PENDING_VEHICLE, { id: 3, plateNumber: 'С003СС777' }]);
        w.vm.importMode = true;
        await flushPromises();

        expect(w.findComponent(BlankImportPanel).props('pendingCount')).toBe(1);

        await w.findComponent(VehiclesList).vm.$emit('clear-list');

        expect(w.vm.vehiclesByAttachment.c1).toEqual([]);
        expect(w.vm.pendingImportCount).toBe(0);
        expect(w.findComponent(BlankImportPanel).props('pendingCount')).toBe(0);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            bold: 'Убрано строк: 3',
            suffix: ' из списка транспортных средств',
        }));
    });

    it('люди: уходят все строки, включая предварительные', async () => {
        const w = await mountApp();
        withPeopleAttachment(w, [EMPLOYEE, PENDING_EMPLOYEE]);
        await flushPromises();

        await w.findComponent(EmployeesList).vm.$emit('clear-list');

        expect(w.vm.employeesByAttachment.p1).toEqual([]);
        expect(w.vm.pendingImportCount).toBe(0);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            bold: 'Убрано строк: 2',
            suffix: ' из списка сотрудников',
        }));
    });

    it('чистится только текущее вложение - строки соседнего остаются', async () => {
        const w = await mountApp();
        w.vm.attachments = [
            { local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' },
            { local_id: 'c2', id: 11, attachment_type: 'cars', display_name: 'Машины 2' },
        ];
        w.vm.vehiclesByAttachment = { c1: [VEHICLE], c2: [PENDING_VEHICLE] };
        w.vm.selectedAttachment = w.vm.attachments[0];
        await flushPromises();

        await w.findComponent(VehiclesList).vm.$emit('clear-list');

        expect(w.vm.vehiclesByAttachment.c1).toEqual([]);
        expect(w.vm.vehiclesByAttachment.c2).toHaveLength(1);
    });

    it('пустой список ничего не удаляет и молчит - уведомления без причины не будет', async () => {
        const w = await mountApp();
        withCarsAttachment(w, []);
        await flushPromises();
        notifyMock.mockReset();

        w.vm.clearList('cars');

        expect(notifyMock).not.toHaveBeenCalled();
    });
});
