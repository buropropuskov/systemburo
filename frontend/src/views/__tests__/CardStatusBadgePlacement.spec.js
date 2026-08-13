import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

/**
 * Бейдж статуса в карточке «Мои сотрудники» / «Мои автомобили» делит строку с соседним
 * полем (должность, формат номера). jsdom не считает ни каскад, ни медиазапросы, ни
 * высоту флекс-строки, поэтому раскладку стережём чтением SFC; сам эффект - замером в
 * браузере на 320/390.
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
    // Отцентрованный по вертикали бейдж при должности (названии формата) в две-три
    // строки повисал посреди высокой строки карточки - «воткнулся по середине».
    it('сотрудники: бейдж выровнен по первой строке пары', () => {
        expect(rule(employees, '.rt-table .employee-row.rt-row > .status-col'))
            .toMatch(/align-self:\s*flex-start/);
    });

    it('машины: бейдж выровнен по первой строке пары', () => {
        expect(rule(cars, '.rt-table .car-row.rt-row > .status-col'))
            .toMatch(/align-self:\s*flex-start/);
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
