import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { apiRequest } from '@/api/client';
import VehicleForm from '../VehicleForm.vue';

// Блок "Проезд" в форме машины (#1036) - копия блока "Места прохода" из EmployeeForm,
// но фильтрует таблицы table_type=cars и пишет выбор в поле passage_tables.

const SYSTEM_TABLES = [
    { id: 10, name: 'passage-cars', display_name: 'Проезд для машин', table_type: 'cars', status: 'active' },
    { id: 11, name: 'passage-people', display_name: 'Проход для людей', table_type: 'people', status: 'active' },
];

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

const defaultApiRequest = (url) => {
    if (url === '/system-tables') {
        return Promise.resolve({ ok: true, json: async () => SYSTEM_TABLES });
    }
    return Promise.resolve({ ok: true, json: async () => [] });
};

vi.mock('@/api/client', () => ({
    apiRequest: vi.fn((url) => defaultApiRequest(url)),
}));
vi.mock('@/api/blacklist', () => ({ checkVehicleBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));

describe('VehicleForm - блок "Проезд" (#1036)', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        apiRequest.mockImplementation(defaultApiRequest);
    });

    it('рендерит только таблицы table_type=cars, скрывая people', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        await flushPromises();

        const items = w.findAll('.passage__item');
        expect(items).toHaveLength(1);
        expect(items[0].text()).toBe('Проезд для машин');
    });

    it('клик по таблице переключает её в selectedPassageTables', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        await flushPromises();

        expect(w.vm.selectedPassageTables).toEqual([]);
        await w.find('.passage__item').trigger('click');
        expect(w.vm.selectedPassageTables).toEqual([10]);
        await w.find('.passage__item').trigger('click');
        expect(w.vm.selectedPassageTables).toEqual([]);
    });

    it('выбранные таблицы попадают в payload машины как passage_tables', async () => {
        const w = mount(VehicleForm, {
            props: {
                fieldConfig: {
                    number: { visible: true, required: false },
                    mark: { visible: true, required: false },
                    unloading_places: { visible: true, required: false },
                },
            },
            attachTo: document.body,
        });
        await flushPromises();

        await w.find('.passage__item').trigger('click');
        expect(w.vm.selectedPassageTables).toEqual([10]);

        w.vm.addVehicle();
        await flushPromises();

        const emitted = w.emitted('vehicle-added');
        expect(emitted).toBeTruthy();
        expect(emitted[0][0].passage_tables).toEqual([10]);
    });

    it('НЕ автовыбирает места проезда по организации и НЕ показывает уведомление (#1036)', async () => {
        apiRequest.mockImplementation((url) => {
            if (url === '/system-tables') return Promise.resolve({ ok: true, json: async () => SYSTEM_TABLES });
            if (url === '/organizations/7/tables') return Promise.resolve({
                ok: true,
                json: async () => [
                    { id: 10, name: 'passage-cars', display_name: 'Проезд для машин', table_type: 'cars', status: 'active' },
                ],
            });
            return Promise.resolve({ ok: true, json: async () => [] });
        });

        const w = mount(VehicleForm, {
            props: { userOrganizationId: 7, userOrganization: 'ООО Ромашка' },
            attachTo: document.body,
        });
        await flushPromises();

        // Таблица привязана к организации, но НЕ выбрана автоматически - пользователь выбирает сам.
        expect(w.vm.selectedPassageTables).toEqual([]);
        expect(notifyMock).not.toHaveBeenCalledWith(
            expect.objectContaining({ prefix: expect.stringContaining('автоматически') }),
        );
    });

    it('кнопка "Добавить" существующих не активна без обязательного места проезда', async () => {
        const w = mount(VehicleForm, {
            props: {
                fieldConfig: {
                    unloading_places: { visible: false },
                    passage_tables: { visible: true, required: true },
                },
            },
            attachTo: document.body,
        });
        await flushPromises();

        w.vm.selectedExistingCars = [{ id: 1, number: 'A001', mark: 'Kamaz' }];
        await flushPromises();

        // Проезд обязателен и не выбран - кнопка заблокирована, как и для новой машины.
        expect(w.vm.canAddExistingCars).toBe(false);

        await w.find('.passage__item').trigger('click');
        expect(w.vm.selectedPassageTables).toEqual([10]);
        expect(w.vm.canAddExistingCars).toBe(true);
    });

    it('addExistingCars без обязательного проезда - уведомление, без эмита', async () => {
        const w = mount(VehicleForm, {
            props: {
                fieldConfig: {
                    unloading_places: { visible: false },
                    passage_tables: { visible: true, required: true },
                },
            },
            attachTo: document.body,
        });
        await flushPromises();

        w.vm.selectedUnloadingPlaces = [5]; // пройти первый guard мест разгрузки
        w.vm.selectedExistingCars = [{ id: 1, number: 'A001', mark: 'Kamaz' }];
        w.vm.addExistingCars();
        await flushPromises();

        expect(notifyMock).toHaveBeenCalledWith(
            expect.objectContaining({ bold: expect.stringContaining('проезд'), type: 'error' }),
        );
        expect(w.emitted('vehicles-added')).toBeFalsy();
    });

    it('НЕ автовыбирает места проезда по компании и НЕ показывает уведомление (#1036)', async () => {
        apiRequest.mockImplementation((url) => {
            if (url === '/system-tables') return Promise.resolve({ ok: true, json: async () => SYSTEM_TABLES });
            if (url === '/companies/7/tables') return Promise.resolve({
                ok: true,
                json: async () => [
                    { id: 10, name: 'passage-cars', display_name: 'Проезд для машин', table_type: 'cars', status: 'active' },
                ],
            });
            return Promise.resolve({ ok: true, json: async () => [] });
        });

        const w = mount(VehicleForm, {
            props: { userCompanyId: 7, userCompany: 'ООО Компания' },
            attachTo: document.body,
        });
        await flushPromises();

        expect(w.vm.selectedPassageTables).toEqual([]);
        expect(notifyMock).not.toHaveBeenCalledWith(
            expect.objectContaining({ prefix: expect.stringContaining('автоматически') }),
        );
    });
});
