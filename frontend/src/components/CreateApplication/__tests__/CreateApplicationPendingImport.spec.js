import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import BlankImportPanel from '../BlankImportPanel.vue';
import VehiclesList from '../VehiclesList.vue';
import EmployeesList from '../EmployeesList.vue';
import { apiRequest } from '@/api/client';
import { toEmployeePayload } from '@/utils/applicationEntityPayload';

// Эпик blank-import-ux, срез U5: разобранные строки бланка попадают в список СРАЗУ, но
// предварительными - серыми, с рабочими действиями, в черновике и мимо подачи. Обычными
// они становятся по «Добавить» в сводке, которое заодно раскатывает выбранные места.

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

beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    notifyMock.mockReset();
    apiRequest.mockClear();
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) });
    hasImportPermission = true;
});

afterEach(() => {
    hasImportPermission = false;
});

async function mountApp() {
    const w = shallowMount(CreateApplication);
    await flushPromises();
    return w;
}

// Строки заводит открытая панель импорта, поэтому режим включаем сразу: гард
// stageImportRows намеренно отбрасывает опоздавший разбор при закрытом режиме.
function withCarsAttachment(w, vehicles = []) {
    w.vm.attachments = [{ local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' }];
    w.vm.vehiclesByAttachment = { c1: vehicles };
    w.vm.selectedAttachment = w.vm.attachments[0];
    w.vm.importMode = true;
}

function withPeopleAttachment(w, employees = []) {
    w.vm.attachments = [{ local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' }];
    w.vm.employeesByAttachment = { p1: employees };
    w.vm.selectedAttachment = w.vm.attachments[0];
    w.vm.importMode = true;
}

const CAR_PLACES = { unloadPlaces: [30], unloadingPlace: 'Склад 1', passage_tables: [20] };
const PEOPLE_PLACES = { targetTables: [10], passageTables: 'Проход 1' };

describe('U5: разобранные строки сразу встают в список предварительными', () => {
    it('панель отдаёт разбор наверх, строки видны в списке и помечены предварительными', async () => {
        const w = await mountApp();
        withCarsAttachment(w);
        w.vm.importMode = true;
        await flushPromises();

        w.findComponent(BlankImportPanel).vm.$emit('stage', {
            attachmentType: 'cars',
            rows: [{ plateNumber: 'А001АА777', mark: 'Volvo', isExisting: false }],
        });
        await flushPromises();

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
        expect(w.vm.vehiclesByAttachment.c1[0]).toMatchObject({ plateNumber: 'А001АА777', isPending: true });
        // Режим импорта не закрывается: человек ещё выбирает места.
        expect(w.vm.importMode).toBe(true);
        expect(w.findComponent(VehiclesList).props('vehicles')[0].isPending).toBe(true);
    });

    // Сводка отдаёт строки после того, как дождалась справочника гражданств: за это время
    // человек успевает выйти из режима, и опоздавший эмит не должен лечь в чужое вложение.
    it('опоздавший разбор не кладёт строки после выхода из режима импорта', async () => {
        const w = await mountApp();
        withCarsAttachment(w);
        w.vm.importMode = false;

        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(0);
    });

    it('опоздавший разбор не кладёт строки в другое вложение', async () => {
        const w = await mountApp();
        w.vm.attachments = [
            { local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' },
            { local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' },
        ];
        w.vm.vehiclesByAttachment = { c1: [] };
        w.vm.employeesByAttachment = { p1: [] };
        w.vm.selectedAttachment = w.vm.attachments[1];
        w.vm.importMode = true;

        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(0);
        expect(w.vm.employeesByAttachment.p1).toHaveLength(0);
    });

    it('дедуп против уже добавленного работает на этапе разбора, а не при «Добавить»', async () => {
        const w = await mountApp();
        withCarsAttachment(w, [{ id: 1, plateNumber: 'A777AA 777', mark: 'BMW' }]);
        w.vm.importMode = true;

        w.vm.stageImportRows({
            attachmentType: 'cars',
            rows: [
                { plateNumber: 'a777aa777', mark: 'BMW' },
                { plateNumber: 'В002ВВ777', mark: 'Kia' },
            ],
        });

        expect(w.vm.vehiclesByAttachment.c1.map((v) => v.plateNumber)).toEqual(['A777AA 777', 'В002ВВ777']);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
            bold: 'уже в списке - пропущена при импорте',
            type: 'error',
        }));
    });
});

describe('U5: предварительная строка живёт как обычная - удаление и правка', () => {
    it('удаление предварительной строки пересчитывает счётчик сводки', async () => {
        const w = await mountApp();
        withCarsAttachment(w);
        w.vm.importMode = true;
        w.vm.stageImportRows({
            attachmentType: 'cars',
            rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }, { plateNumber: 'В002ВВ777', mark: 'Kia' }],
        });
        await flushPromises();

        expect(w.vm.pendingImportCount).toBe(2);
        expect(w.findComponent(BlankImportPanel).props('pendingCount')).toBe(2);

        w.vm.deleteVehicle(w.vm.vehiclesByAttachment.c1[0].id);
        await flushPromises();

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(1);
        expect(w.vm.pendingImportCount).toBe(1);
        expect(w.findComponent(BlankImportPanel).props('pendingCount')).toBe(1);
    });

    it('правка предварительной строки не делает её обычной', async () => {
        const w = await mountApp();
        withCarsAttachment(w);
        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        const staged = w.vm.vehiclesByAttachment.c1[0];
        // Форма отдаёт свой объект строки и про служебный флаг ничего не знает.
        w.vm.handleVehicleUpdated({ id: staged.id, plateNumber: 'А003АА777', mark: 'Volvo' });

        expect(w.vm.vehiclesByAttachment.c1[0]).toMatchObject({ plateNumber: 'А003АА777', isPending: true });
    });

    it('правка обычной строки не делает её предварительной', async () => {
        const w = await mountApp();
        withPeopleAttachment(w, [{ id: 1, lastName: 'Иванов', firstName: 'Иван' }]);

        w.vm.handleEmployeeUpdated({ id: 1, lastName: 'Петров', firstName: 'Иван' });

        expect(w.vm.employeesByAttachment.p1[0].isPending).toBeUndefined();
    });
});

describe('U5: «Добавить» переводит предварительные строки в обычные', () => {
    it('машины получают выбранные места и теряют предварительный флаг', async () => {
        const w = await mountApp();
        withCarsAttachment(w);
        w.vm.importMode = true;
        w.vm.importResult = { rows: [], summary: { read: 2, accepted: 2, rejected: 0 } };
        w.vm.stageImportRows({
            attachmentType: 'cars',
            rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }, { plateNumber: 'В002ВВ777', mark: 'Kia' }],
        });
        notifyMock.mockReset();

        w.vm.handleImportRows({ attachmentType: 'cars', rows: [], places: CAR_PLACES });

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(2);
        w.vm.vehiclesByAttachment.c1.forEach((vehicle) => {
            expect(vehicle.isPending).toBeUndefined();
            expect(vehicle.unloadPlaces).toEqual([30]);
            expect(vehicle.passage_tables).toEqual([20]);
            expect(vehicle.unloadingPlace).toBe('Склад 1');
        });
        expect(w.vm.importMode).toBe(false);
        expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Добавлено строк: 2' }));
    });

    it('люди получают места прохода и теряют предварительный флаг', async () => {
        const w = await mountApp();
        withPeopleAttachment(w);
        w.vm.importMode = true;
        w.vm.stageImportRows({
            attachmentType: 'people',
            rows: [{ lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' }],
        });

        w.vm.handleImportRows({ attachmentType: 'people', rows: [], places: PEOPLE_PLACES });

        expect(w.vm.employeesByAttachment.p1[0]).toMatchObject({
            lastName: 'Иванов',
            targetTables: [10],
            passageTables: 'Проход 1',
        });
        expect(w.vm.employeesByAttachment.p1[0].isPending).toBeUndefined();
        expect(w.vm.pendingImportCount).toBe(0);
    });

    // Сквозной замок отметки согласия: в бланке колонки под неё нет, поэтому сводка
    // ставит одну отметку на пачку и отдаёт её тем же патчем, что и места. Разорви эту
    // связь - работники уедут в заявку без согласия, причём молча: серверный гейт по
    // умолчанию не обязателен, подача пройдёт, а следа в базе не останется.
    it('отметка согласия из сводки доезжает до строк и до тела заявки', async () => {
        const w = await mountApp();
        withPeopleAttachment(w);
        w.vm.importMode = true;
        w.vm.stageImportRows({
            attachmentType: 'people',
            rows: [{ lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' }],
        });

        w.vm.handleImportRows({
            attachmentType: 'people',
            rows: [],
            places: { ...PEOPLE_PLACES, pdConsent: true },
        });

        const row = w.vm.employeesByAttachment.p1[0];
        expect(row.pdConsent).toBe(true);
        expect(toEmployeePayload([row])[0].pd_consent).toBe(true);
    });

    it('исправленные вручную строки приходят вместе с «Добавить» и идут в список обычными', async () => {
        const w = await mountApp();
        withCarsAttachment(w);
        w.vm.importMode = true;
        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        w.vm.handleImportRows({
            attachmentType: 'cars',
            rows: [{ plateNumber: 'В002ВВ777', mark: 'Kia', ...CAR_PLACES }],
            places: CAR_PLACES,
        });

        expect(w.vm.vehiclesByAttachment.c1).toHaveLength(2);
        expect(w.vm.vehiclesByAttachment.c1.every((v) => !v.isPending)).toBe(true);
    });
});

describe('U5: черновик хранит предварительные строки, подача - нет', () => {
    async function submitCars(w) {
        w.vm.responsiblePerson = 'Иванов И.И.';
        w.vm.phoneNumber = '+7 (999) 000-00-00';
        w.vm.consentGiven = true;
        w.vm.organization = 'ООО Ромашка';
        await w.vm.sendCompleteApplication();
        const call = apiRequest.mock.calls.find(([url]) => String(url).includes('submit-complete-application'));
        expect(call).toBeTruthy();
        return JSON.parse(call[1].body);
    }

    it('тело подачи не содержит предварительных машин, черновик содержит', async () => {
        const w = await mountApp();
        withCarsAttachment(w, [{ id: 1, plateNumber: 'A777AA 777', mark: 'BMW' }]);
        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        const body = await submitCars(w);

        expect(body.attachments[0].data.vehicles).toHaveLength(1);
        expect(body.attachments[0].data.vehicles[0].car_number).toBe('A777AA 777');

        const draft = JSON.parse(localStorage.getItem('draftApplicationState'));
        expect(draft.vehiclesByAttachment.c1).toHaveLength(2);
        expect(draft.vehiclesByAttachment.c1.filter((v) => v.isPending)).toHaveLength(1);
    });

    it('тело подачи не содержит предварительных людей', async () => {
        const w = await mountApp();
        withPeopleAttachment(w, [{ id: 1, lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' }]);
        w.vm.stageImportRows({
            attachmentType: 'people',
            rows: [{ lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 222222' }],
        });

        const body = await submitCars(w);

        expect(body.attachments[0].data.employees).toHaveLength(1);
        expect(body.attachments[0].data.employees[0].last_name).toBe('Иванов');
    });

    it('привязка к справочнику не берёт предварительные строки', async () => {
        const w = await mountApp();
        withCarsAttachment(w, [{ id: 1, plateNumber: 'A777AA 777', mark: 'BMW', isExisting: false }]);
        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        await w.vm.collectNewDataForBinding();

        expect(w.vm.newVehiclesToBind.map((v) => v.plateNumber)).toEqual(['A777AA 777']);
    });

    it('пока строки предварительные, отправка заявки заблокирована с понятной причиной', async () => {
        const w = await mountApp();
        withCarsAttachment(w, [{ id: 1, plateNumber: 'A777AA 777', mark: 'BMW' }]);
        w.vm.responsiblePerson = 'Иванов И.И.';
        w.vm.phoneNumber = '+7 (999) 000-00-00';
        w.vm.consentGiven = true;
        w.vm.organization = 'ООО Ромашка';
        w.vm.attachmentDatesByAttachment = { c1: w.vm.getDefaultDateData() };
        w.vm.attachmentDatesByAttachment.c1.singleDate = '01.01.2027';
        expect(w.vm.submitValidation.join(' ')).not.toContain('предварительные');

        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        expect(w.vm.canSubmit).toBe(false);
        expect(w.vm.submitValidation.join(' ')).toContain('строки из бланка ещё предварительные (1)');
    });
});

describe('U5: смена вложения и перезагрузка', () => {
    // Доводка: предварительные строки живут ровно столько, сколько открыт разбор, а смена
    // вложения его закрывает - значит строки уходят вместе с ним, а не остаются серыми в
    // списке, вернуться к которому уже нечем.
    it('переключение вложений уносит предварительные строки вместе с разбором', async () => {
        const w = await mountApp();
        w.vm.attachments = [
            { local_id: 'c1', id: 10, attachment_type: 'cars', display_name: 'Машины' },
            { local_id: 'p1', id: 9, attachment_type: 'people', display_name: 'Люди' },
        ];
        w.vm.vehiclesByAttachment = { c1: [] };
        w.vm.selectedAttachment = w.vm.attachments[0];
        w.vm.importMode = true;
        w.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });

        await w.vm.handleAttachmentSelected(w.vm.attachments[1]);
        await flushPromises();

        expect(w.vm.importMode).toBe(false);
        expect(w.vm.vehiclesByAttachment.c1).toEqual([]);
    });

    it('черновик после перезагрузки возвращает строку предварительной вместе со сводкой', async () => {
        const first = await mountApp();
        withCarsAttachment(first);
        first.vm.stageImportRows({ attachmentType: 'cars', rows: [{ plateNumber: 'А001АА777', mark: 'Volvo' }] });
        first.vm.saveToLocalStorage();

        const second = shallowMount(CreateApplication);
        await flushPromises();

        expect(second.vm.vehiclesByAttachment.c1[0].isPending).toBe(true);
        // Серых строк без сводки не бывает: разбор перезагрузку не переживает, поэтому
        // сводка открывается по самим строкам.
        expect(second.vm.importMode).toBe(true);
    });

    it('после перезагрузки сводка открывается по предварительным строкам без нового файла', async () => {
        const w = await mountApp();
        withCarsAttachment(w, [{ id: 1, plateNumber: 'А001АА777', mark: 'Volvo', isPending: true }]);
        w.vm.importMode = true;
        await flushPromises();

        const panel = w.findComponent(BlankImportPanel);
        expect(panel.props('result')).toBeNull();
        expect(panel.props('pendingCount')).toBe(1);
    });
});

describe('U5: список показывает предварительные строки приглушёнными', () => {
    it('колоночная и карточная раскладки машин помечают предварительную строку', () => {
        const vehicles = [
            { id: 1, plateNumber: 'A777AA777', mark: 'BMW' },
            { id: 2, plateNumber: 'В002ВВ777', mark: 'Kia', isPending: true },
        ];

        const desktop = shallowMount(VehiclesList, { props: { vehicles } });
        const desktopCells = desktop.findAll('[data-testid="vehicles-row"]');
        expect(desktopCells).toHaveLength(2);
        expect(desktopCells[0].classes()).not.toContain('vcol__cell--pending');
        expect(desktopCells[1].classes()).toContain('vcol__cell--pending');

        const mobile = shallowMount(VehiclesList, { props: { vehicles } });
        mobile.vm.isNarrow = true;
        return mobile.vm.$nextTick().then(() => {
            const rows = mobile.findAll('[data-testid="vehicles-row"]');
            expect(rows[0].classes()).not.toContain('is-pending');
            expect(rows[1].classes()).toContain('is-pending');
        });
    });

    it('список сотрудников помечает предварительную строку', () => {
        const w = shallowMount(EmployeesList, {
            props: {
                employees: [
                    { id: 1, lastName: 'Иванов', firstName: 'Иван' },
                    { id: 2, lastName: 'Петров', firstName: 'Пётр', isPending: true },
                ],
            },
        });

        const rows = w.findAll('[data-testid="employees-row"]');
        expect(rows[0].classes()).not.toContain('is-pending');
        expect(rows[1].classes()).toContain('is-pending');
    });
});
