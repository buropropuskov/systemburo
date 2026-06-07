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
