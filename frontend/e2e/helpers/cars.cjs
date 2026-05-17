/**
 * API helpers для unique-cars (/api/unique-cars).
 * Используются e2e-тестами для setup/teardown тестовых машин.
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

async function listCars(request, token) {
  const res = await request.get(`${API_BASE}/unique-cars`, { headers: authHeaders(token) });
  if (!res.ok()) throw new Error(`GET /unique-cars failed: ${res.status()}`);
  return unwrap(await res.json());
}

async function createCar(request, token, data) {
  const res = await request.post(`${API_BASE}/unique-cars`, {
    headers: authHeaders(token),
    data,
  });
  if (!res.ok() && res.status() !== 201) {
    throw new Error(`POST /unique-cars failed: ${res.status()} ${await res.text()}`);
  }
  return unwrap(await res.json());
}

async function deleteCar(request, token, id) {
  const res = await request.delete(`${API_BASE}/unique-cars/${id}`, { headers: authHeaders(token) });
  if (!res.ok() && res.status() !== 204) {
    throw new Error(`DELETE /unique-cars/${id} failed: ${res.status()}`);
  }
}

async function cleanupE2eCars(request) {
  const token = await loginAsSuperAdmin(request);
  const cars = await listCars(request, token).catch(() => []);
  for (const c of cars) {
    const mark = c.mark || '';
    const number = c.number || '';
    if (mark.startsWith('e2e_') || number.startsWith('E2E')) {
      await deleteCar(request, token, c.id).catch(() => {});
    }
  }
}

module.exports = {
  listCars,
  createCar,
  deleteCar,
  cleanupE2eCars,
  e2eCarName: e2eName,
};
