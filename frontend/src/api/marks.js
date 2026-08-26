import { apiRequest } from './client';

/**
 * API клиент справочника марок автомобилей (#185).
 * Все методы возвращают unwrapped data (см. apiRequest.wrapJsonUnwrap в client.js).
 */

export async function listMarks({ includeArchived = false } = {}) {
  const qs = includeArchived ? '?include_archived=true' : '';
  const res = await apiRequest(`/marks${qs}`);
  return res.json();
}

export async function createMark({ name }) {
  const res = await apiRequest('/marks', {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function renameMark(id, { name }) {
  const res = await apiRequest(`/marks/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function archiveMark(id) {
  const res = await apiRequest(`/marks/${id}/archive`, { method: 'POST' });
  return res.json();
}

export async function restoreMark(id) {
  const res = await apiRequest(`/marks/${id}/restore`, { method: 'POST' });
  return res.json();
}

export async function getMarkHistory(id) {
  const res = await apiRequest(`/marks/${id}/history`);
  return res.json();
}

/** Групповые операции над марками (id-keyed). Возвращают BulkOpResult. */
export async function bulkArchiveMarks(ids) {
  const res = await apiRequest('/marks/bulk/archive', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function bulkRestoreMarks(ids) {
  const res = await apiRequest('/marks/bulk/restore', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}
