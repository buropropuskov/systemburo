import { apiRequest } from './client';

export async function getOrganizations() {
  const res = await apiRequest('/organizations');
  return res.json();
}

export async function createOrganization(data) {
  const res = await apiRequest('/organizations', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateOrganization(id, data) {
  const res = await apiRequest(`/organizations/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteOrganization(id) {
  return apiRequest(`/organizations/${id}`, { method: 'DELETE' });
}

/**
 * Пользователи, привязанные к организации через organization_id (участники,
 * дают user_count), в отличие от ответственных из /:id/users (#1046).
 * @param {number} id
 * @returns {Promise<Array>}
 */
export async function getOrganizationMembers(id) {
  const res = await apiRequest(`/organizations/${id}/members`);
  return res.json();
}

export async function getMyOrganization() {
  const res = await apiRequest('/get-organization');
  return res.json();
}

export async function getCompanies() {
  const res = await apiRequest('/companies');
  return res.json();
}

export async function createCompany(data) {
  const res = await apiRequest('/companies', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateCompany(id, data) {
  const res = await apiRequest(`/companies/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteCompany(id) {
  return apiRequest(`/companies/${id}`, { method: 'DELETE' });
}

/**
 * Пользователи, привязанные к компании через company_id (участники, дают
 * user_count), в отличие от ответственных из /:id/users (#1046).
 * @param {number} id
 * @returns {Promise<Array>}
 */
export async function getCompanyMembers(id) {
  const res = await apiRequest(`/companies/${id}/members`);
  return res.json();
}

// --- Блокеры архивации: перенос всех участников (#1379 delete-blockers) ---
// Список блокеров = активные участники, его даёт getOrganizationMembers/
// getCompanyMembers. Отдельного маршрута под блокеры больше нет: он отдавал ровно
// тот же ответ и держал собственный гейт, из-за чего право обходилось соседом
// (#2002). Здесь только действие переноса. unwrap бросает на !res.ok с сообщением
// бэка (эталон api/approvers.js), чтобы 4xx (target архивный/== источнику/404) не
// прошёл молчаливым успехом, а показался в notify.

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Перенести всех активных участников организации id в целевую targetId,
 * освобождая исходную для архивации. Идемпотентно (0 блокеров -> reassigned:0).
 * @param {number} id исходная организация
 * @param {number} targetId целевая организация
 * @returns {Promise<{reassigned: number}>}
 */
export async function reassignOrganizationUsers(id, targetId) {
  const res = await apiRequest(`/organizations/${id}/reassign-users`, {
    method: 'POST',
    body: JSON.stringify({ target_id: targetId }),
  });
  return unwrap(res, 'Не удалось перенести пользователей');
}

/**
 * Перенести всех активных участников компании id в целевую targetId.
 * @param {number} id исходная компания
 * @param {number} targetId целевая компания
 * @returns {Promise<{reassigned: number}>}
 */
export async function reassignCompanyUsers(id, targetId) {
  const res = await apiRequest(`/companies/${id}/reassign-users`, {
    method: 'POST',
    body: JSON.stringify({ target_id: targetId }),
  });
  return unwrap(res, 'Не удалось перенести пользователей');
}

// --- Групповые операции (bulk-ops, #1046) --------------------------------
// Обёртки для организаций и компаний живут в одном файле (как весь company-API
// выше). Все возвращают BulkOpResult { success_count, error_count, errors:[{id,
// name, error}] }, развёрнутый из envelope: статус 207 (частичный успех) бэк
// тоже отдаёт с success=true, поэтому wrapJsonUnwrap вернёт data, а не message.

/**
 * @typedef {Object} BulkItemError
 * @property {number} id
 * @property {string} name
 * @property {string} error
 */
/**
 * @typedef {Object} BulkOpResult
 * @property {number} success_count
 * @property {number} error_count
 * @property {BulkItemError[]} errors
 */

/**
 * POST /{entity}/bulk/{operation} с телом body.
 * @param {'organizations'|'companies'} entity
 * @param {string} operation
 * @param {Object} body
 * @returns {Promise<BulkOpResult>}
 */
async function bulkRequest(entity, operation, body) {
  const res = await apiRequest(`/${entity}/bulk/${operation}`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  return res.json();
}

/** Групповая смена типа организаций (type=null снимает тип). */
export function bulkUpdateOrganizationType(ids, type) {
  return bulkRequest('organizations', 'type', { ids, type });
}
/** Групповое назначение мест разгрузки организациям (mode=replace|add). */
export function bulkAssignOrganizationUnloadPlaces(ids, unloadPlaceIds, mode) {
  return bulkRequest('organizations', 'unload-places', { ids, unload_place_ids: unloadPlaceIds, mode });
}
/** Групповое назначение целевых таблиц организациям (mode=replace|add). */
export function bulkAssignOrganizationTables(ids, tableIds, mode) {
  return bulkRequest('organizations', 'tables', { ids, table_ids: tableIds, mode });
}
/** Групповое назначение ответственных организациям (mode=replace|add, primary не назначается). */
export function bulkAssignOrganizationUsers(ids, users, mode) {
  return bulkRequest('organizations', 'users', { ids, users, mode });
}
/** Групповое архивирование организаций. */
export function bulkArchiveOrganizations(ids) {
  return bulkRequest('organizations', 'archive', { ids });
}
/** Групповое восстановление организаций из архива. */
export function bulkRestoreOrganizations(ids) {
  return bulkRequest('organizations', 'restore', { ids });
}

/** Групповая смена типа компаний (type=null снимает тип). */
export function bulkUpdateCompanyType(ids, type) {
  return bulkRequest('companies', 'type', { ids, type });
}
/** Групповое назначение мест разгрузки компаниям (mode=replace|add). */
export function bulkAssignCompanyUnloadPlaces(ids, unloadPlaceIds, mode) {
  return bulkRequest('companies', 'unload-places', { ids, unload_place_ids: unloadPlaceIds, mode });
}
/** Групповое назначение целевых таблиц компаниям (mode=replace|add). */
export function bulkAssignCompanyTables(ids, tableIds, mode) {
  return bulkRequest('companies', 'tables', { ids, table_ids: tableIds, mode });
}
/** Групповое назначение ответственных компаниям (mode=replace|add, primary не назначается). */
export function bulkAssignCompanyUsers(ids, users, mode) {
  return bulkRequest('companies', 'users', { ids, users, mode });
}
/** Групповое архивирование компаний. */
export function bulkArchiveCompanies(ids) {
  return bulkRequest('companies', 'archive', { ids });
}
/** Групповое восстановление компаний из архива. */
export function bulkRestoreCompanies(ids) {
  return bulkRequest('companies', 'restore', { ids });
}
