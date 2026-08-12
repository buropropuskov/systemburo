import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import BlankImportPanel from '../BlankImportPanel.vue';
import VehicleForm from '../VehicleForm.vue';
import VehiclesList from '../VehiclesList.vue';
import EmployeeForm from '../EmployeeForm.vue';
import ItemsList from '../ItemsList.vue';

// Эпик blank-import, срез D1D2: загрузка заполненного бланка (D1) и показ сводки
// результата (D2, с U4 - панель на месте формы). Принятые строки уходят в список ТЕМ ЖЕ
// путём, что ручное массовое добавление - handleEmployeesAdded/handleVehiclesAdded
// (закрыто в срезе E2E3).

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

// Право на импорт списка выдаётся точечно (см. tourCoverage), поэтому по умолчанию его
// нет - вход в режим не должен появляться сам собой.
let hasImportPermission = false;
vi.mock('@/stores/permissions', () => ({
    usePermissionsStore: () => ({
        hasPermission: (key) => hasImportPermission && key === 'action.import.list',
    }),
}));

const uploadImportListMock = vi.fn();
vi.mock('@/api/blankImport', () => ({
    downloadBlankTemplate: vi.fn(),
    uploadImportList: (...args) => uploadImportListMock(...args),
}));

beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    notifyMock.mockReset();
    uploadImportListMock.mockReset();
});

async function mountApp() {
    const w = shallowMount(CreateApplication);
    await flushPromises();
    return w;
}

function withSelectedAttachment(w, type = 'people') {
    w.vm.attachments = [{ local_id: 'a1', id: 9, attachment_type: type, display_name: 'Вложение' }];
    w.vm.selectedAttachment = w.vm.attachments[0];
}

describe('CreateApplication - загрузка заполненного бланка (D1D2)', () => {
    it('успешная загрузка кладёт сводку на место области загрузки', async () => {
        const w = await mountApp();
        withSelectedAttachment(w);
        w.vm.importMode = true;

        const result = { rows: [{ row_number: 1, employee: {}, errors: [], warnings: [] }], summary: { read: 1, accepted: 1, rejected: 0 } };
        uploadImportListMock.mockResolvedValue(result);

        await w.vm.uploadImportFile(new File(['x'], 'blank.xlsx'));

        expect(uploadImportListMock).toHaveBeenCalledWith(9, expect.any(File));
        expect(w.vm.importResult).toEqual(result);
        expect(w.vm.importMode).toBe(true);
        expect(w.vm.importUploading).toBe(false);
    });

    it('403/ошибка сети показывает уведомление и не роняет форму', async () => {
        const w = await mountApp();
        withSelectedAttachment(w);

        uploadImportListMock.mockRejectedValue(new Error('Недостаточно прав для этого действия.'));

        await expect(w.vm.uploadImportFile(new File(['x'], 'blank.xlsx'))).resolves.toBeUndefined();

        expect(notifyMock).toHaveBeenCalledTimes(1);
        const call = notifyMock.mock.calls[0][0];
        expect(call.type).toBe('error');
        expect(call.bold).toContain('Недостаточно прав');
        expect(w.vm.importResult).toBeNull();
        expect(w.vm.importUploading).toBe(false);
        // форма продолжает работать после сбоя загрузки
        expect(w.vm.attachments).toHaveLength(1);
    });

    it('повторный клик, пока файл ещё летит, не шлёт второй запрос', async () => {
        const w = await mountApp();
        withSelectedAttachment(w);

        let resolveUpload;
        uploadImportListMock.mockImplementation(() => new Promise((resolve) => { resolveUpload = resolve; }));

        const first = w.vm.uploadImportFile(new File(['x'], 'a.xlsx'));
        await flushPromises();
        expect(w.vm.importUploading).toBe(true);

        const second = w.vm.uploadImportFile(new File(['x'], 'b.xlsx'));
        await flushPromises();

        resolveUpload({ rows: [], summary: { read: 0, accepted: 0, rejected: 0 } });
        await Promise.all([first, second]);

        expect(uploadImportListMock).toHaveBeenCalledTimes(1);
    });

    it('handleImportRows(people) добавляет строки через handleEmployeesAdded и выходит из импорта', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = { p1: [] };
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.importMode = true;
        w.vm.importResult = { rows: [], summary: {} };

        const rows = [{ lastName: 'Иванов', firstName: 'Иван', isExisting: false }];
        w.vm.handleImportRows({ attachmentType: 'people', rows });

        expect(w.vm.employeesByAttachment.p1).toHaveLength(1);
        expect(w.vm.employeesByAttachment.p1[0]).toMatchObject({ lastName: 'Иванов' });
        expect(w.vm.importMode).toBe(false);
        expect(w.vm.importResult).toBeNull();
        expect(notifyMock).toHaveBeenCalledTimes(1);
    });

    it('handleImportRows(cars) добавляет строки через handleVehiclesAdded', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'c1', attachment_type: 'cars', display_name: 'Машины' }];
        w.vm.vehiclesByAttachment = { c1: [] };
        w.vm.selectedAttachment = w.vm.attachments[0];

        const rows = [{ plateNumber: 'А001АА777', mark: 'Volvo', isExisting: false }];
        w.vm.handleImportRows({ attachmentType: 'cars', rows });

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
        expect(w.vm.vehiclesByAttachment.c1[0]).toMatchObject({ plateNumber: 'А001АА777' });
    });

    it('пустой массив rows ничего не делает и не закрывает сводку молча', async () => {
        const w = await mountApp();
        w.vm.importMode = true;
        w.vm.importResult = { rows: [], summary: {} };

        w.vm.handleImportRows({ attachmentType: 'people', rows: [] });

        expect(w.vm.importMode).toBe(true);
        expect(w.vm.importResult).not.toBeNull();
        expect(notifyMock).not.toHaveBeenCalled();
    });

    // Ревью D1D2 (замечание 3): открытая шторка дропзона переживала переключение на
    // другое вложение - у него свой шаблон/право, режим должен закрываться.
    it('переключение на другое вложение выходит из режима импорта', async () => {
        const w = await mountApp();
        w.vm.attachments = [
            { local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' },
            { local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' },
        ];
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.importMode = true;
        w.vm.importResult = { rows: [], summary: {} };

        await w.vm.handleAttachmentSelected(w.vm.attachments[1]);

        expect(w.vm.importMode).toBe(false);
        expect(w.vm.importResult).toBeNull();
    });
});

