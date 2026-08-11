import { apiRequest } from './client';

// Legacy (#229 переехало на новые ключи через permission_groups, оставлено для совместимости).
export async function getMyPermissions() {
  const res = await apiRequest('/permissions/my');
  return res.json();
}

export async function getUserPermissions(userId) {
  const res = await apiRequest(`/permissions/user/${userId}`);
  return res.json();
}

export async function updateUserPermissions(userId, data) {
  const res = await apiRequest(`/permissions/user/${userId}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

// Эффективные права целевого юзера с источником (роль/группа/override) для
// правого столбца UserAccessModal. Формат: {mode, permissions[{key,value,source}],
// denied, banned, ban_reason}. Только super-admin (#187 Фаза 3).
export async function getUserEffectivePermissions(userId) {
  const res = await apiRequest(`/permissions/user/${userId}/effective`);
  return res.json();
}

// Иерархический каталог точечных прав (категория -> листья, super_only, table.*).
export async function getPermissionCatalog() {
  const res = await apiRequest('/permissions/catalog');
  return res.json();
}

// Выдача/снятие флага "Администратор" целевому юзеру (super-only).
export async function setUserAdmin(userId, isAdmin) {
  const res = await apiRequest(`/users/${userId}/admin`, {
    method: 'PUT',
    body: JSON.stringify({ is_admin: isAdmin }),
  });
  return res.json();
}

// Назначение роли юзеру (null = без роли).
export async function setUserRole(userId, roleId) {
  const res = await apiRequest(`/users/${userId}/role`, {
    method: 'PUT',
    body: JSON.stringify({ role_id: roleId }),
  });
  return res.json();
}

// --- Permission Groups (#229) ---

export async function listPermissionGroups() {
  const res = await apiRequest('/permission-groups');
  return res.json();
}

export async function getPermissionGroup(id) {
  const res = await apiRequest(`/permission-groups/${id}`);
  return res.json();
}

export async function createPermissionGroup(data) {
  const res = await apiRequest('/permission-groups', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updatePermissionGroup(id, data) {
  const res = await apiRequest(`/permission-groups/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deletePermissionGroup(id) {
  const res = await apiRequest(`/permission-groups/${id}`, {
    method: 'DELETE',
  });
  return res.json();
}

export async function mergePermissionGroups(data) {
  const res = await apiRequest('/permission-groups/merge', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function assignGroupToUser(userId, groupId) {
  const res = await apiRequest(`/users/${userId}/permission-groups/${groupId}`, {
    method: 'POST',
  });
  return res.json();
}

export async function unassignGroupFromUser(userId, groupId) {
  const res = await apiRequest(`/users/${userId}/permission-groups/${groupId}`, {
    method: 'DELETE',
  });
  return res.json();
}

// --- Roles (#229) ---

export async function listRoles() {
  const res = await apiRequest('/roles');
  return res.json();
}

export async function createRole(data) {
  const res = await apiRequest('/roles', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateRole(id, data) {
  const res = await apiRequest(`/roles/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteRole(id) {
  const res = await apiRequest(`/roles/${id}`, {
    method: 'DELETE',
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(body?.error || body?.message || 'Не удалось удалить роль');
  }
  return body;
}

export async function setRoleDefaultGroups(id, groupIds) {
  const res = await apiRequest(`/roles/${id}/default-groups`, {
    method: 'PUT',
    body: JSON.stringify({ group_ids: groupIds }),
  });
  return res.json();
}

// Точечные права роли (полная замена прямых allow-грантов по ключам каталога).
export async function setRolePermissions(id, keys) {
  const res = await apiRequest(`/roles/${id}/permissions`, {
    method: 'PUT',
    body: JSON.stringify({ keys }),
  });
  return res.json();
}

// --- Access Denials (#230) ---

export async function listAccessDenials(params = {}) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') qs.set(k, v);
  });
  const url = '/access-denials' + (qs.size ? `?${qs}` : '');
  const res = await apiRequest(url);
  return res.json();
}

export async function listAccessDenialsArchive(params = {}) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') qs.set(k, v);
  });
  const url = '/access-denials/archive' + (qs.size ? `?${qs}` : '');
  const res = await apiRequest(url);
  return res.json();
}

export async function deleteAccessDenials(params = {}) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') qs.set(k, v);
  });
  const url = '/access-denials' + (qs.size ? `?${qs}` : '');
  const res = await apiRequest(url, { method: 'DELETE' });
  return res.json();
}

export async function archiveAccessDenials(cutoff) {
  const url = cutoff ? `/access-denials/archive?cutoff=${encodeURIComponent(cutoff)}` : '/access-denials/archive';
  const res = await apiRequest(url, { method: 'POST' });
  return res.json();
}

// --- Ban (#230) ---

export async function banUser(userId, reason = '') {
  const res = await apiRequest(`/users/${userId}/ban`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  });
  return res.json();
}

export async function unbanUser(userId) {
  const res = await apiRequest(`/users/${userId}/unban`, { method: 'POST' });
  return res.json();
}
