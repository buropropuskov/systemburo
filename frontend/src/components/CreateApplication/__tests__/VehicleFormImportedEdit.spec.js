import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import VehicleForm from '../VehicleForm.vue';

// U3: строка машины, добавленная импортом бланка, приходит с formatId=null (сервер его
// туда не кладёт - подбор формата для импорта решает отдельный срез бэкенда, U2). Правка
// такой строки должна сама подобрать формат по номеру и разложить его на ячейки, а не
// оставлять поля правки пустыми под ранее выбранным дефолтным форматом.

const RU_FORMAT = {
    format: { id: 1, name: 'Российский', is_default: true },
    cells: [
        { cell_order: 1, cell_type: 'letters', min_length: 1, max_length: 1, allowed_letters: 'АВЕКМНОРСТУХ', alphabet_type: 'cyrillic' },
        { cell_order: 2, cell_type: 'numbers', min_length: 3, max_length: 3 },
        { cell_order: 3, cell_type: 'letters', min_length: 2, max_length: 2, allowed_letters: 'АВЕКМНОРСТУХ', alphabet_type: 'cyrillic' },
        { cell_order: 4, cell_type: 'numbers', min_length: 2, max_length: 3 },
    ],
};

vi.mock('@/api/client', () => ({
    apiRequest: vi.fn((url) => {
        if (url === '/license-plate-formats') {
            return Promise.resolve({ ok: true, json: async () => [RU_FORMAT] });
        }
        return Promise.resolve({ ok: true, json: async () => [] });
    }),
}));
vi.mock('@/api/blacklist', () => ({ checkVehicleBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify, enqueue: vi.fn() })) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));

const importedVehicle = (overrides = {}) => ({
    plateNumber: 'А123ВС777',
    mark: 'Toyota',
    markId: null,
    markName: 'Toyota',
    unloadingPlace: '',
    unloadPlaces: [1],
    passage_tables: [1],
    formatId: null,
    isExisting: false,
    ...overrides,
});

