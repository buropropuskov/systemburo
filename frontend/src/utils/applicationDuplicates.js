/**
 * Правила совпадения людей и машин внутри одного вложения заявки.
 *
 * Единая точка для всех путей добавления: ручной ввод в форме, подтверждение выбора
 * существующих и блокировка уже добавленных строк в модалках выбора. Иначе критерии
 * расходятся и дубль проходит тем путём, где проверка мягче.
 */

const BY_FACT = 'по факту';

const normText = (value) => (value == null ? '' : String(value)).trim().toLowerCase().replace(/\s+/g, ' ');

// Паспорт и госномер набирают по-разному ("AB 123456", "ab123456") - сравниваем без пробелов.
const normCompact = (value) => normText(value).replace(/\s/g, '');

const fullNameKey = (employee) => {
    const parts = [employee.lastName, employee.firstName, employee.middleName].map(normText);
    return parts.some(Boolean) ? parts.join('|') : '';
};

/**
 * Один и тот же человек: сначала запись каталога, потом паспорт, потом ФИО.
 * ФИО - фолбэк на случай, когда шаблон вложения прячет поле паспорта; при пустых
 * и паспорте, и ФИО совпадения нет, чтобы не блокировать по двум пустым строкам.
 */
export function isSameEmployee(a, b) {
    if (!a || !b) return false;

    if (a.existingEmployeeId && b.existingEmployeeId && a.existingEmployeeId === b.existingEmployeeId) return true;

    const passportA = normCompact(a.passportSeriesNumber);
    const passportB = normCompact(b.passportSeriesNumber);
    if (passportA && passportB) return passportA === passportB;

    const nameA = fullNameKey(a);
    return !!nameA && nameA === fullNameKey(b);
}

/**
 * Одна и та же машина: запись каталога либо госномер. Номер "По факту" не опознаёт
 * конкретную машину, поэтому таких строк в заявке может быть несколько.
 */
export function isSameVehicle(a, b) {
    if (!a || !b) return false;

    if (a.existingCarId && b.existingCarId && a.existingCarId === b.existingCarId) return true;

    const plateA = normText(a.plateNumber);
    const plateB = normText(b.plateNumber);
    if (!plateA || !plateB || plateA === BY_FACT || plateB === BY_FACT) return false;

    return normCompact(plateA) === normCompact(plateB);
}

/** Строка списка, совпадающая с кандидатом, или null. excludeId пропускает саму себя при правке. */
export function findDuplicateEmployee(employees, candidate, excludeId = null) {
    return (employees || []).find(
        (employee) => (excludeId == null || employee.id !== excludeId) && isSameEmployee(employee, candidate),
    ) || null;
}

/** Строка списка, совпадающая с кандидатом, или null. excludeId пропускает саму себя при правке. */
export function findDuplicateVehicle(vehicles, candidate, excludeId = null) {
    return (vehicles || []).find(
        (vehicle) => (excludeId == null || vehicle.id !== excludeId) && isSameVehicle(vehicle, candidate),
    ) || null;
}

/** Первая строка списка, повторяющая любую из предыдущих, или null. */
export function findFirstDuplicate(list, isSame) {
    const seen = [];
    for (const item of list || []) {
        if (seen.some(previous => isSame(previous, item))) return item;
        seen.push(item);
    }
    return null;
}

/** Сотрудник из /unique-employees в форму сравнения. */
export function employeeFromCatalog(raw) {
    return {
        existingEmployeeId: raw.id,
        lastName: raw.last_name,
        firstName: raw.first_name,
        middleName: raw.middle_name,
        passportSeriesNumber: raw.passport_series_number,
    };
}

/** Машина из /unique-cars в форму сравнения. */
export function vehicleFromCatalog(raw) {
    return {
        existingCarId: raw.id,
        plateNumber: raw.number,
        mark: raw.mark,
    };
}

/** Подпись строки для уведомления о дубле. */
export function employeeLabel(employee) {
    const name = [employee.lastName, employee.firstName, employee.middleName]
        .map((part) => (part || '').trim())
        .filter(Boolean)
        .join(' ');
    return name || (employee.passportSeriesNumber || '').trim() || 'Сотрудник';
}

/** Подпись строки для уведомления о дубле. */
export function vehicleLabel(vehicle) {
    const plate = (vehicle.plateNumber || '').trim();
    const mark = (vehicle.mark || '').trim();
    return [mark, plate].filter(Boolean).join(' ') || 'Машина';
}
