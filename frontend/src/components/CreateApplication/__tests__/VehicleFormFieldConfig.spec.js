import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import VehicleForm from '../VehicleForm.vue';

// H-7 (#529): VehicleForm уважает field-config выбранного шаблона.
// Ключи реестра: number / mark / unloading_places.
// Чекбоксы "по факту" - под-контрол поля, скрываются вместе с ним.

vi.mock('@/api/client', () => ({
    apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [] }),
}));
vi.mock('@/api/blacklist', () => ({
    checkVehicleBlacklist: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/stores/auth', () => ({
    useAuthStore: vi.fn(() => ({ token: 'test-token' })),
}));
vi.mock('@/api/marks', () => ({
    listMarks: vi.fn().mockResolvedValue([]),
}));

const mountForm = (fieldConfig = {}) =>
    mount(VehicleForm, {
        props: { fieldConfig },
        attachTo: document.body,
    });

describe('VehicleForm - потребление field-config (#529)', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('без конфига: fieldVisible/fieldRequired возвращают true по умолчанию', () => {
        const w = mountForm();
        expect(w.vm.fieldVisible('number')).toBe(true);
        expect(w.vm.fieldRequired('number')).toBe(true);
        expect(w.vm.fieldVisible('mark')).toBe(true);
        expect(w.vm.fieldRequired('mark')).toBe(true);
        expect(w.vm.fieldVisible('unloading_places')).toBe(true);
        expect(w.vm.fieldRequired('unloading_places')).toBe(true);
    });

    it('скрывает блок "Номер Т/С" при number.visible=false', () => {
        const w = mountForm({ number: { visible: false, required: true } });
        expect(w.find('.completion__number').exists()).toBe(false);
    });

    it('скрывает блок "Марка Т/С" при mark.visible=false', () => {
        const w = mountForm({ mark: { visible: false, required: true } });
        expect(w.find('.completion__mark').exists()).toBe(false);
    });

    it('скрывает блок "Места разгрузки" при unloading_places.visible=false', () => {
        const w = mountForm({ unloading_places: { visible: false, required: true } });
        expect(w.find('.completion__unloading').exists()).toBe(false);
    });

    it('при number.visible=true блок номера виден', () => {
        const w = mountForm({ number: { visible: true, required: true } });
        expect(w.find('.completion__number').exists()).toBe(true);
    });

    it('снимает звёздочку номера при number.required=false', () => {
        const w = mountForm({ number: { visible: true, required: false } });
        const numberBlock = w.find('.completion__number');
        expect(numberBlock.exists()).toBe(true);
        // Звёздочка внутри блока номера отсутствует
        expect(numberBlock.find('.required').exists()).toBe(false);
    });

    it('снимает звёздочку марки при mark.required=false', () => {
        const w = mountForm({ mark: { visible: true, required: false } });
        const markBlock = w.find('.completion__mark');
        expect(markBlock.exists()).toBe(true);
        expect(markBlock.find('.required').exists()).toBe(false);
    });

    it('снимает звёздочку мест разгрузки при unloading_places.required=false', () => {
        const w = mountForm({ unloading_places: { visible: true, required: false } });
        const unloadBlock = w.find('.completion__unloading');
        expect(unloadBlock.exists()).toBe(true);
        expect(unloadBlock.find('.required').exists()).toBe(false);
    });

    it('при скрытом number блок "по факту" тоже скрыт (вместе с полем)', () => {
        const w = mountForm({ number: { visible: false, required: true } });
        // .number-fact находится внутри .completion__number - весь блок скрыт
        expect(w.find('.number-fact').exists()).toBe(false);
    });

    it('при скрытой mark блок "по факту" марки тоже скрыт', () => {
        const w = mountForm({ mark: { visible: false, required: true } });
        expect(w.find('.mark-fact').exists()).toBe(false);
    });
});
