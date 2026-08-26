/**
 * Сборка строк вложения под DTO бэкенда (#1685).
 *
 * Один и тот же набор полей уходит двумя путями: подачей заявки
 * (POST /applications/submit-complete-application) и дополнением уже поданной
 * (POST /applications/:id/supplements) - на сервере оба разбираются одними и теми же
 * VehicleInput / EmployeeInput / ItemInput. Формы хранят строки в camelCase, бэкенд ждёт
 * snake_case, и пока преобразование лежало в каждом флоу своей копией, они успели
 * разойтись на mark_name. Поле бэкенд игнорирует, поэтому обошлось, но следующее
 * разойдётся уже с потерей данных - молча, потому что лишний ключ в JSON никто не
 * проверяет. Поэтому маппинг здесь один на оба пути.
 */

/**
 * @param {Array<Object>} vehicles строки формы машин
 * @returns {Array<Object>} массив под services.VehicleInput
 */
export function toVehiclePayload(vehicles) {
    return (vehicles || []).map((vehicle) => ({
        car_number: vehicle.plateNumber,
        car_brand: vehicle.mark,
        mark_id: vehicle.markId || null,
        mark_name: vehicle.markName || vehicle.mark || null,
        unload_place: vehicle.unloadingPlace,
        unload_places: vehicle.unloadPlaces || [],
        passage_tables: vehicle.passage_tables || [],
        // Отметка о согласии субъекта на обработку персональных данных. У машин поле
        // шаблона выключено по умолчанию, поэтому обычно приходит false и сервер просто
        // не пишет отметку.
        pd_consent: vehicle.pdConsent === true,
    }));
}

/**
 * @param {Array<Object>} employees строки формы сотрудников
 * @returns {Array<Object>} массив под services.EmployeeInput
 */
export function toEmployeePayload(employees) {
    return (employees || []).map((employee) => ({
        last_name: employee.lastName,
        first_name: employee.firstName,
        middle_name: employee.middleName,
        citizenship_id: employee.citizenshipId,
        position: employee.position,
        passport_series_number: employee.passportSeriesNumber,
        patent_number: employee.patentNumber,
        other_permission: employee.otherPermission,
        target_tables: employee.targetTables || [],
        // Отметка о согласии субъекта на обработку персональных данных: дату и автора
        // ставит сервер, отсюда уходит только факт подтверждения. Сотрудник, выбранный
        // из реестра, приходит с pdConsent=true - согласие получено при заведении записи.
        pd_consent: employee.pdConsent === true,
    }));
}

/**
 * Порядок строк значим: order_index задаёт очерёдность позиций в бланке.
 *
 * @param {Array<Object>} items строки формы ТМЦ
 * @returns {Array<Object>} массив под services.ItemInput
 */
export function toItemPayload(items) {
    return (items || []).map((item, index) => ({
        name: item.itemName,
        count: item.quantity,
        order_index: index + 1,
    }));
}

/**
 * Собирает содержимое вложения по его типу.
 *
 * @param {string} attachmentType cars | people | items
 * @param {Array<Object>} rows строки формы
 * @returns {Object} объект с ключом vehicles / employees / items
 */
export function toAttachmentContent(attachmentType, rows) {
    switch (attachmentType) {
        case 'cars':
            return { vehicles: toVehiclePayload(rows) };
        case 'people':
            return { employees: toEmployeePayload(rows) };
        case 'items':
            return { items: toItemPayload(rows) };
        default:
            return {};
    }
}
