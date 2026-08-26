import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

/**
 * Бейдж статуса в карточке «Мои сотрудники» / «Мои автомобили» - в подвале карточки,
 * одной строкой с кнопками «Изменить»/«Удалить» (разбор второго круга замечаний
 * владельца, #1097 w8). Раньше бейдж делил строку с соседним полем (должность, формат
 * номера) и выравнивался по её верхнему краю через align-self: flex-start - то поле
 * несло пунктирную границу сверху, а бейдж нет, но оба совпадали по одной Y-координате,
 * поэтому бейдж вставал прямо на чужую границу («стоит поперёк»). jsdom не считает ни
 * каскад, ни медиазапросы, ни высоту флекс-строки, поэтому раскладку стережём чтением
 * SFC; сам эффект - замером в браузере на 320/390.
 */

const VIEWS = resolve(__dirname, '..');
const employees = readFileSync(resolve(VIEWS, 'EmployeeView.vue'), 'utf8');
const cars = readFileSync(resolve(VIEWS, 'CarsView.vue'), 'utf8');
const statusBadge = readFileSync(
    resolve(VIEWS, '../components/ui/StatusBadge.vue'),
    'utf8'
);

/** Тело правила для селектора, без учёта переносов. */
function rule(src, selector) {
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
    return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

describe('бейдж статуса в карточке не наезжает на соседнее поле', () => {
    // Бейдж и колонка действий несут ОДИНАКОВЫЙ отступ и сплошную границу подвала -
    // совпадать по чужому пунктиру им уже нечему (это и было причиной наложения).
    //
    // align-self: flex-start (третий круг замечаний, #1097 w9): бейдж и кнопки -
    // соседние ячейки одной строки подвала, их высоты по контенту не совпадают ровно,
    // и align-self: center центрировал каждую ячейку независимо - верхние края (а с
    // ними border-top) расходились, серая линия подвала переламывалась после бейджа.
    // flex-start прижимает обе ячейки к верхнему краю строки - border-top гарантированно
    // на одной Y.
    it('сотрудники: бейдж стоит в подвале карточки, а не на пунктире соседнего поля', () => {
        const statusRule = rule(employees, '.rt-table .employee-row.rt-row > .status-col');
        expect(statusRule).toMatch(/order:\s*10/);
        expect(statusRule).toMatch(/border-top:\s*1px solid/);
        expect(statusRule).toMatch(/align-self:\s*flex-start/);
    });

    it('машины: бейдж стоит в подвале карточки, а не на пунктире соседнего поля', () => {
        const statusRule = rule(cars, '.rt-table .car-row.rt-row > .status-col');
        expect(statusRule).toMatch(/order:\s*10/);
        expect(statusRule).toMatch(/border-top:\s*1px solid/);
        expect(statusRule).toMatch(/align-self:\s*flex-start/);
    });

    // height: 100% у ячейки десктопной таблицы в карточке смысла не имеет, а процент
    // от высоты карточки развернул бы поле поверх соседних строк.
    it('поле карточки не тянет высоту от родителя', () => {
        expect(rule(employees, '.employee-row.rt-row > .employee-col')).toMatch(/height:\s*auto/);
        expect(rule(cars, '.car-row.rt-row > .car-col')).toMatch(/height:\s*auto/);
    });

    // 120px нужны, чтобы пилюли выстроились в столбец таблицы; в карточке столбца нет,
    // и этот минимум забирал половину строки шириной 320px у соседнего поля.
    it('на узком экране пилюля живёт по содержимому, без минимума под столбец', () => {
        expect(statusBadge).toMatch(
            /@media \(max-width: 767\.98px\)\s*\{\s*\.status-badge\s*\{[^}]*min-width:\s*0/
        );
    });
});
