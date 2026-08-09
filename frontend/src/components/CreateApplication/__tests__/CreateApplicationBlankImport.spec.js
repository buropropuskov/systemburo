import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// Эпик blank-import, срез D1D2: загрузка заполненного бланка (D1) и открытие модалки
// результата (D2). Принятые строки уходят в список ТЕМ ЖЕ путём, что ручное массовое
// добавление - handleEmployeesAdded/handleVehiclesAdded (закрыто в срезе E2E3).

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
    it('успешная загрузка открывает модалку результата и закрывает дропзон', async () => {
        const w = await mountApp();
        withSelectedAttachment(w);
        w.vm.showImportDropzone = true;

        const result = { rows: [{ row_number: 1, employee: {}, errors: [], warnings: [] }], summary: { read: 1, accepted: 1, rejected: 0 } };
        uploadImportListMock.mockResolvedValue(result);

        await w.vm.uploadImportFile(new File(['x'], 'blank.xlsx'));

        expect(uploadImportListMock).toHaveBeenCalledWith(9, expect.any(File));
        expect(w.vm.importResult).toEqual(result);
        expect(w.vm.showImportResultModal).toBe(true);
        expect(w.vm.showImportDropzone).toBe(false);
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
        expect(w.vm.showImportResultModal).toBe(false);
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

    it('handleImportRows(people) добавляет строки через handleEmployeesAdded и закрывает модалку', async () => {
        const w = await mountApp();
        w.vm.attachments = [{ local_id: 'p1', attachment_type: 'people', display_name: 'Люди' }];
        w.vm.employeesByAttachment = { p1: [] };
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.showImportResultModal = true;
        w.vm.importResult = { rows: [], summary: {} };

        const rows = [{ lastName: 'Иванов', firstName: 'Иван', isExisting: false }];
        w.vm.handleImportRows({ attachmentType: 'people', rows });

        expect(w.vm.employeesByAttachment.p1).toHaveLength(1);
        expect(w.vm.employeesByAttachment.p1[0]).toMatchObject({ lastName: 'Иванов' });
        expect(w.vm.showImportResultModal).toBe(false);
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

    it('пустой массив rows ничего не делает и не закрывает модалку молча', async () => {
        const w = await mountApp();
        w.vm.showImportResultModal = true;

        w.vm.handleImportRows({ attachmentType: 'people', rows: [] });

        expect(w.vm.showImportResultModal).toBe(true);
        expect(notifyMock).not.toHaveBeenCalled();
    });

    // Ревью D1D2 (замечание 3): открытая шторка дропзона переживала переключение на
    // другое вложение - у него свой шаблон/право, шторка должна закрываться.
    it('переключение на другое вложение закрывает открытую шторку загрузки бланка', async () => {
        const w = await mountApp();
        w.vm.attachments = [
            { local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' },
            { local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' },
        ];
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.showImportDropzone = true;

        await w.vm.handleAttachmentSelected(w.vm.attachments[1]);

        expect(w.vm.showImportDropzone).toBe(false);
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
