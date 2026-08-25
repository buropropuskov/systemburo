import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import VehicleForm from '../VehicleForm.vue';
import EmployeeForm from '../EmployeeForm.vue';
import CreateApplication from '../CreateApplication.vue';
import { apiRequest } from '@/api/client';

// Отмена правки машины/сотрудника раньше пересоздавала форму по :key: заново летели
// запросы справочников, повторно всплывал тост про автовыбор места разгрузки, а блок
// формы схлопывался на время загрузки - страница дёргалась.

const ALL_PLACES = [
    { id: 1, name: 'Ворота 1', status: 'active' },
    { id: 3, name: 'Ворота 3', status: 'active' },
];
const ORG_PLACES = [{ id: 1, name: 'Ворота 1' }];
const FORMATS = [
    { format: { id: 1, name: 'Стандарт', is_default: true }, cells: [{}, {}, {}] },
    { format: { id: 2, name: 'Прицеп', is_default: false }, cells: [{}, {}] },
];
const CITIZENSHIPS = [
    { id: 1, name: 'Россия', is_default: true, patent_required: false },
    { id: 2, name: 'Узбекистан', is_default: false, patent_required: true },
];

vi.mock('@/api/client', () => ({
    apiRequest: vi.fn((url) => {
        if (url === '/unload-places') return Promise.resolve({ ok: true, json: async () => ALL_PLACES });
        if (url === '/organizations/5/unload-places') return Promise.resolve({ ok: true, json: async () => ORG_PLACES });
        if (url === '/license-plate-formats') return Promise.resolve({ ok: true, json: async () => FORMATS });
        if (url === '/citizenships') return Promise.resolve({ ok: true, json: async () => CITIZENSHIPS });
        return Promise.resolve({ ok: true, json: async () => [] });
    }),
}));
vi.mock('@/api/blacklist', () => ({
    checkVehicleBlacklist: vi.fn().mockResolvedValue(null),
    checkPersonBlacklist: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: (...args) => notify(...args), enqueue: vi.fn() })) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([{ id: 4, name: 'BMW', is_active: true }]) }));

beforeEach(() => {
    vi.clearAllMocks();
    notify.mockClear();
    localStorage.clear();
    setActivePinia(createPinia());
});

describe('Отмена правки машины (VehicleForm)', () => {
    async function mountForm() {
        const w = mount(VehicleForm, { props: { userOrganizationId: 5 }, attachTo: document.body });
        await flushPromises();
        return w;
    }

    it('не перезагружает справочники и не повторяет тост автовыбора', async () => {
        const w = await mountForm();
        const callsAfterMount = apiRequest.mock.calls.length;
        notify.mockClear();

        w.vm.editVehicle({ id: 7, plateNumber: 'А123АА', mark: 'BMW', markId: 4, formatId: 2, unloadPlaces: [3], passage_tables: [9] });
        await flushPromises();
        expect(w.vm.editingVehicle).toBeTruthy();

        w.vm.cancelEdit();
        await flushPromises();

        expect(apiRequest.mock.calls.length).toBe(callsAfterMount);
        expect(notify).not.toHaveBeenCalled();
    });

    it('возвращает форму к состоянию новой записи', async () => {
        const w = await mountForm();
        expect(w.vm.selectedUnloadingPlaces).toEqual([1]);

        w.vm.editVehicle({ id: 7, plateNumber: 'А123АА', mark: 'BMW', markId: 4, formatId: 2, unloadPlaces: [3], passage_tables: [9] });
        await flushPromises();

        w.vm.cancelEdit();
        await flushPromises();

        expect(w.vm.editingVehicle).toBeNull();
        expect(w.vm.selectedMark).toBe('');
        expect(w.vm.selectedPassageTables).toEqual([]);
        // Места разгрузки заявки восстановлены (их возвращал ремаунт), формат - дефолтный.
        expect(w.vm.selectedUnloadingPlaces).toEqual([1]);
        expect(w.vm.selectedFormat.format.id).toBe(1);
        expect(w.vm.numberParts).toEqual(['', '', '']);
    });

    it('уже выбранные места разгрузки заявки не теряются', async () => {
        const w = mount(VehicleForm, {
            props: { userOrganizationId: 5, applicationUnloadPlaces: [3] },
            attachTo: document.body,
        });
        await flushPromises();
        expect(w.vm.selectedUnloadingPlaces).toEqual([3]);

        w.vm.editVehicle({ id: 7, plateNumber: 'А123АА', mark: 'BMW', markId: 4, formatId: 2, unloadPlaces: [1], passage_tables: [] });
        await flushPromises();
        w.vm.cancelEdit();
        await flushPromises();

        expect(w.vm.selectedUnloadingPlaces).toEqual([3]);
    });
});

describe('Отмена правки сотрудника (EmployeeForm)', () => {
    it('не перезагружает справочники и возвращает дефолтное гражданство', async () => {
        const w = mount(EmployeeForm, { props: { userOrganizationId: 5 }, attachTo: document.body });
        await flushPromises();
        const callsAfterMount = apiRequest.mock.calls.length;

        w.vm.editEmployee({
            id: 3, lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович',
            position: 'Слесарь', passportSeriesNumber: '1234 567890', patentNumber: '',
            otherPermission: '', citizenshipId: 2, targetTables: [9],
        });
        await flushPromises();
        expect(w.vm.editingEmployee).toBeTruthy();
        expect(w.vm.selectedCitizenship.id).toBe(2);

        w.vm.cancelEdit();
        await flushPromises();

        expect(apiRequest.mock.calls.length).toBe(callsAfterMount);
        expect(w.vm.editingEmployee).toBeNull();
        expect(w.vm.lastName).toBe('');
        expect(w.vm.selectedPassageTables).toEqual([]);
        expect(w.vm.selectedCitizenship.id).toBe(1);
    });
});

describe('CreateApplication: отмена правки не пересоздаёт форму', () => {
    async function mountApp(attachmentType) {
        localStorage.setItem('draftApplicationState', JSON.stringify({
            message: 'черновик',
            attachments: [{ local_id: 'a1', attachment_type: attachmentType, display_name: 'Вложение' }],
        }));
        const w = shallowMount(CreateApplication);
        await flushPromises();
        return w;
    }

    it.each([
        ['cars', 'VehicleForm', 'vehicleFormKey'],
        ['people', 'EmployeeForm', 'employeeFormKey'],
        ['items', 'ItemsForm', 'itemsFormKey'],
    ])('вложение %s: edit-cancelled не меняет ключ формы', async (type, componentName, keyField) => {
        const w = await mountApp(type);
        const form = w.findComponent({ name: componentName });
        expect(form.exists()).toBe(true);

        const keyBefore = w.vm[keyField];
        form.vm.$emit('edit-cancelled');
        await flushPromises();

        expect(w.vm[keyField]).toBe(keyBefore);
    });
});
