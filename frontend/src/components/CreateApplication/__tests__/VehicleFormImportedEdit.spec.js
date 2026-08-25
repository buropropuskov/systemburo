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
