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