// Срез D3: импортированные строки сверяются с тем, что УЖЕ есть в текущем вложении
// заявки - иначе дубль долетает до финального гарда (findDuplicateEntry) и отбивает
// подачу целиком уже после того, как человек заполнил всю форму.
describe('CreateApplication - дедуп импорта против уже добавленного в заявку (D3)', () => {
    it('импортированный дубль уже добавленного сотрудника не попадает в список', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = {
            p1: [{ id: 1, lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510 111111' }],
        };
        w.vm.selectedAttachment = w.vm.attachments[0];

        w.vm.handleImportRows({
            attachmentType: 'people',
            rows: [{ lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510111111' }],
        });

        expect(w.vm.employeesByAttachment.p1).toHaveLength(1);
    });

    it('импортированный дубль уже добавленной машины не попадает в список', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'c1', attachment_type: 'cars', display_name: 'Машины' }];
        w.vm.vehiclesByAttachment = { c1: [{ id: 1, plateNumber: 'A777AA 777', mark: 'BMW' }] };
        w.vm.selectedAttachment = w.vm.attachments[0];

        w.vm.handleImportRows({
            attachmentType: 'cars',
            rows: [{ plateNumber: 'a777aa777', mark: 'BMW' }],
        });

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
    });

    it('о пропущенном при импорте дубле сообщается уведомлением', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = {
            p1: [{ id: 1, lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510 111111' }],
        };
        w.vm.selectedAttachment = w.vm.attachments[0];

        w.vm.handleImportRows({
            attachmentType: 'people',
            rows: [{ lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510111111' }],
        });

        expect(notifyMock).toHaveBeenCalledTimes(1);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            prefix: 'Иванов Иван Иванович ',
            bold: 'уже в списке - пропущен при импорте',
            type: 'error',
        }));
    });

    // employeeLabel для безымянной строки фолбэком отдаёт паспорт - в уведомлении
    // импорта персональных данных быть не должно.
    it('дубль без ФИО назван обезличенно, паспорт в уведомление не попадает', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = {
            p1: [{ id: 1, lastName: '', firstName: '', middleName: '', passportSeriesNumber: '4510 111111' }],
        };
        w.vm.selectedAttachment = w.vm.attachments[0];

        w.vm.handleImportRows({
            attachmentType: 'people',
            rows: [{ lastName: '', firstName: '', middleName: '', passportSeriesNumber: '4510111111' }],
        });

        expect(notifyMock).toHaveBeenCalledTimes(1);
        const call = notifyMock.mock.calls[0][0];
        expect(call.prefix).toBe('Строка без ФИО ');
        expect(JSON.stringify(call)).not.toContain('4510');
    });

    it('уникальные строки из той же пачки проходят, дубль среди них - нет', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = {
            p1: [{ id: 1, lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' }],
        };
        w.vm.selectedAttachment = w.vm.attachments[0];

        w.vm.handleImportRows({
            attachmentType: 'people',
            rows: [
                { lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510111111' }, // дубль уже добавленного
                { lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 222222' }, // новый
            ],
        });

        expect(w.vm.employeesByAttachment.p1.map((e) => e.lastName)).toEqual(['Иванов', 'Петров']);
    });

    // Бэк отсеивает дубли внутри файла на разборе (attachment_import_validate.go) - в
    // rows такого прийти не должно. Но правка проблемной строки прямо в модалке
    // результата (галочка "Добавить") может свести две РАЗНЫЕ строки файла к одному
    // и тому же человеку/машине уже ПОСЛЕ бэкового разбора - гард ловит и это тем же
    // накопительным проходом, тем же приёмом, что addExistingEmployees/addExistingCars
    // для каталога. Решение: осмысленное поведение - в список идёт первая, вторая
    // гасится как дубль с тем же уведомлением, а не долетает до findDuplicateEntry.
    it('дубль внутри самой импортируемой пачки при пустой заявке: добавляется только первая строка', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = { p1: [] };
        w.vm.selectedAttachment = w.vm.attachments[0];

        w.vm.handleImportRows({
            attachmentType: 'people',
            rows: [
                { lastName: 'Сидоров', firstName: 'Сидр', passportSeriesNumber: '4510 333333' },
                { lastName: 'Сидоров', firstName: 'Сидр', passportSeriesNumber: '4510333333' },
            ],
        });

        expect(w.vm.employeesByAttachment.p1).toHaveLength(1);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            bold: 'уже в списке - пропущен при импорте',
            type: 'error',
        }));
    });
});

