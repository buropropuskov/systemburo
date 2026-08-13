import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import VehiclesList from '../VehiclesList.vue';
import EmployeesList from '../EmployeesList.vue';

/**
 * Мобильная раскладка списков подачи заявки (волна 5 мокапа): кнопки шапки по
 * высоте бейджа, действия строки - подвалом карточки, итог блока - отдельной
 * строкой. Здесь проверяется, ЧТО и ГДЕ отрисовано; размеры и попадание пальцем -
 * замером в браузере.
 */

function mockMatchMedia(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

const VEHICLES = [
    { id: 1, plateNumber: 'О999ОО777', mark: 'ГАЗ', unloadingPlace: 'Дебаркадер №1' },
    { id: 2, plateNumber: 'К050УА902', mark: 'BMW X5' },
];

const EMPLOYEES = [
    { id: 1, lastName: 'Васьков', firstName: 'Денис', middleName: 'Александрович' },
    { id: 2, lastName: 'Кокотов', firstName: 'Андрей', middleName: 'Викторович' },
];

const PASS_TIME = { timeFrom: '08:00', timeTo: '18:00' };

let origMatchMedia;

beforeEach(() => {
    setActivePinia(createPinia());
    origMatchMedia = window.matchMedia;
});

afterEach(() => {
    window.matchMedia = origMatchMedia;
});

// isNarrow выставляется в onMounted - без nextTick рендер остаётся десктопным,
// и мобильные проверки прошли бы мимо.
const mountVehicles = async (narrow, props = {}) => {
    mockMatchMedia(narrow);
    const w = shallowMount(VehiclesList, { props: { vehicles: VEHICLES, ...props } });
    await w.vm.$nextTick();
    return w;
};

const mountEmployees = async (narrow, props = {}) => {
    mockMatchMedia(narrow);
    const w = shallowMount(EmployeesList, { props: { employees: EMPLOYEES, ...props } });
    await w.vm.$nextTick();
    return w;
};

describe('итог блока отдельной строкой', () => {
    it('на телефоне очистка уходит в подвал, в шапке её нет', async () => {
        const w = await mountVehicles(true);

        expect(w.find('.header-with-badge [data-testid="vehicles-clear-btn"]').exists()).toBe(false);
        expect(w.find('.list-foot [data-testid="vehicles-clear-btn"]').exists()).toBe(true);
        expect(w.get('[data-testid="vehicles-total"]').text()).toBe('Всего 2 машины');
    });

    it('на десктопе очистка остаётся в шапке, подвала нет', async () => {
        const w = await mountVehicles(false);

        expect(w.find('.header-with-badge [data-testid="vehicles-clear-btn"]').exists()).toBe(true);
        expect(w.find('.list-foot').exists()).toBe(false);
    });

    it('счётчик склоняется по числу строк', async () => {
        const one = await mountVehicles(true, { vehicles: VEHICLES.slice(0, 1) });
        expect(one.get('[data-testid="vehicles-total"]').text()).toBe('Всего 1 машина');

        const many = Array.from({ length: 5 }, (_, i) => ({ id: i + 1, plateNumber: `А00${i}АА777` }));
        const five = await mountVehicles(true, { vehicles: many });
        expect(five.get('[data-testid="vehicles-total"]').text()).toBe('Всего 5 машин');
    });

    it('список сотрудников считает своими словами', async () => {
        const narrow = await mountEmployees(true);
        expect(narrow.get('[data-testid="employees-total"]').text()).toBe('Всего 2 сотрудника');

        const wide = await mountEmployees(false);
        expect(wide.find('.list-foot').exists()).toBe(false);
        expect(wide.find('.header-with-badge [data-testid="employees-clear-btn"]').exists()).toBe(true);
    });

    it('пустой список подвала не заводит', async () => {
        expect((await mountVehicles(true, { vehicles: [] })).find('.list-foot').exists()).toBe(false);
        expect((await mountEmployees(true, { employees: [] })).find('.list-foot').exists()).toBe(false);
    });
});

describe('действия строки - бейджами с подписями', () => {
    it('карточка машины несёт три подписанных действия', async () => {
        const w = await mountVehicles(true);
        const actions = w.findAll('[data-testid="vehicles-row"] .actions-col')[0];

        expect(actions.text()).toContain('Изменить');
        expect(actions.text()).toContain('Удалить');
        expect(actions.text()).toContain('Детали');
    });

    it('кнопки карточки машины эмитят те же события, что и колоночная раскладка', async () => {
        const w = await mountVehicles(true);
        const row = w.findAll('[data-testid="vehicles-row"]')[0];

        await row.get('.edit-btn').trigger('click');
        await row.get('.delete-btn').trigger('click');

        expect(w.emitted('edit-vehicle')[0][0].plateNumber).toBe('О999ОО777');
        expect(w.emitted('delete-vehicle')).toEqual([[1]]);
    });

    it('у сотрудников подпись живёт рядом с иконкой - разметка одна на обе раскладки', async () => {
        const w = await mountEmployees(false);
        const labels = w.findAll('[data-testid="employees-row"] .act-label').map((el) => el.text());

        expect(labels.slice(0, 3)).toEqual(['Детали', 'Изменить', 'Удалить']);
    });
});

describe('вторая строка карточки машины', () => {
    it('показывает место разгрузки и часы пребывания', async () => {
        const w = await mountVehicles(true, { detailInfo: PASS_TIME });
        const meta = w.findAll('[data-testid="vehicles-row-meta"]');

        expect(meta[0].text()).toContain('Дебаркадер №1');
        expect(meta[0].text()).toContain('08:00—18:00');
    });

    it('без места и без времени строки нет вовсе', async () => {
        const w = await mountVehicles(true, { vehicles: [VEHICLES[1]] });

        expect(w.find('[data-testid="vehicles-row-meta"]').exists()).toBe(false);
    });

    it('время есть, места нет - место названо словами, а не пустотой', async () => {
        const w = await mountVehicles(true, { vehicles: [VEHICLES[1]], detailInfo: PASS_TIME });

        expect(w.get('[data-testid="vehicles-row-meta"]').text()).toContain('Место не выбрано');
    });
});

/*
 * Размеры jsdom не считает, поэтому договорённости мокапа сторожим чтением CSS:
 * кнопка шапки ростом с бейдж, зона нажатия 44px, и в карточке больше нет резерва
 * под кнопки, приколотые к правому краю строки.
 */
describe('замки мобильной геометрии', () => {
    const sources = {
        'VehiclesList.vue': readFileSync(resolve(__dirname, '../VehiclesList.vue'), 'utf8'),
        'EmployeesList.vue': readFileSync(resolve(__dirname, '../EmployeesList.vue'), 'utf8'),
        'ItemsList.vue': readFileSync(resolve(__dirname, '../ItemsList.vue'), 'utf8'),
    };

    it.each(['VehiclesList.vue', 'EmployeesList.vue'])(
        '%s: кнопка шапки 22px, зона нажатия добирается до 44px',
        (file) => {
            const rule = sources[file].match(/\.list-mini-btn\s*\{([\s\S]*?)\}/);
            expect(rule, 'нет правила .list-mini-btn').not.toBeNull();
            const height = Number(rule[1].match(/height:\s*(\d+)px/)[1]);
            expect(height).toBe(22);

            const before = sources[file].match(/\.list-mini-btn::before\s*\{([\s\S]*?)\}/);
            expect(before, 'зона нажатия не расширена').not.toBeNull();
            const outset = Number(before[1].match(/inset:\s*-(\d+)px/)[1]);
            expect(height + outset * 2).toBeGreaterThanOrEqual(44);
        },
    );

    it.each(Object.keys(sources))('%s: карточка не резервирует место под кнопки справа', (file) => {
        expect(sources[file]).not.toMatch(/padding:\s*10px\s+\d{2,}px\s+10px/);
    });

    it.each(Object.keys(sources))('%s: действия карточки не приколоты абсолютом', (file) => {
        const mobile = sources[file].slice(sources[file].indexOf('@media (max-width: 767.98px)'));
        const rule = mobile.match(/\.actions-col\s*\{([\s\S]*?)\}/);
        expect(rule, 'нет мобильного правила .actions-col').not.toBeNull();
        expect(rule[1]).toMatch(/position:\s*static/);
        expect(rule[1]).toMatch(/flex-basis:\s*100%/);
    });
});
