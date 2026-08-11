import { apiRequest } from './client';

export async function getUsers() {
  const res = await apiRequest('/users/all');
  return res.json();
}

// Админ создаёт пользователя. /register не экспонируется, потому что
// публичная регистрация не предусмотрена — юзеров заводит только админ
// через защищённый POST /users.
export async function createUser(data) {
  const res = await apiRequest('/users', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateUserType(username, typeId) {
  const res = await apiRequest(`/users/${username}/type`, {
    method: 'PUT',
    body: JSON.stringify({ type_id: typeId }),
  });
  return res.json();
}

export async function updateUserPassword(username, password) {
  const res = await apiRequest(`/users/${username}/password`, {
    method: 'PUT',
    body: JSON.stringify({ password }),
  });
  return res.json();
}

/**
 * Смена СВОЕГО пароля. Учётка берётся из маркера доступа, имени в запросе нет.
 * Возвращает сырой Response: вызывающему нужен текст ошибки сервера
 * (неверный текущий пароль, нарушение политики, превышение числа попыток).
 * @param {string} currentPassword
 * @param {string} newPassword
 * @returns {Promise<Response>}
 */
export async function changeOwnPassword(currentPassword, newPassword) {
  return apiRequest('/users/me/password', {
    method: 'PUT',
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
}

export async function updateUserInfo(username, data) {
  const res = await apiRequest(`/users/${username}/info`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateUserOrganization(username, organizationId) {
  const res = await apiRequest(`/users/${username}/organization`, {
    method: 'PUT',
    body: JSON.stringify({ organization_id: organizationId }),
  });
  return res.json();
}

export async function updateUserCompany(username, companyId) {
  const res = await apiRequest(`/users/${username}/company`, {
    method: 'PUT',
    body: JSON.stringify({ company_id: companyId }),
  });
  return res.json();
}

export async function deleteUser(username) {
  return apiRequest(`/users/${username}`, { method: 'DELETE' });
}

// --- Места доступа охранника (#706, BE-S5) ---

export async function getUserUnloadPlaces(username) {
  const res = await apiRequest(`/users/${username}/unload-places`);
  return res.json();
}

export async function setUserUnloadPlaces(username, unloadPlaceIds) {
  return apiRequest(`/users/${username}/unload-places`, {
    method: 'PUT',
    body: JSON.stringify({ unload_place_ids: unloadPlaceIds }),
  });
}

export async function getUserTables(username) {
  const res = await apiRequest(`/users/${username}/tables`);
  return res.json();
}

export async function setUserTables(username, tableIds) {
  return apiRequest(`/users/${username}/tables`, {
    method: 'PUT',
    body: JSON.stringify({ table_ids: tableIds }),
  });
}

// --- История входов (auth_events, #1076) ---

/**
 * Постранично читает историю входов пользователя.
 * @param {string} username
 * @param {{page?: number, limit?: number, category?: string, from?: string, to?: string}} [params]
 * @returns {Promise<{items: Array, total: number, page: number, limit: number}>}
 */
export async function getUserAuthEvents(username, params = {}) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== '' && value !== null && value !== undefined) qs.append(key, value);
  });
  const suffix = qs.toString() ? `?${qs.toString()}` : '';
  const res = await apiRequest(`/users/${username}/auth-events${suffix}`);
  return res.json();
}

/**
 * Групповые операции над пользователями (username-keyed). Возвращают BulkOpResult
 * ({ success_count, error_count, errors }) при 200/207 либо { message } при 4xx.
 */
export async function bulkArchiveUsers(usernames) {
  const res = await apiRequest('/users/bulk/archive', { method: 'POST', body: JSON.stringify({ usernames }) });
  return res.json();
}

export async function bulkRestoreUsers(usernames) {
  const res = await apiRequest('/users/bulk/restore', { method: 'POST', body: JSON.stringify({ usernames }) });
  return res.json();
}

export async function bulkUpdateUsersType(usernames, typeId) {
  const res = await apiRequest('/users/bulk/type', { method: 'POST', body: JSON.stringify({ usernames, type_id: typeId }) });
  return res.json();
}

export async function bulkAssignUsersOrganization(usernames, organizationId) {
  const res = await apiRequest('/users/bulk/organization', { method: 'POST', body: JSON.stringify({ usernames, organization_id: organizationId }) });
  return res.json();
}

export async function bulkAssignUsersCompany(usernames, companyId) {
  const res = await apiRequest('/users/bulk/company', { method: 'POST', body: JSON.stringify({ usernames, company_id: companyId }) });
  return res.json();
}

export async function bulkBanUsers(usernames, reason) {
  const res = await apiRequest('/users/bulk/ban', { method: 'POST', body: JSON.stringify({ usernames, reason: reason || '' }) });
  return res.json();
}

export async function bulkUnbanUsers(usernames) {
  const res = await apiRequest('/users/bulk/unban', { method: 'POST', body: JSON.stringify({ usernames }) });
  return res.json();
}

/**
 * Снять блокировку входа: обнуляет счётчик неудачных попыток, лестницу кулдаунов
 * и сам лок. Возвращает `{ reset }` - false означает, что блокировки и не было.
 * Бросает на 4xx, чтобы отказ не прошёл молчаливым успехом.
 */
export async function resetUserLockout(username) {
  const res = await apiRequest(`/users/${encodeURIComponent(username)}/reset-lockout`, { method: 'POST' });
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || 'Не удалось снять блокировку входа');
  return body;
}
