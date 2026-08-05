/**
 * Helpers для e2e-тестов системы прав (роли, группы, журнал отказов, ban).
 * Через `request` Playwright-контекст. Учитывает httpCredentials из playwright.config.
 * Все методы возвращают unwrapped data (без envelope success/data).
 */

const SUPER_ADMIN = {
  username: process.env.E2E_SUPERADMIN_USER || 'buropropuskov',
  password: process.env.E2E_SUPERADMIN_PASSWORD || 'admin123',
};

// Прямой URL до backend API. На staging - nginx проксирует /api на бэк
// (https://stagingburo.washka17.ru/api). В CI и dev фронт на :8081
// без proxy, бэк на :8080 - тогда нужен absolute URL.
const API_BASE = process.env.E2E_API_BASE_URL || '/api';

const E2E_PREFIX = 'e2e_';

function unwrap(body) {
  if (body && typeof body === 'object' && 'success' in body) {
    if (!body.success) {
      throw new Error(`API error: ${body.error || 'unknown'}`);
    }
    return body.data;
  }
  return body;
}

async function loginAsSuperAdmin(request) {
  const res = await request.post(`${API_BASE}/login`, {
    data: { username: SUPER_ADMIN.username, password: SUPER_ADMIN.password },
  });
  if (!res.ok()) {
    throw new Error(`superadmin login failed: ${res.status()} ${await res.text()}`);
  }
  const data = unwrap(await res.json());
  if (!data.token) throw new Error('login response missing token');
  return data.token;
}

