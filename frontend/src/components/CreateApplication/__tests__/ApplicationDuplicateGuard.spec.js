import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import EmployeeForm from '../EmployeeForm.vue';
import VehicleForm from '../VehicleForm.vue';

// Один и тот же сотрудник/машина не должны попадать в список вложения дважды.
// Позитивный кейс в каждом блоке доказывает, что форма валидна и добавление вообще
// проходит - иначе гард нельзя отличить от заблокированной кнопки.

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

vi.mock('@/api/client', () => ({
    apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [] }),
}));
vi.mock('@/api/blacklist', () => ({
    checkPersonBlacklist: vi.fn().mockResolvedValue(null),
    checkVehicleBlacklist: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })) }));
vi.mock('@/components/CreateApplication/ExistingEmployeesModal.vue', () => ({
    default: { name: 'ExistingEmployeesModal', template: '<div />' },
}));
vi.mock('@/components/CreateApplication/ExistingCarsModal.vue', () => ({
    default: { name: 'ExistingCarsModal', template: '<div />' },
}));

// Обязательность полей приходит из шаблона вложения; здесь снимаем её, чтобы тест
// проверял гард дублей, а не правила заполнения формы.
const optional = (...keys) => Object.fromEntries(keys.map(key => [key, { visible: true, required: false }]));

const EMPLOYEE_CONFIG = {
    ...optional('last_name', 'first_name', 'position', 'citizenship', 'passport', 'target_tables'),
    patent: { visible: false, required: false },
    work_permission: { visible: false, required: false },
};

const VEHICLE_CONFIG = optional('number', 'mark', 'unloading_places', 'passage_tables');

const ADDED_EMPLOYEE = {
    id: 1,
    lastName: 'Иванов',
    firstName: 'Иван',
    middleName: 'Иванович',
    passportSeriesNumber: '4510 111111',
    isExisting: false,
};

const ADDED_VEHICLE = { id: 1, plateNumber: 'A777AA 777', mark: 'BMW', isExisting: false };

async function mountEmployeeForm(existingEmployees = []) {
    const w = mount(EmployeeForm, { props: { fieldConfig: EMPLOYEE_CONFIG, existingEmployees } });
    await flushPromises();
    await w.setData({ selectedCitizenship: { id: 1, name: 'Россия' } });
    return w;
}

async function fillEmployee(w, { lastName, firstName, middleName = '', passportSeriesNumber = '' }) {
    // pdConsent: форма не даёт добавить человека без отметки о согласии субъекта на
    // обработку персональных данных - для этих кейсов она просто должна быть.
    await w.setData({ lastName, firstName, middleName, passportSeriesNumber, position: 'Слесарь', pdConsent: true });
}

// Период однодневный: машине «По факту» иначе нельзя (#2320), а прочие проверки
// формы от него не зависят.
const ONE_DAY_PERIOD = { date_from: '2026-09-05', date_to: '2026-09-05', time_from: '09:00', time_to: '18:00' };

async function mountVehicleForm(existingVehicles = [], entryPeriod = ONE_DAY_PERIOD) {
    const w = mount(VehicleForm, { props: { fieldConfig: VEHICLE_CONFIG, existingVehicles, entryPeriod } });
    await flushPromises();
    return w;
}

// Номер собирается из ячеек формата (numberParts.join(' ')), формат в тесте не нужен.
async function fillPlate(w, parts) {
    await w.setData({ isNumberByFact: false, numberParts: parts, selectedMark: 'BMW' });
}

beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    localStorage.clear();
});

