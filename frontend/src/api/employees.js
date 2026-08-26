import { apiRequest, apiRequestRaw } from './client';

export async function createEmployee(data) {
  const res = await apiRequest('/employees', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function getActiveEmployeesForTable(tableId) {
  const res = await apiRequest(`/employees/active-for-table/${tableId}`);
  return res.json();
}

/**
 * Ручное добавление сотрудников в таблицу без заявки (#1049, режим-1).
 * payload -> services.ManualEmployeeRequest (snake_case): organization_id, company_id,
 * table_id, entry_date_from/to, entry_time_from/to, employees[].
 * @param {object} payload
 * @returns {Promise<{success: boolean, message: string, attachment_id: number, employee_ids: number[]}>}
 */
export async function createManualEmployees(payload) {
  const res = await apiRequest('/employees/manual', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.message || 'Не удалось добавить сотрудников вручную');
  }
  return res.json();
}

/**
 * Поиск сотрудника в реестре по ФИО (для открытия карточки со страницы ЧС).
 * Возвращает запись unique_employee или null, если совпадения нет (404).
 */
export async function lookupUniqueEmployee({ last_name, first_name, middle_name = '' }) {
  const qs = new URLSearchParams({ last_name, first_name, middle_name: middle_name || '' });
  const res = await apiRequest(`/unique-employees/lookup?${qs.toString()}`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`Ошибка поиска сотрудника (${res.status})`);
  return res.json();
}

/**
 * Реестр сотрудников порциями (#1158, срез 3): передавая page/per_page, включает
 * серверную пагинацию (GetAllPaginated) и поиск search_query вместо legacy полного
 * списка. Пагинация лежит в envelope.meta рядом с data, а apiRequest снимает только
 * data и meta теряется - поэтому читаем сырой ответ через apiRequestRaw (см.
 * getUniqueCarsPaginated в api/cars.js).
 * @param {{filter_type?: string, search_query?: string, page?: number, per_page?: number}} params
 * @returns {Promise<{items: object[], meta: {total: number, page: number, per_page: number}}>}
 */
export async function getUniqueEmployeesPaginated(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequestRaw(`/unique-employees${query ? '?' + query : ''}`);
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось загрузить сотрудников');
  }
  return {
    items: body.data || [],
    meta: body.meta || { total: 0, page: 1, per_page: 30 },
  };
}

/**
 * Групповой перенос сотрудников между таблицами «Проход» (#1194): снимает
 * привязку к fromTableId и привязывает к каждой из toTableIds. Возвращает
 * BulkOpResult ({success_count, error_count, errors:[{id, name, error}]}),
 * развёрнутый из envelope - на 200/207 (успех/частичный успех) это data, на
 * структурную ошибку (400/403) - {message} (см. bulkRequest в api/organizations.js).
 * @param {number[]} ids
 * @param {number} fromTableId
 * @param {number[]} toTableIds
 */
export async function bulkMoveEmployeesTable(ids, fromTableId, toTableIds) {
  const res = await apiRequest('/employees/bulk/move-table', {
    method: 'POST',
    body: JSON.stringify({ ids, from_table_id: fromTableId, to_table_ids: toTableIds }),
  });
  return res.json();
}

/**
 * Групповое добавление сотрудников в таблицы «Проход» (#1194), не снимая
 * текущие привязки. Возвращает BulkOpResult, см. bulkMoveEmployeesTable.
 * @param {number[]} ids
 * @param {number[]} tableIds
 */
export async function bulkAddEmployeesTable(ids, tableIds) {
  const res = await apiRequest('/employees/bulk/add-table', {
    method: 'POST',
    body: JSON.stringify({ ids, table_ids: tableIds }),
  });
  return res.json();
}

/**
 * Групповое снятие сотрудников с ОДНОЙ таблицы «Проход» (#1194, срез S5):
 * используется и для bulk-«Убрать» (bulk-bar, ids - все выделенные), и для
 * per-row пункта «Убрать из этой таблицы» (ids с одним элементом,
 * TableRowRemoveMenu). Если привязка к tableId была последней у сотрудника -
 * BE сам деактивирует его (status=0), фронту не нужен отдельный вызов
 * деактивации. Возвращает BulkOpResult, см. bulkMoveEmployeesTable.
 * @param {number[]} ids
 * @param {number} tableId
 */
export async function bulkUnbindEmployeesTable(ids, tableId) {
  const res = await apiRequest('/employees/bulk/unbind-table', {
    method: 'POST',
    body: JSON.stringify({ ids, table_id: tableId }),
  });
  return res.json();
}
