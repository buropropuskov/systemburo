import { apiRequest } from './client';

/**
 * API клиент чёрного списка (#443) - машины и люди.
 * Методы возвращают unwrapped data (см. wrapJsonUnwrap в client.js).
 */

export async function listVehicleBlacklist({ includeArchived = false } = {}) {
  const qs = includeArchived ? '?include_archived=true' : '';
  const res = await apiRequest(`/vehicle-blacklist${qs}`);
  return res.json();
}

export async function listPersonBlacklist({ includeArchived = false } = {}) {
  const qs = includeArchived ? '?include_archived=true' : '';
  const res = await apiRequest(`/person-blacklist${qs}`);
  return res.json();
}

/**
 * Предпросмотр последствий внесения человека: где он сейчас фигурирует.
 * Ничего не меняет - только считает.
 * @returns {Promise<{matches: number, tables: string[], rows: Array<{label: string, organization?: string, tables: string[], applications: string[]}>}>}
 */
export async function personBlacklistImpact({ lastName, firstName, middleName = '' }) {
  const qs = new URLSearchParams({ last_name: lastName, first_name: firstName, middle_name: middleName });
  const res = await apiRequest(`/person-blacklist/impact?${qs}`);
  return res.json();
}

/** Предпросмотр последствий внесения машины: где она сейчас фигурирует. */
export async function vehicleBlacklistImpact({ carNumber, markId }) {
  const qs = new URLSearchParams({ car_number: carNumber, mark_id: String(markId) });
  const res = await apiRequest(`/vehicle-blacklist/impact?${qs}`);
  return res.json();
}

/**
 * Мутация с пробросом ошибки: apiRequest не кидает на 4xx (возвращает {message}),
 * поэтому проверяем res.ok сами - иначе 409/400 проглотились бы как успех.
 */
async function mutate(path, { method = 'POST', body } = {}) {
  const res = await apiRequest(path, { method, ...(body && { body: JSON.stringify(body) }) });
  const data = await res.json();
  if (!res.ok) {
    throw new Error((data && data.message) || `Ошибка запроса (${res.status})`);
  }
  return data;
}

export function createVehicleBlacklist({ car_number, mark_id, reason }) {
  return mutate('/vehicle-blacklist', { body: { car_number, mark_id, reason } });
}

export function updateVehicleBlacklist(id, { car_number, mark_id, reason }) {
  return mutate(`/vehicle-blacklist/${id}`, { method: 'PUT', body: { car_number, mark_id, reason } });
}

export function archiveVehicleBlacklist(id) {
  return mutate(`/vehicle-blacklist/${id}`, { method: 'DELETE' });
}

export function restoreVehicleBlacklist(id) {
  return mutate(`/vehicle-blacklist/${id}/restore`);
}

export function purgeVehicleBlacklist(id) {
  return mutate(`/vehicle-blacklist/${id}/purge`, { method: 'DELETE' });
}

/** Групповые операции над ЧС машин (id-keyed). Возвращают BulkOpResult. */
export async function bulkArchiveVehicleBlacklist(ids) {
  const res = await apiRequest('/vehicle-blacklist/bulk/archive', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function bulkRestoreVehicleBlacklist(ids) {
  const res = await apiRequest('/vehicle-blacklist/bulk/restore', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export function createPersonBlacklist({ last_name, first_name, middle_name, reason }) {
  return mutate('/person-blacklist', { body: { last_name, first_name, middle_name, reason } });
}

export function updatePersonBlacklist(id, { last_name, first_name, middle_name, reason }) {
  return mutate(`/person-blacklist/${id}`, { method: 'PUT', body: { last_name, first_name, middle_name, reason } });
}

export function archivePersonBlacklist(id) {
  return mutate(`/person-blacklist/${id}`, { method: 'DELETE' });
}

export function restorePersonBlacklist(id) {
  return mutate(`/person-blacklist/${id}/restore`);
}

export function purgePersonBlacklist(id) {
  return mutate(`/person-blacklist/${id}/purge`, { method: 'DELETE' });
}

/** Групповые операции над ЧС людей (id-keyed). Возвращают BulkOpResult. */
export async function bulkArchivePersonBlacklist(ids) {
  const res = await apiRequest('/person-blacklist/bulk/archive', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function bulkRestorePersonBlacklist(ids) {
  const res = await apiRequest('/person-blacklist/bulk/restore', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function getVehicleBlacklistHistory(id) {
  const res = await apiRequest(`/vehicle-blacklist/${id}/history`);
  return res.json();
}

export async function getPersonBlacklistHistory(id) {
  const res = await apiRequest(`/person-blacklist/${id}/history`);
  return res.json();
}

/** Весь журнал ЧС машин (все события всех записей, включая удалённые). */
export async function getAllVehicleBlacklistHistory() {
  const res = await apiRequest('/vehicle-blacklist/history');
  return res.json();
}

/** Весь журнал ЧС людей (все события всех записей, включая удалённые). */
export async function getAllPersonBlacklistHistory() {
  const res = await apiRequest('/person-blacklist/history');
  return res.json();
}

/**
 * Проверка машины в активном ЧС по номеру и марке (#443).
 * Возвращает { is_blacklisted, reason } - см. wrapJsonUnwrap.
 */
export async function checkVehicleBlacklist({ car_number, mark_id }) {
  const qs = new URLSearchParams({ car_number, mark_id: String(mark_id) });
  const res = await apiRequest(`/vehicle-blacklist/check?${qs.toString()}`);
  return res.json();
}

/**
 * Проверка человека в активном ЧС по ФИО (#443).
 * Совпадение строгое: фамилия + имя + отчество (пустое отчество матчит только пустое).
 */
export async function checkPersonBlacklist({ last_name, first_name, middle_name = '' }) {
  const qs = new URLSearchParams({ last_name, first_name, middle_name: middle_name || '' });
  const res = await apiRequest(`/person-blacklist/check?${qs.toString()}`);
  return res.json();
}
