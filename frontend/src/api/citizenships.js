import { apiRequest } from './client';

/**
 * API клиент справочника гражданств (#415).
 *
 * apiRequest разворачивает envelope: на успехе res.json() отдаёт data,
 * на ошибке - { message }. res.ok проверяем явно и бросаем Error с
 * сообщением бэка, чтобы компонент показал реальную ошибку (например, 409
 * при попытке архивировать гражданство по умолчанию), а не считал её успехом.
 *
 * PUT - полная замена: бэкенд перезаписывает name/icon/is_default/patent_required
 * целиком (отсутствующие поля трактуются как false/null). Поэтому updateCitizenship
 * всегда шлёт все поля, включая icon, чтобы не затирать его.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

export async function listCitizenships({ includeArchived = false } = {}) {
  const qs = includeArchived ? '?include_archived=true' : '';
  const res = await apiRequest(`/citizenships${qs}`);
  return unwrap(res, 'Не удалось загрузить гражданства');
}

export async function createCitizenship({ name, isDefault = false, patentRequired = false }) {
  const res = await apiRequest('/citizenships', {
    method: 'POST',
    body: JSON.stringify({ name, is_default: isDefault, patent_required: patentRequired }),
  });
  return unwrap(res, 'Не удалось создать гражданство');
}

export async function updateCitizenship(id, { name, icon = null, isDefault = false, patentRequired = false }) {
  const res = await apiRequest(`/citizenships/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, icon, is_default: isDefault, patent_required: patentRequired }),
  });
  return unwrap(res, 'Не удалось сохранить гражданство');
}

export async function archiveCitizenship(id) {
  const res = await apiRequest(`/citizenships/${id}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось архивировать гражданство');
}

export async function restoreCitizenship(id) {
  const res = await apiRequest(`/citizenships/${id}/restore`, { method: 'POST' });
  return unwrap(res, 'Не удалось восстановить гражданство');
}

// История аудита: на успехе envelope разворачивается в массив записей, на 4xx
// unwrap бросит Error - иначе модалка молча показала бы "История пуста" вместо ошибки.
export async function getCitizenshipHistory(id) {
  const res = await apiRequest(`/citizenships/${id}/history`);
  return unwrap(res, 'Не удалось загрузить историю гражданства');
}

/**
 * Групповые операции над гражданствами (id-keyed). Возвращают BulkOpResult
 * НЕ через unwrap (не бросают на !ok) - частичный успех (207) тоже res.ok,
 * а полный провал (400 "Не выбраны гражданства") компонент разбирает сам по
 * форме {message} в handleBulkResult, не через try/catch.
 */
export async function bulkArchiveCitizenships(ids) {
  const res = await apiRequest('/citizenships/bulk/archive', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function bulkRestoreCitizenships(ids) {
  const res = await apiRequest('/citizenships/bulk/restore', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}
