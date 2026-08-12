/**
 * Счётчики сущностей заявки словами: «2 машины», «1 сотрудник», «5 позиций ТМЦ».
 *
 * Живёт отдельно от supplementStatuses.js: те же существительные нужны подвалу
 * списков подачи заявки («Всего 2 машины»), а состав раунда дополнения - лишь
 * один из потребителей.
 */

/** Существительные сущностей в трёх формах: 1 / 2-4 / 5+. */
export const COUNT_NOUNS = {
    vehicles: ['машина', 'машины', 'машин'],
    employees: ['сотрудник', 'сотрудника', 'сотрудников'],
    items: ['позиция ТМЦ', 'позиции ТМЦ', 'позиций ТМЦ'],
};

/**
 * Русское склонение по числу: 1 машина, 2-4 машины, 5+ машин (11-14 - третья форма).
 * @param {number} count
 * @param {string[]} forms три формы существительного
 * @returns {string}
 */
export function pluralRu(count, forms) {
    const mod10 = count % 10;
    const mod100 = count % 100;
    if (mod10 === 1 && mod100 !== 11) return forms[0];
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
    return forms[2];
}

/**
 * Число со склонённым существительным: «2 машины».
 * @param {number} count
 * @param {'vehicles'|'employees'|'items'} kind
 * @returns {string}
 */
export function entityCountLabel(count, kind) {
    const total = Number(count) || 0;
    const forms = COUNT_NOUNS[kind];
    return forms ? `${total} ${pluralRu(total, forms)}` : String(total);
}