// Срез U4: вход в массовый ввод переехал из пары кнопок над формой в шапку списка, а
// сам режим подменяет форму ручного ввода панелью загрузки.
describe('CreateApplication - режим импорта (U4)', () => {
    afterEach(() => {
        hasImportPermission = false;
    });

    it('вход в импорт отдаётся списку только при праве action.import.list', async () => {
        const w = await mountApp();
        withSelectedAttachment(w, 'cars');
        await flushPromises();
        expect(w.findComponent(VehiclesList).props('canImport')).toBe(false);

        hasImportPermission = true;
        const withRight = await mountApp();
        withSelectedAttachment(withRight, 'cars');
        await flushPromises();
        expect(withRight.findComponent(VehiclesList).props('canImport')).toBe(true);
    });

    // У ТМЦ списочной части в бланке нет - импортировать нечего даже с правом.
    it('у вложения ТМЦ входа в импорт нет даже с правом', async () => {
        hasImportPermission = true;
        const w = await mountApp();
        withSelectedAttachment(w, 'items');
        await flushPromises();

        expect(w.vm.canImportList).toBe(false);
        expect(w.findComponent(ItemsList).exists()).toBe(true);
    });

    it('вход в режим прячет форму ввода машин и показывает панель загрузки', async () => {
        hasImportPermission = true;
        const w = await mountApp();
        withSelectedAttachment(w, 'cars');
        await flushPromises();
        expect(w.findComponent(VehicleForm).exists()).toBe(true);

        w.findComponent(VehiclesList).vm.$emit('toggle-import');
        await flushPromises();

        expect(w.vm.importMode).toBe(true);
        expect(w.findComponent(VehicleForm).exists()).toBe(false);
        expect(w.findComponent(BlankImportPanel).exists()).toBe(true);
        // Список рядом остаётся: из его шапки и выходят обратно.
        expect(w.findComponent(VehiclesList).exists()).toBe(true);
        expect(w.findComponent(VehiclesList).props('importActive')).toBe(true);
    });

    it('вход в режим прячет форму ввода сотрудников', async () => {
        hasImportPermission = true;
        const w = await mountApp();
        withSelectedAttachment(w, 'people');
        await flushPromises();
        expect(w.findComponent(EmployeeForm).exists()).toBe(true);

        w.vm.toggleImportMode();
        await flushPromises();

        expect(w.findComponent(EmployeeForm).exists()).toBe(false);
        expect(w.findComponent(BlankImportPanel).exists()).toBe(true);
    });

    it('после загрузки панель получает сводку с теми же счётчиками, что вернул сервер', async () => {
        hasImportPermission = true;
        const w = await mountApp();
        withSelectedAttachment(w, 'people');
        w.vm.importMode = true;
        await flushPromises();
        expect(w.findComponent(BlankImportPanel).props('result')).toBeNull();

        const result = {
            rows: [{ row_number: 1, employee: {}, errors: [], warnings: [] }],
            summary: { read: 5, accepted: 4, rejected: 1 },
        };
        uploadImportListMock.mockResolvedValue(result);

        await w.findComponent(BlankImportPanel).vm.$emit('file', new File(['x'], 'blank.xlsx'));
        await flushPromises();

        expect(w.findComponent(BlankImportPanel).props('result')).toEqual(result);
    });

    it('выход из режима возвращает форму и гасит разобранный файл', async () => {
        hasImportPermission = true;
        const w = await mountApp();
        withSelectedAttachment(w, 'cars');
        w.vm.importMode = true;
        w.vm.importResult = { rows: [], summary: { read: 1, accepted: 1, rejected: 0 } };
        await flushPromises();

        w.findComponent(VehiclesList).vm.$emit('toggle-import');
        await flushPromises();

        expect(w.vm.importMode).toBe(false);
        expect(w.vm.importResult).toBeNull();
        expect(w.findComponent(VehicleForm).exists()).toBe(true);
        expect(w.findComponent(BlankImportPanel).exists()).toBe(false);
    });

    // "Загрузить другой файл" в сводке - не выход из режима: остаёмся в панели, но с
    // чистой областью загрузки.
    it('сброс сводки оставляет режим импорта открытым', async () => {
        hasImportPermission = true;
        const w = await mountApp();
        withSelectedAttachment(w, 'people');
        w.vm.importMode = true;
        w.vm.importResult = { rows: [], summary: { read: 1, accepted: 1, rejected: 0 } };
        await flushPromises();

        await w.findComponent(BlankImportPanel).vm.$emit('reset');
        await flushPromises();

        expect(w.vm.importMode).toBe(true);
        expect(w.findComponent(BlankImportPanel).props('result')).toBeNull();
    });
});

