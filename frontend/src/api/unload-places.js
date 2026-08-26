import { apiRequest } from './client';

/**
 * unwrap бросает на !res.ok с сообщением бэка (fallback - если тела нет),
 * чтобы 4xx/5xx не прошёл молчаливым успехом (эталон api/approvers.js).
 */
async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/** Групповые операции над местами разгрузки (id-keyed). Возвращают BulkOpResult. */
export async function bulkArchiveUnloadPlaces(ids) {
  const res = await apiRequest('/unload-places/bulk/archive', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function bulkRestoreUnloadPlaces(ids) {
  const res = await apiRequest('/unload-places/bulk/restore', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

/**
 * Организации и компании, привязанные к месту разгрузки. Набор совпадает с тем,
 * что блокирует удаление места (гейт Delete считает по junction без фильтра
 * активности), поэтому архивные орг/компании тоже приходят (is_active=false).
 * @returns {Promise<{organizations: Array<{id, name, is_active}>, companies: Array<{id, name, is_active}>}>}
 */
export async function getUnloadPlaceUsage(id) {
  const res = await apiRequest(`/unload-places/${id}/usage`);
  return unwrap(res, 'Не удалось загрузить привязки места разгрузки');
}

/**
 * Снять все привязки организаций/компаний к месту разгрузки. Идемпотентно.
 * @returns {Promise<{organizations_detached: number, companies_detached: number}>}
 */
export async function detachAllUnloadPlace(id) {
  const res = await apiRequest(`/unload-places/${id}/detach-all`, { method: 'POST' });
  return unwrap(res, 'Не удалось отвязать место разгрузки');
}

/**
 * Снять привязку места разгрузки к ОДНОЙ организации. Идемпотентно (повтор по
 * уже снятой -> {detached:false}).
 * @returns {Promise<{detached: boolean}>}
 */
export async function detachOrganizationFromUnloadPlace(id, organizationId) {
  const res = await apiRequest(`/unload-places/${id}/organizations/${organizationId}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось отвязать организацию');
}

/**
 * Снять привязку места разгрузки к ОДНОЙ компании. Идемпотентно.
 * @returns {Promise<{detached: boolean}>}
 */
export async function detachCompanyFromUnloadPlace(id, companyId) {
  const res = await apiRequest(`/unload-places/${id}/companies/${companyId}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось отвязать компанию');
}