describe('CreateApplication - гард дублей перед подачей', () => {
    const PEOPLE_ATTACHMENT = { local_id: 'a1', attachment_type: 'people', display_name: 'Люди' };
    const CARS_ATTACHMENT = { local_id: 'a2', attachment_type: 'cars', display_name: 'Машины' };

    async function mountApp(data) {
        const w = shallowMount(CreateApplication);
        await flushPromises();
        await w.setData(data);
        return w;
    }

    it('черновик с повтором человека - подача не проходит', async () => {
        const w = await mountApp({
            attachments: [PEOPLE_ATTACHMENT],
            employeesByAttachment: {
                a1: [
                    { id: 1, lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510 111111' },
                    { id: 2, lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510111111' },
                ],
            },
        });

        expect(w.vm.findDuplicateEntry()).toEqual({ attachmentName: 'Люди', label: 'Иванов Иван Иванович' });
    });

    it('черновик с повтором машины - подача не проходит', async () => {
        const w = await mountApp({
            attachments: [CARS_ATTACHMENT],
            vehiclesByAttachment: {
                a2: [
                    { id: 1, plateNumber: 'A777AA 777', mark: 'BMW' },
                    { id: 2, plateNumber: 'a777aa777', mark: 'Toyota' },
                ],
            },
        });

        expect(w.vm.findDuplicateEntry()).toEqual({ attachmentName: 'Машины', label: 'Toyota a777aa777' });
    });

    it('одно лицо в двух разных вложениях - не повтор', async () => {
        const person = { id: 1, lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' };
        const w = await mountApp({
            attachments: [PEOPLE_ATTACHMENT, { local_id: 'a3', attachment_type: 'people', display_name: 'Люди 2' }],
            employeesByAttachment: { a1: [person], a3: [{ ...person, id: 2 }] },
        });

        expect(w.vm.findDuplicateEntry()).toBeNull();
    });

    // Две «По факту» здесь остаются: findDuplicateEntry отвечает на вопрос «одна и
    // та же машина дважды», а не «сколько безымянных пропусков в заявке» - второе
    // проверяет правило #2320 в самой форме.
    it('чистый черновик проходит', async () => {
        const w = await mountApp({
            attachments: [PEOPLE_ATTACHMENT, CARS_ATTACHMENT],
            employeesByAttachment: { a1: [{ id: 1, lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' }] },
            vehiclesByAttachment: { a2: [{ id: 1, plateNumber: 'По факту', mark: 'По факту' }, { id: 2, plateNumber: 'По факту', mark: 'BMW' }] },
        });

        expect(w.vm.findDuplicateEntry()).toBeNull();
    });
});

describe('EmployeeForm - запрет повторного добавления сотрудника', () => {
    it('уникальный сотрудник добавляется', async () => {
        const w = await mountEmployeeForm([ADDED_EMPLOYEE]);
        await fillEmployee(w, { lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 222222' });

        w.vm.addEmployee();

        expect(w.emitted('employee-added')).toHaveLength(1);
        expect(notifyMock).not.toHaveBeenCalled();
    });

    it('тот же паспорт - не добавляется, показывается уведомление', async () => {
        const w = await mountEmployeeForm([ADDED_EMPLOYEE]);
        await fillEmployee(w, { lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510111111' });

        w.vm.addEmployee();

        expect(w.emitted('employee-added')).toBeUndefined();
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            prefix: 'Иванов Иван Иванович ',
            bold: 'уже добавлен в список',
            type: 'error',
        }));
    });

    it('без паспорта дубль ловится по ФИО', async () => {
        const w = await mountEmployeeForm([{ id: 1, lastName: 'Сидоров', firstName: 'Сидр', middleName: 'Сидорович' }]);
        await fillEmployee(w, { lastName: ' сидоров ', firstName: 'СИДР', middleName: 'Сидорович' });

        w.vm.addEmployee();

        expect(w.emitted('employee-added')).toBeUndefined();
        expect(notifyMock).toHaveBeenCalled();
    });

    it('тёзка с другим паспортом добавляется', async () => {
        const w = await mountEmployeeForm([ADDED_EMPLOYEE]);
        await fillEmployee(w, { lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510 999999' });

        w.vm.addEmployee();

        expect(w.emitted('employee-added')).toHaveLength(1);
    });

    it('правка своей же строки не считается дублем', async () => {
        const w = await mountEmployeeForm([ADDED_EMPLOYEE]);
        await w.setData({ editingEmployee: ADDED_EMPLOYEE });
        await fillEmployee(w, { lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510 111111' });

        w.vm.addEmployee();

        expect(w.emitted('employee-updated')).toHaveLength(1);
        expect(notifyMock).not.toHaveBeenCalled();
    });

    it('правка строки в копию другой строки блокируется', async () => {
        const second = { id: 2, lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 222222' };
        const w = await mountEmployeeForm([ADDED_EMPLOYEE, second]);
        await w.setData({ editingEmployee: ADDED_EMPLOYEE });
        await fillEmployee(w, { lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 222222' });

        w.vm.addEmployee();

        expect(w.emitted('employee-updated')).toBeUndefined();
        expect(notifyMock).toHaveBeenCalled();
    });

    it('выбор существующих: уже добавленные отсеиваются, остальные проходят', async () => {
        const w = await mountEmployeeForm([{ id: 1, isExisting: true, existingEmployeeId: 5, lastName: 'Иванов', firstName: 'Иван' }]);
        await w.setData({
            selectedPassageTables: [10],
            selectedExistingEmployees: [
                { id: 5, last_name: 'Иванов', first_name: 'Иван', middle_name: '', passport_series_number: '' },
                { id: 6, last_name: 'Петров', first_name: 'Пётр', middle_name: '', passport_series_number: '4510 222222' },
            ],
        });

        w.vm.addExistingEmployees();

        const added = w.emitted('employees-added');
        expect(added).toHaveLength(1);
        expect(added[0][0]).toHaveLength(1);
        expect(added[0][0][0].existingEmployeeId).toBe(6);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    });
});

describe('VehicleForm - запрет повторного добавления машины', () => {
    it('уникальный номер добавляется', async () => {
        const w = await mountVehicleForm([ADDED_VEHICLE]);
        await fillPlate(w, ['B111BB', '77']);

        w.vm.addVehicle();

        expect(w.emitted('vehicle-added')).toHaveLength(1);
        expect(notifyMock).not.toHaveBeenCalled();
    });

    it('тот же номер - не добавляется, показывается уведомление', async () => {
        const w = await mountVehicleForm([ADDED_VEHICLE]);
        await fillPlate(w, ['a777aa', '777']);

        w.vm.addVehicle();

        expect(w.emitted('vehicle-added')).toBeUndefined();
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            prefix: 'BMW A777AA 777 ',
            bold: 'уже добавлена в список',
            type: 'error',
        }));
    });

    it('тот же номер с другой маркой - тоже дубль', async () => {
        const w = await mountVehicleForm([ADDED_VEHICLE]);
        await fillPlate(w, ['A777AA', '777']);
        await w.setData({ selectedMark: 'Toyota' });

        w.vm.addVehicle();

        expect(w.emitted('vehicle-added')).toBeUndefined();
    });

    // Раньше «По факту» намеренно пропускали мимо проверки дублей: такой номер не
    // опознаёт машину, и строк могло быть сколько угодно. С #2320 правило другое -
    // одна такая машина на заявку, потому что по безымянному пропуску заезжает кто
    // угодно. Проверка дублей по-прежнему их не сравнивает (разные машины), но
    // добавить вторую не даёт отдельное правило.
    it('вторая машина "По факту" не добавляется', async () => {
        const w = await mountVehicleForm([{ id: 1, plateNumber: 'По факту', mark: 'По факту' }]);
        await w.setData({ isNumberByFact: true, isMarkByFact: true });

        w.vm.addVehicle();

        expect(w.emitted('vehicle-added')).toBeUndefined();
    });

    it('машину "По факту" не добавить, пока период длиннее дня', async () => {
        const w = await mountVehicleForm([], { date_from: '2026-09-05', date_to: '2026-10-05' });
        await w.setData({ isNumberByFact: true, isMarkByFact: true });

        w.vm.addVehicle();

        expect(w.emitted('vehicle-added')).toBeUndefined();
    });

    it('первая машина "По факту" добавляется свободно', async () => {
        const w = await mountVehicleForm([{ id: 1, plateNumber: 'A777AA 777', mark: 'BMW' }]);
        await w.setData({ isNumberByFact: true, isMarkByFact: true });

        w.vm.addVehicle();

        expect(w.emitted('vehicle-added')).toHaveLength(1);
        expect(notifyMock).not.toHaveBeenCalled();
    });

    it('правка своей же строки не считается дублем', async () => {
        const w = await mountVehicleForm([ADDED_VEHICLE]);
        await w.setData({ editingVehicle: ADDED_VEHICLE });
        await fillPlate(w, ['A777AA', '777']);

        w.vm.addVehicle();

        expect(w.emitted('vehicle-updated')).toHaveLength(1);
        expect(notifyMock).not.toHaveBeenCalled();
    });

    it('выбор существующих: уже добавленные отсеиваются', async () => {
        const w = await mountVehicleForm([ADDED_VEHICLE]);
        await w.setData({
            selectedUnloadingPlaces: [3],
            selectedExistingCars: [
                { id: 5, number: 'A777AA777', mark: 'BMW' },
                { id: 6, number: 'C222CC 22', mark: 'Toyota' },
            ],
        });

        w.vm.addExistingCars();

        const added = w.emitted('vehicles-added');
        expect(added).toHaveLength(1);
        expect(added[0][0]).toHaveLength(1);
        expect(added[0][0][0].existingCarId).toBe(6);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    });
});