// Список рядом с панелью остаётся кликабельным, а правка строки живёт в форме ручного
// ввода - без показа формы кнопка «Редактировать» была бы мёртвой. Разбор при этом не
// теряется: панель уступает форме место, а не закрывается.
describe('CreateApplication - правка строки из списка в режиме импорта (U4)', () => {
    const vehicleEdit = vi.fn();
    const employeeEdit = vi.fn();

    // Формы подменяем заглушками с теми же методами, что зовёт родитель: важно не что
    // делает форма, а что она вообще получила строку.
    const formStubs = {
        VehicleForm: {
            name: 'VehicleForm',
            template: '<div />',
            methods: { editVehicle: (...args) => vehicleEdit(...args) },
        },
        EmployeeForm: {
            name: 'EmployeeForm',
            template: '<div />',
            methods: { editEmployee: (...args) => employeeEdit(...args) },
        },
    };

    async function mountWithFormStubs() {
        const w = shallowMount(CreateApplication, { global: { stubs: formStubs } });
        await flushPromises();
        return w;
    }

    beforeEach(() => {
        hasImportPermission = true;
        vehicleEdit.mockReset();
        employeeEdit.mockReset();
    });

    afterEach(() => {
        hasImportPermission = false;
    });

    it('правка машины отдаёт строку форме и сохраняет разбор', async () => {
        const w = await mountWithFormStubs();
        w.vm.attachments = [{ local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' }];
        w.vm.vehiclesByAttachment = { c1: [{ id: 1, plateNumber: 'А001АА777', mark: 'Volvo' }] };
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.importMode = true;
        w.vm.importResult = { rows: [], summary: { read: 1, accepted: 1, rejected: 0 } };
        await flushPromises();

        w.vm.editVehicle(w.vm.vehicles[0]);
        await flushPromises();

        expect(vehicleEdit).toHaveBeenCalledWith(expect.objectContaining({ plateNumber: 'А001АА777' }));
        // Режим остаётся открытым, панель лишь уступает форме место - разбор бланка
        // переживает правку строки и возвращается после неё.
        expect(w.vm.importMode).toBe(true);
        expect(w.vm.importResult).not.toBeNull();
        expect(w.findComponent(BlankImportPanel).exists()).toBe(true);
        expect(notifyMock).not.toHaveBeenCalled();
    });

    it('правка строки сотрудника показывает форму и не трогает режим', async () => {
        const w = await mountWithFormStubs();
        w.vm.attachments = [{ local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = { p1: [{ id: 1, lastName: 'Иванов', firstName: 'Иван' }] };
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.importMode = true;
        await flushPromises();

        w.vm.editEmployee(w.vm.employees[0]);
        await flushPromises();

        expect(w.vm.importMode).toBe(true);
        expect(employeeEdit).toHaveBeenCalledWith(expect.objectContaining({ lastName: 'Иванов' }));
        expect(notifyMock).not.toHaveBeenCalled();
    });
});
