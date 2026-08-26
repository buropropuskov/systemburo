/**
 * Приведение сырых строк снимка таблицы (payload.rows) к форме, которую ожидают
 * CarsTable/PeopleTable в preview-режиме.
 *
 * Снимок хранит DTO строк ровно так, как их отдаёт страница таблицы
 * (TableCarResponse/TableEmployeeResponse: organization, application_number, ...),
 * а компоненты рендерят обогащённую форму (organization_name, applicationNumber,
 * территориальный статус - состоянием кнопок въезда/выезда). Функция повторяет то
 * обогащение без сети и проецирует сохранённый territory_status на entry/exit -
 * так просмотр версии выглядит как основная страница таблицы.
 *
 * territory_status: 1=на территории, 2=выехал, 0/nil=не въезжал.
 */

/**
 * @param {*} status сохранённый territory_status строки снимка
 * @returns {{ entry_checked: boolean, exit_checked: boolean, territory_status: number }}
 */
function territoryChecks(status) {
  const ts = Number(status) || 0;
  // Взаимоисключающая проекция, как в CarsTable/PeopleTable/FactTable: 1=на
  // территории (Въезд), 2=выехал (Выезд). Для выехавшего Въезд НЕ отмечен -
  // иначе строка противоречит счётчику "Выехал" в шапке версии.
  return {
    territory_status: ts,
    entry_checked: ts === 1,
    exit_checked: ts === 2,
  };
}

function normalizeCarRow(r) {
  return {
    id: r.id,
    car_number: r.car_number || '',
    car_brand: r.car_brand || '',
    organization_id: r.organization_id ?? null,
    organization_name: r.organization || 'Не указана',
    company: r.company || null,
    company_id: r.company_id ?? null,
    unload_place: r.unload_place || '-',
    // Снимок не хранит id мест разгрузки (только человекочитаемую строку) -
    // formatUnloadPlaces упадёт на фолбэк item.unload_place.
    unload_place_ids: [],
    entry_date_to: r.entry_date_to || '',
    entry_time_from: r.entry_time_from || '',
    entry_time_to: r.entry_time_to || '',
    status: 'В работе',
    applicationId: r.application_id ?? null,
    applicationNumber: r.application_number || null,
    ...territoryChecks(r.territory_status),
  };
}

function normalizeEmployeeRow(r) {
  return {
    id: r.id,
    last_name: r.last_name || '',
    first_name: r.first_name || '',
    middle_name: r.middle_name || '',
    organization_id: r.organization_id ?? null,
    organization_name: r.organization || 'Не указана',
    company: r.company || null,
    company_id: r.company_id ?? null,
    position: r.position || null,
    citizenshipName: r.citizenship_name || null,
    pass_time: r.pass_time || '',
    pass_places: r.pass_places || '',
    entry_date_to: r.entry_date_to || '',
    status: 'Активен',
    applicationId: r.application_id ?? null,
    applicationNumber: r.application_number || null,
    ...territoryChecks(r.territory_status),
  };
}

/**
 * Нормализует строки снимка в preview-элементы CarsTable/PeopleTable.
 * @param {Array<object>|undefined} rows сырые строки payload.rows снимка
 * @param {'cars'|'people'} tableType тип таблицы снимка
 * @returns {Array<object>} элементы, готовые к передаче как :preview-items
 */
export function normalizeSnapshotRows(rows, tableType) {
  if (!Array.isArray(rows)) return [];
  const fn = tableType === 'people' ? normalizeEmployeeRow : normalizeCarRow;
  const seen = new Set();
  return rows.map((r, i) => {
    const item = fn(r || {});
    // preview-таблицы используют item.id как :key. Снимок может не иметь id или
    // содержать пересечение id между обычными и fact-строками - гарантируем
    // уникальность синтетическим ключом, чтобы не поймать конфликт :key.
    if (item.id === null || item.id === undefined || seen.has(item.id)) {
      item.id = -1 - i;
    }
    seen.add(item.id);
    return item;
  });
}
