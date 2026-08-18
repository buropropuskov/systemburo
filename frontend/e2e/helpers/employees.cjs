/**
 * API helpers для unique-employees (/api/unique-employees).
 */

const { loginAsSuperAdmin, e2eName } = require('./permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function unwrap(body) {
  if (body && typeof body === 'object' && 'success' in body) {
    if (!body.success) throw new Error(`API error: ${body.error || 'unknown'}`);
    return body.data;
  }
  return body;
}

function authHeaders(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

async function listEmployees(request, token) {
  const res = await request.get(`${API_BASE}/unique-employees`, { headers: authHeaders(token) });
  if (!res.ok()) throw new Error(`GET /unique-employees failed: ${res.status()}`);
  return unwrap(await res.json());
}

async function createEmployee(request, token, data) {
  const res = await request.post(`${API_BASE}/unique-employees`, {
    headers: authHeaders(token),
    // Запись реестра создаётся только с подтверждённым согласием субъекта на обработку
    // персональных данных - сервер без флага отвечает 400. Тест может прислать свой
    // pd_consent, если проверяет именно этот отказ.
    data: { pd_consent: true, ...data },
  });
  if (!res.ok() && res.status() !== 201) {
    throw new Error(`POST /unique-employees failed: ${res.status()} ${await res.text()}`);
  }
  return unwrap(await res.json());
}

async function deleteEmployee(request, token, id) {
  const res = await request.delete(`${API_BASE}/unique-employees/${id}`, { headers: authHeaders(token) });
  if (!res.ok() && res.status() !== 204) {
    throw new Error(`DELETE /unique-employees/${id} failed: ${res.status()}`);
  }
}

async function cleanupE2eEmployees(request) {
  const token = await loginAsSuperAdmin(request);
  const list = await listEmployees(request, token).catch(() => []);
  for (const e of list) {
    const last = e.last_name || '';
    if (last.startsWith('E2E_')) {
      await deleteEmployee(request, token, e.id).catch(() => {});
    }
  }
}

module.exports = {
  listEmployees,
  createEmployee,
  deleteEmployee,
  cleanupE2eEmployees,
  e2eEmployeeName: e2eName,
};