describe('VehicleForm - правка импортированной строки (U3)', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('показывает номер существующей машины в заблокированных ячейках вместо пустого прошлого ввода', async () => {
        const w = mount(VehicleForm, { attachTo: document.body });
        try {
            await flushPromises();
            await w.vm.editVehicle(importedVehicle({ isExisting: true, formatId: 1, plateNumber: 'А123ВС77' }));
            await flushPromises();
            const cells = w.findAll('.number__input');
            expect(cells.map(cell => cell.element.value).join('')).toBe('А123ВС77');
            expect(cells.every(cell => cell.element.disabled)).toBe(true);
            expect(w.get('.number-fact input').element.disabled).toBe(true);
        } finally { w.unmount(); }
    });

    it('не теряет исходный номер и привязку существующей машины с неизвестным форматом', async () => {
        const w = mount(VehicleForm, { props: { fieldConfig: {
            mark: { required: false }, unloading_places: { visible: false }, passage_tables: { visible: false },
        } }, attachTo: document.body });
        try {
            await flushPromises();
            await w.vm.editVehicle(importedVehicle({ id: 50, existingCarId: 73, isExisting: true, formatId: 999, plateNumber: 'ZZ-991' }));
            await flushPromises();
            expect(w.get('.number__input').element.value).toBe('ZZ-991');
            expect(w.get('.number__input').element.disabled).toBe(true);
            expect(notify).not.toHaveBeenCalled();
            w.vm.addVehicle();
            expect(w.emitted('vehicle-updated')?.[0]?.[0]).toMatchObject({
                id: 50, existingCarId: 73, isExisting: true, formatId: 999, plateNumber: 'ZZ-991',
            });
        } finally { w.unmount(); }
    });

    it('поздняя загрузка форматов не возвращает ранее выбранную машину после новой правки или отмены', async () => {
        const w = mount(VehicleForm, { attachTo: document.body });
        try {
            await flushPromises();
            let resolveFormats;
            w.vm.availableFormats = [];
            w.vm.formatsReady = new Promise(resolve => { resolveFormats = resolve; });
            const first = w.vm.editVehicle(importedVehicle({ isExisting: true, plateNumber: 'А123ВС77', mark: 'Первая' }));
            const second = w.vm.editVehicle(importedVehicle({ isExisting: true, plateNumber: 'В456АА77', mark: 'Вторая' }));
            expect(w.vm.canAddVehicle).toBe(false);
            w.vm.addVehicle();
            expect(w.emitted('vehicle-updated')).toBeUndefined();
            w.vm.openExistingCarsModal();
            expect(w.vm.showExistingCarsModal).toBe(false);
            await w.setData({ selectedExistingCars: [{ id: 73, number: 'А123ВС77' }] });
            expect(w.vm.canAddVehicle).toBe(false);
            w.vm.addVehicle();
            expect(w.emitted('vehicles-added')).toBeUndefined();
            await w.setData({ selectedExistingCars: [] });
            w.vm.availableFormats = [RU_FORMAT];
            resolveFormats();
            await Promise.all([first, second]);
            expect(w.vm.numberParts.join('')).toBe('В456АА77');
            expect(w.vm.selectedMark).toBe('Вторая');
            expect(w.vm.vehicleEditPending).toBe(false);
            w.vm.availableFormats = [];
            w.vm.formatsReady = new Promise(resolve => { resolveFormats = resolve; });
            const pending = w.vm.editVehicle(importedVehicle({ isExisting: true }));
            w.vm.cancelEdit();
            w.vm.availableFormats = [RU_FORMAT];
            resolveFormats();
            await pending;
            expect(w.vm.editingVehicle).toBeNull();
            expect(w.vm.vehicleEditPending).toBe(false);
            expect(w.vm.selectedMark).toBe('');
            expect(w.vm.numberParts.join('')).toBe('');
        } finally { w.unmount(); }
    });

    it('открытие импортированной строки на правку раскладывает номер по ячейкам формата', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        await flushPromises();

        w.vm.editVehicle(importedVehicle());
        await flushPromises();

        expect(w.vm.selectedFormat).toBeTruthy();
        expect(w.vm.selectedFormat.format.id).toBe(1);
        expect(w.vm.numberParts).toEqual(['А', '123', 'ВС', '777']);
    });

    it('сохранение правки не теряет и не искажает разложенный номер', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        await flushPromises();

        w.vm.editVehicle(importedVehicle({ plateNumber: 'А123ВС77', mark: 'Kia' }));
        await flushPromises();

        w.vm.addVehicle();
        await flushPromises();

        // Сохранение собирает строку из ячеек через join(' ') - тот же формат, что и у
        // машины, добавленной вручную (addVehicle); важно, что символы номера не потерялись
        // и не переставились местами, а не буквальное совпадение с исходной строкой без пробелов.
        const updates = w.emitted('vehicle-updated');
        expect(updates).toBeTruthy();
        expect(updates[0][0].plateNumber).toBe('А 123 ВС 77');
        expect(updates[0][0].plateNumber.replace(/\s+/g, '')).toBe('А123ВС77');
    });

    it('номер, не подошедший ни под один формат, не остаётся молча пустым', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        await flushPromises();

        w.vm.editVehicle(importedVehicle({ plateNumber: 'ZZZZZZZZ' }));
        await flushPromises();

        expect(w.vm.selectedFormat).toBeFalsy();
        expect(w.vm.numberParts).toEqual([]);
        expect(notify).toHaveBeenCalledWith(expect.objectContaining({
            prefix: expect.stringContaining('ZZZZZZZZ'),
            type: 'error',
        }));
    });

    // Клик по "редактировать" может опередить загрузку справочника форматов:
    // mounted асинхронный, и без ожидания подбор ложно не находил бы ничего.
    it('правка сразу после монтирования дожидается справочника форматов', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });

        await w.vm.editVehicle(importedVehicle({ plateNumber: 'А123ВС777' }));
        await flushPromises();

        expect(w.vm.selectedFormat).toBeTruthy();
        expect(w.vm.numberParts.join('')).not.toBe('');
        expect(notify).not.toHaveBeenCalled();
    });

    it('особый случай "По факту" не задет', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        await flushPromises();

        w.vm.editVehicle(importedVehicle({ plateNumber: 'По факту' }));
        await flushPromises();

        expect(w.vm.isNumberByFact).toBe(true);
        expect(notify).not.toHaveBeenCalled();
    });
});