function authHeaders(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

async function apiGet(request, token, path) {
  const res = await request.get(`${API_BASE}${path}`, { headers: authHeaders(token) });
  if (!res.ok()) throw new Error(`GET ${path} failed: ${res.status()}`);
  return unwrap(await res.json());
}

async function apiPost(request, token, path, body) {
  const res = await request.post(`${API_BASE}${path}`, { headers: authHeaders(token), data: body || {} });
  if (!res.ok() && res.status() !== 201) {
    throw new Error(`POST ${path} failed: ${res.status()} ${await res.text()}`);
  }
  return unwrap(await res.json());
}

async function apiPut(request, token, path, body) {
  const res = await request.put(`${API_BASE}${path}`, { headers: authHeaders(token), data: body || {} });
  if (!res.ok()) throw new Error(`PUT ${path} failed: ${res.status()} ${await res.text()}`);
  return unwrap(await res.json());
}

async function apiDelete(request, token, path) {
  const res = await request.delete(`${API_BASE}${path}`, { headers: authHeaders(token) });
  if (!res.ok() && res.status() !== 204) {
    throw new Error(`DELETE ${path} failed: ${res.status()} ${await res.text()}`);
  }
  if (res.status() === 204) return null;
  return unwrap(await res.json());
}

// Что создал текущий воркер. Playwright держит спек-файл в отдельном процессе,
// поэтому реестр не пересекается с параллельным файлом - его сущности останутся
// нетронутыми, даже если совпадёт префикс имени.
const ownEntities = { roles: [], groups: [] };

const listRoles = (req, t) => apiGet(req, t, '/roles');
const createRole = async (req, t, data) => {
  const role = await apiPost(req, t, '/roles', data);
  if (role && role.id) ownEntities.roles.push(role.id);
  return role;
};
const updateRole = (req, t, id, data) => apiPut(req, t, `/roles/${id}`, data);
const deleteRole = (req, t, id) => apiDelete(req, t, `/roles/${id}`);
const setRoleDefaultGroups = (req, t, id, groupIds) =>
  apiPut(req, t, `/roles/${id}/default-groups`, { group_ids: groupIds });

const listGroups = (req, t) => apiGet(req, t, '/permission-groups');
const createGroup = async (req, t, data) => {
  const group = await apiPost(req, t, '/permission-groups', data);
  if (group && group.id) ownEntities.groups.push(group.id);
  return group;
};
const updateGroup = (req, t, id, data) => apiPut(req, t, `/permission-groups/${id}`, data);
const deleteGroup = (req, t, id) => apiDelete(req, t, `/permission-groups/${id}`);
const mergeGroups = async (req, t, data) => {
  const group = await apiPost(req, t, '/permission-groups/merge', data);
  if (group && group.id) ownEntities.groups.push(group.id);
  return group;
};

const assignGroupToUser = (req, t, userId, groupId) =>
  apiPost(req, t, `/users/${userId}/permission-groups/${groupId}`);
const unassignGroupFromUser = (req, t, userId, groupId) =>
  apiDelete(req, t, `/users/${userId}/permission-groups/${groupId}`);
const setUserRole = (req, t, userId, roleId) =>
  apiPut(req, t, `/users/${userId}/role`, { role_id: roleId });

const banUser = (req, t, userId) => apiPost(req, t, `/users/${userId}/ban`);
const unbanUser = (req, t, userId) => apiDelete(req, t, `/users/${userId}/ban`);

const listAccessDenials = (req, t, params = {}) => {
  const qs = new URLSearchParams(params).toString();
  return apiGet(req, t, `/access-denials${qs ? '?' + qs : ''}`);
};

function uniqSuffix() {
  return Math.random().toString(36).slice(2, 8);
}

function e2eName(prefix) {
  return `${E2E_PREFIX}${prefix}_${uniqSuffix()}`;
}

// Порог "сущность брошена". Уборка идёт по общему префиксу e2e_, а пять
// permissions-спек живут в одном шарде и бегут на двух воркерах - без порога
// afterAll одного файла сносил группу, которую сосед только что создал и ещё
// проверяет. Своё удаляем точно по реестру, по префиксу - только остатки
// прошлых прогонов.
const ABANDONED_AFTER_MS = 10 * 60 * 1000;

/** Сущность из прошлого прогона, а не живая работа параллельного файла. */
function isAbandoned(entity) {
  const createdAt = Date.parse(entity.created_at || '');
  return Number.isFinite(createdAt) && Date.now() - createdAt > ABANDONED_AFTER_MS;
}

/**
 * Удаляет созданное этим файлом и брошенные e2e_-остатки прошлых прогонов
 * (best-effort, без throw). Вызывается в afterAll/afterEach чтобы staging
 * не зарастал тестовыми данными.
 */
async function cleanupE2eEntities(request) {
  const token = await loginAsSuperAdmin(request);

  for (const roleId of ownEntities.roles.splice(0)) {
    await deleteRole(request, token, roleId).catch(() => {});
  }
  for (const groupId of ownEntities.groups.splice(0)) {
    await deleteGroup(request, token, groupId).catch(() => {});
  }

  for (const role of await listRoles(request, token).catch(() => [])) {
    const isE2e = (role.code || '').startsWith(E2E_PREFIX) || (role.name || '').startsWith(E2E_PREFIX);
    if (isE2e && isAbandoned(role)) {
      await deleteRole(request, token, role.id).catch(() => {});
    }
  }
  for (const group of await listGroups(request, token).catch(() => [])) {
    if ((group.name || '').startsWith(E2E_PREFIX) && isAbandoned(group)) {
      await deleteGroup(request, token, group.id).catch(() => {});
    }
  }
}

module.exports = {
  SUPER_ADMIN,
  E2E_PREFIX,
  loginAsSuperAdmin,
  apiGet,
  apiPost,
  apiPut,
  apiDelete,
  unwrap,
  listRoles,
  createRole,
  updateRole,
  deleteRole,
  setRoleDefaultGroups,
  listGroups,
  createGroup,
  updateGroup,
  deleteGroup,
  mergeGroups,
  assignGroupToUser,
  unassignGroupFromUser,
  setUserRole,
  banUser,
  unbanUser,
  listAccessDenials,
  e2eName,
  uniqSuffix,
  cleanupE2eEntities,
};
