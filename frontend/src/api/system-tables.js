import { apiRequest, apiRequestRaw } from './client';

export async function getSystemTables() {
  const res = await apiRequest('/system-tables');
  return res.json();
}

export async function getSystemTable(id) {
  const res = await apiRequest(`/system-tables/${id}`);
  return res.json();
}

export async function createSystemTable(data) {
  const res = await apiRequest('/system-tables', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateSystemTable(id, data) {
  const res = await apiRequest(`/system-tables/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteSystemTable(id) {
  return apiRequest(`/system-tables/${id}`, {
    method: 'DELETE',
  });
}

export async function uploadTablePhotos(tableId, formData) {
  const res = await apiRequest(`/system-tables/${tableId}/photos`, {
    method: 'POST',
    body: formData,
    headers: {},
  });
  return res.json();
}

export async function deleteTablePhoto(tableId, photoId) {
  return apiRequest(`/system-tables/${tableId}/photos/${photoId}`, {
    method: 'DELETE',
  });
}

export async function setMainTablePhoto(tableId, photoId) {
  return apiRequest(`/system-tables/${tableId}/photos/${photoId}/main`, {
    method: 'POST',
  });
}

/**
 * Версии (снимки) состояния таблицы (#980). Список - метаданные без payload
 * (дата, причина, автор, агрегаты); пагинация + опц. фильтр периода from/to.
 * Читаем сырой ответ (apiRequestRaw), т.к. total лежит в meta пагинации, а
 * wrapJsonUnwrap отдаёт только data.
 * @param {number} tableId
 * @param {{ page?: number, perPage?: number, from?: string, to?: string }} [opts]
 * @returns {Promise<{ items: Array, total: number }>}
 */
export async function listTableSnapshots(tableId, { page = 1, perPage = 20, from = '', to = '' } = {}) {
  const params = new URLSearchParams();
  params.set('page', String(page));
  params.set('per_page', String(perPage));
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await apiRequestRaw(`/system-tables/${tableId}/snapshots?${params.toString()}`);
  if (!res.ok) throw new Error(`Failed to list snapshots: ${res.status}`);
  const body = await res.json();
  const items = body && body.success ? (body.data || []) : (Array.isArray(body) ? body : []);
  const total = body && body.meta && typeof body.meta.total === 'number' ? body.meta.total : items.length;
  return { items, total };
}

/**
 * Полная версия (слепок) состояния таблицы: payload со строками+статусами.
 * @param {number} tableId
 * @param {number} snapshotId
 * @returns {Promise<{ id: number, table_id: number, taken_at: string, reason: string,
 *   actor_user_id?: number, payload: { table_type: string, rows: Array }, counts: object }>}
 */
export async function getTableSnapshot(tableId, snapshotId) {
  const res = await apiRequest(`/system-tables/${tableId}/snapshots/${snapshotId}`);
  return res.json();
}
