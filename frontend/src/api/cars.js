import { apiRequest, apiRequestRaw } from './client';

export async function getActiveCarsForTable(tableId) {
  const res = await apiRequest(`/cars/active-for-table/${tableId}`);
  return res.json();
}

/**
 * Ручное добавление машин в таблицу без заявки (#1049, режим-1).
 * payload -> services.ManualCarRequest (snake_case): organization_id, company_id,
 * table_id, entry_date_from/to, entry_time_from/to, roof_access, free_parking, vehicles[].
 * @param {object} payload
 * @returns {Promise<{success: boolean, message: string, attachment_id: number, car_ids: number[]}>}
 */
export async function createManualCars(payload) {
  const res = await apiRequest('/cars/manual', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.message || 'Не удалось добавить машины вручную');
  }
  return res.json();
}

export async function getFactCarsForTable(tableId) {
  const res = await apiRequest(`/cars/fact-for-table/${tableId}`);
  return res.json();
}

export async function getCarHistory(id) {
  const res = await apiRequest(`/cars/${id}/history`);
  return res.json();
}

export async function addCarHistoryEntry(id, data) {
  const res = await apiRequest(`/cars/${id}/history`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateTerritoryStatus(id, data) {
  const res = await apiRequest(`/cars/${id}/territory-status`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deactivateCar(id) {
  const res = await apiRequest(`/cars/${id}/deactivate`, { method: 'PUT' });
  return res.json();
}

export async function activateCar(id) {
  const res = await apiRequest(`/cars/${id}/activate`, { method: 'PUT' });
  return res.json();
}

export async function getCarsCurrentStatus() {
  const res = await apiRequest('/cars/history/current-status');
  return res.json();
}

/**
 * Поиск машины в реестре по номеру и марке (для открытия карточки со страницы ЧС).
 * Возвращает запись unique_car или null, если совпадения нет (404).
 */
export async function lookupUniqueCar({ number, mark }) {
  const qs = new URLSearchParams({ number, mark: mark || '' });
  const res = await apiRequest(`/unique-cars/lookup?${qs.toString()}`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`Ошибка поиска машины (${res.status})`);
  return res.json();
}

/**
 * Реестр машин порциями (#1158, срез 2): передавая page/per_page, включает
 * серверную пагинацию (GetAllPaginated) и поиск search_query вместо legacy
 * полного списка. Пагинация лежит в envelope.meta рядом с data, а apiRequest
 * снимает только data и meta теряется - поэтому читаем сырой ответ через
 * apiRequestRaw (см. getApplicationsPaginated в api/applications.js).
 * @param {{filter_type?: string, search_query?: string, page?: number, per_page?: number}} params
 * @returns {Promise<{items: object[], meta: {total: number, page: number, per_page: number}}>}
 */
export async function getUniqueCarsPaginated(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequestRaw(`/unique-cars${query ? '?' + query : ''}`);
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось загрузить машины');
  }
  return {
    items: body.data || [],
    meta: body.meta || { total: 0, page: 1, per_page: 30 },
  };
}

/**
 * Групповой перенос машин между таблицами «Проезд» (#1194): снимает привязку к
 * fromTableId и привязывает к каждой из toTableIds. Возвращает BulkOpResult
 * ({success_count, error_count, errors:[{id, name, error}]}), развёрнутый из
 * envelope - на 200/207 (успех/частичный успех) это data, на структурную
 * ошибку (400/403) - {message} (см. bulkRequest в api/organizations.js).
 * @param {number[]} ids
 * @param {number} fromTableId
 * @param {number[]} toTableIds
 */
export async function bulkMoveCarsTable(ids, fromTableId, toTableIds) {
  const res = await apiRequest('/cars/bulk/move-table', {
    method: 'POST',
    body: JSON.stringify({ ids, from_table_id: fromTableId, to_table_ids: toTableIds }),
  });
  return res.json();
}

/**
 * Групповое добавление машин в таблицы «Проезд» (#1194), не снимая текущие
 * привязки. Возвращает BulkOpResult, см. bulkMoveCarsTable.
 * @param {number[]} ids
 * @param {number[]} tableIds
 */
export async function bulkAddCarsTable(ids, tableIds) {
  const res = await apiRequest('/cars/bulk/add-table', {
    method: 'POST',
    body: JSON.stringify({ ids, table_ids: tableIds }),
  });
  return res.json();
}

/**
 * Групповое снятие машин с ОДНОЙ таблицы «Проезд» (#1194, срез S5): используется
 * и для bulk-«Убрать» (bulk-bar, ids - все выделенные), и для per-row пункта
 * «Убрать из этой таблицы» (ids с одним элементом, TableRowRemoveMenu). Если
 * привязка к tableId была последней у машины - BE сам деактивирует её (status=0),
 * фронту не нужен отдельный вызов деактивации. Возвращает BulkOpResult, см.
 * bulkMoveCarsTable.
 * @param {number[]} ids
 * @param {number} tableId
 */
export async function bulkUnbindCarsTable(ids, tableId) {
  const res = await apiRequest('/cars/bulk/unbind-table', {
    method: 'POST',
    body: JSON.stringify({ ids, table_id: tableId }),
  });
  return res.json();
}
