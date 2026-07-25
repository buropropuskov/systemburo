import { apiRequest, apiRequestRaw } from './client';
import { parseContentDispositionFilename } from '@/utils/download';

/**
 * unwrap бросает на !res.ok с сообщением бэка (fallback - если тела нет),
 * чтобы 4xx/5xx не прошёл молчаливым успехом (эталон api/approvers.js).
 */
async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

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

/** Групповые операции над системными таблицами (id-keyed). Возвращают BulkOpResult. */
export async function bulkArchiveSystemTables(ids) {
  const res = await apiRequest('/system-tables/bulk/archive', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function bulkRestoreSystemTables(ids) {
  const res = await apiRequest('/system-tables/bulk/restore', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

/**
 * Организации и компании, привязанные к системной таблице. Набор совпадает с тем,
 * что блокирует удаление таблицы (гейт Delete считает по junction без фильтра
 * активности), поэтому архивные орг/компании тоже приходят (is_active=false).
 * @returns {Promise<{organizations: Array<{id, name, is_active}>, companies: Array<{id, name, is_active}>}>}
 */
export async function getSystemTableUsage(id) {
  const res = await apiRequest(`/system-tables/${id}/usage`);
  return unwrap(res, 'Не удалось загрузить привязки таблицы');
}

/**
 * Снять все привязки организаций/компаний к системной таблице. Идемпотентно.
 * BE гейтит requireAdmin (page.admin); FE-кнопку гейтим тем же правом.
 * @returns {Promise<{organizations_detached: number, companies_detached: number}>}
 */
export async function detachAllSystemTable(id) {
  const res = await apiRequest(`/system-tables/${id}/detach-all`, { method: 'POST' });
  return unwrap(res, 'Не удалось отвязать таблицу');
}

/**
 * Снять привязку таблицы к ОДНОЙ организации. Идемпотентно (повтор по уже
 * снятой -> {detached:false}).
 * @returns {Promise<{detached: boolean}>}
 */
export async function detachOrganizationFromSystemTable(id, organizationId) {
  const res = await apiRequest(`/system-tables/${id}/organizations/${organizationId}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось отвязать организацию');
}

/**
 * Снять привязку таблицы к ОДНОЙ компании. Идемпотентно.
 * @returns {Promise<{detached: boolean}>}
 */
export async function detachCompanyFromSystemTable(id, companyId) {
  const res = await apiRequest(`/system-tables/${id}/companies/${companyId}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось отвязать компанию');
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
  // apiRequestRaw + res.ok, а НЕ apiRequest: wrapJsonUnwrap при success:false не
  // бросает, а отдаёт {message}, и вычищенный ретеншном снимок (404 между
  // загрузкой списка и кликом) молча стал бы "пустой версией" вместо ошибки.
  const res = await apiRequestRaw(`/system-tables/${tableId}/snapshots/${snapshotId}`);
  if (!res.ok) throw new Error(`Failed to load snapshot: ${res.status}`);
  const body = await res.json();
  return body && body.success ? body.data : body;
}

/**
 * Ручной снимок текущего состояния таблицы (#980 срез 6): создаёт новую версию
 * reason=manual. apiRequestRaw + res.ok, чтобы провал (400/404/5xx) бросал явно,
 * а не проглатывался в {message} (wrapJsonUnwrap на !success не кидает).
 * @param {number} tableId
 * @returns {Promise<{ id: number, message: string }>}
 */
export async function createTableSnapshot(tableId) {
  const res = await apiRequestRaw(`/system-tables/${tableId}/snapshots`, {
    method: 'POST',
    silent403: true,
  });
  if (!res.ok) throw new Error(`Failed to create snapshot: ${res.status}`);
  const body = await res.json();
  return body && body.success ? body.data : body;
}

/**
 * Выгрузка версии (или текущего состояния) таблицы файлом (#980 срез 6). Читаем
 * бинарный ответ через apiRequestRaw, имя берём из Content-Disposition (RFC 5987
 * filename* с кириллицей + ASCII-фолбэк), сохраняем через saveBlobAs.
 * @param {number} tableId
 * @param {number|'current'} snapshotId  ID версии либо 'current' для текущего состояния
 * @param {'xlsx'|'pdf'} [format]
 * @returns {Promise<{ blob: Blob, filename: string }>}
 */
export async function exportTableSnapshot(tableId, snapshotId, format = 'xlsx') {
  const res = await apiRequestRaw(
    `/system-tables/${tableId}/snapshots/${snapshotId}/export?format=${format}`,
    { silent403: true },
  );
  if (!res.ok) throw new Error(`Failed to export snapshot: ${res.status}`);
  const blob = await res.blob();
  const cd = res.headers.get('Content-Disposition') || '';
  const filename = parseContentDispositionFilename(cd, `snapshot.${format === 'pdf' ? 'pdf' : 'xlsx'}`);
  return { blob, filename };
}

/**
 * Чистка версий таблицы старше olderThanMonths месяцев (#980 срез 6).
 * Разрушительно - BE гейтит requireAdmin (page.admin); FE-кнопку гейтим тем же
 * правом. apiRequestRaw + res.ok для честного проброса ошибки.
 * @param {number} tableId
 * @param {number} olderThanMonths
 * @returns {Promise<{ deleted: number, message: string }>}
 */
export async function cleanupTableSnapshots(tableId, olderThanMonths) {
  const res = await apiRequestRaw(
    `/system-tables/${tableId}/snapshots?older_than=${olderThanMonths}`,
    { method: 'DELETE', silent403: true },
  );
  if (!res.ok) throw new Error(`Failed to cleanup snapshots: ${res.status}`);
  const body = await res.json();
  return body && body.success ? body.data : body;
}
