import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import BlankImportPanel from '../BlankImportPanel.vue';

// Предварительные строки живут ровно столько, сколько открыт разбор: закрыли панель, не
// нажав «Добавить», - строки уходят из списка вместе с разбором. Правка строки закрытием
// не считается, а после «Добавить» строки уже обычные, и правило их не касается.

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

const VehicleFormStub = {
    name: 'VehicleForm',
    template: '<div class="vehicle-form-stub" />',
    methods: { editVehicle: vi.fn() },
};

const IMPORT_RESULT = { summary: { read: 1, accepted: 1, rejected: 0 }, rows: [] };

async function mountApp() {
    const w = shallowMount(CreateApplication, { global: { stubs: { VehicleForm: VehicleFormStub } } });
    await flushPromises();
    return w;
}

function withCars(w, vehicles, { importMode = true } = {}) {
    w.vm.attachments = [{ local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' }];
    w.vm.vehiclesByAttachment = { c1: vehicles };
    w.vm.selectedAttachment = w.vm.attachments[0];
    w.vm.importMode = importMode;
    if (importMode) w.vm.importResult = IMPORT_RESULT;
}

const SAVED = { id: 1, plateNumber: 'А001АА777', mark: 'Volvo' };
const PENDING = { id: 2, plateNumber: 'В002ВВ777', mark: 'Kia', isPending: true };

beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    notifyMock.mockReset();
    hasImportPermission = true;
});

afterEach(() => {
    hasImportPermission = false;
});

describe('Незавершённый импорт не оставляет серых строк', () => {
    it('закрытие панели убирает предварительные строки и говорит об этом', async () => {
        const w = await mountApp();
        withCars(w, [SAVED, PENDING]);
        await flushPromises();

        w.findComponent(BlankImportPanel).vm.$emit('close');
        await flushPromises();

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
        expect(w.vm.vehiclesByAttachment.c1[0].id).toBe(SAVED.id);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            bold: 'Разбор бланка закрыт: убрано строк 1',
        }));
    });

    it('после «Добавить» строки обычные, и закрытие их не трогает', async () => {
        const w = await mountApp();
        withCars(w, [PENDING]);
        await flushPromises();

        w.vm.handleImportRows({ attachmentType: 'cars', rows: [], places: { unloadPlaces: [30] } });
        await flushPromises();

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
        expect(w.vm.vehiclesByAttachment.c1[0].isPending).toBeUndefined();
        expect(w.vm.importMode).toBe(false);
        // Про уборку не сообщаем - убирать было нечего.
        expect(notifyMock).not.toHaveBeenCalledWith(expect.objectContaining({
            bold: expect.stringContaining('Разбор бланка закрыт'),
        }));
    });

    it('правка строки закрытием не считается - строки остаются на месте', async () => {
        const w = await mountApp();
        withCars(w, [PENDING]);
        await flushPromises();

        w.vm.editVehicle(w.vm.vehicles[0]);
        await flushPromises();

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
        expect(w.vm.vehiclesByAttachment.c1[0].isPending).toBe(true);

        w.vm.handleVehicleUpdated({ ...PENDING, mark: 'Scania' });
        await flushPromises();

        expect(w.vm.vehiclesByAttachment.c1[0].isPending).toBe(true);
        expect(w.findComponent(BlankImportPanel).element.style.display).toBe('');
    });

    it('переключение на другое вложение уносит серые строки того, из которого ушли', async () => {
        const w = await mountApp();
        w.vm.attachments = [
            { local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' },
            { local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' },
        ];
        w.vm.vehiclesByAttachment = { c1: [SAVED, PENDING] };
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.importMode = true;
        w.vm.importResult = IMPORT_RESULT;
        await flushPromises();

        await w.vm.handleAttachmentSelected(w.vm.attachments[1]);
        await flushPromises();

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
        expect(w.vm.vehiclesByAttachment.c1[0].id).toBe(SAVED.id);
    });

    it('очистка списка при открытом импорте не сообщает об уборке дважды', async () => {
        const w = await mountApp();
        withCars(w, [SAVED, PENDING]);
        await flushPromises();

        w.vm.clearList('cars');
        notifyMock.mockReset();
        w.vm.closeImportMode();

        expect(notifyMock).not.toHaveBeenCalled();
    });
});

describe('Перезагрузка страницы: серых строк без панели не бывает', () => {
    it('черновик с предварительными строками открывает сводку по ним', async () => {
        const first = await mountApp();
        withCars(first, [], { importMode: true });
        first.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });
        first.vm.saveToLocalStorage();

        const second = shallowMount(CreateApplication, { global: { stubs: { VehicleForm: VehicleFormStub } } });
        await flushPromises();

        expect(second.vm.vehiclesByAttachment.c1[0].isPending).toBe(true);
        expect(second.vm.importMode).toBe(true);
        const panel = second.findComponent(BlankImportPanel);
        expect(panel.exists()).toBe(true);
        // Разбор перезагрузку не переживает - сводка открывается по одним строкам.
        expect(panel.props('result')).toBeNull();
        expect(panel.props('pendingCount')).toBe(1);
    });

    it('без права на импорт строки из черновика убираются, а не висят без панели', async () => {
        const first = await mountApp();
        withCars(first, [], { importMode: true });
        first.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });
        first.vm.saveToLocalStorage();

        hasImportPermission = false;
        notifyMock.mockReset();
        const second = shallowMount(CreateApplication, { global: { stubs: { VehicleForm: VehicleFormStub } } });
        await flushPromises();

        expect(second.vm.vehiclesByAttachment.c1).toEqual([]);
        expect(second.vm.importMode).toBe(false);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    });
});

// Новое вложение создаётся не через handleAttachmentSelected, а через selectAttachment -
// этим путём панель импорта переживала смену, показывая сводку прошлого вложения, и
// закрыть её было нечем: у другого типа кнопки «Импорт» нет.
describe('CreateApplication - панель импорта не переживает смену вложения', () => {
    it('создание вложения другого типа закрывает режим импорта', async () => {
        const w = await mountApp();
        w.vm.attachments = [
            { local_id: 'c1', attachment_type: 'cars', display_name: 'Авто' },
            { local_id: 'i1', attachment_type: 'items', display_name: 'ТМЦ' },
        ];
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.importMode = true;
        await w.vm.$nextTick();

        w.vm.selectAttachment(w.vm.attachments[1]);
        await w.vm.$nextTick();

        expect(w.vm.importMode).toBe(false);
    });
});
