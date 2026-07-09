import { apiRequest } from './client';

export async function getActiveCarsForTables() {
  const res = await apiRequest('/cars/active-for-tables');
  return res.json();
}

export async function getFactCarsForTables() {
  const res = await apiRequest('/cars/fact-for-tables');
  return res.json();
}

export async function getActiveCarsForTable(tableId) {
  const res = await apiRequest(`/cars/active-for-table/${tableId}`);
  return res.json();
}

/**
 * Ручное добавление машин в таблицу без заявки (#1049, режим-1).
 * payload -> services.ManualCarRequest (snake_case): organization_id, company_id,
 * table_id, entry_date_from/to, entry_time_from/to, roof_access, free_parking, vehicles[].
 * @param {object} payload
 * @returns {Promise<{success: boolean, message: string, attachment_id: number, car_ids: number[]}>}
 */
export async function createManualCars(payload) {
  const res = await apiRequest('/cars/manual', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.message || 'Не удалось добавить машины вручную');
  }
  return res.json();
}

export async function getFactCarsForTable(tableId) {
  const res = await apiRequest(`/cars/fact-for-table/${tableId}`);
  return res.json();
}

export async function getCarHistory(id) {
  const res = await apiRequest(`/cars/${id}/history`);
  return res.json();
}

export async function addCarHistoryEntry(id, data) {
  const res = await apiRequest(`/cars/${id}/history`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateTerritoryStatus(id, data) {
  const res = await apiRequest(`/cars/${id}/territory-status`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deactivateCar(id) {
  const res = await apiRequest(`/cars/${id}/deactivate`, { method: 'PUT' });
  return res.json();
}

export async function activateCar(id) {
  const res = await apiRequest(`/cars/${id}/activate`, { method: 'PUT' });
  return res.json();
}

export async function getCarsCurrentStatus() {
  const res = await apiRequest('/cars/history/current-status');
  return res.json();
}

/**
 * Поиск машины в реестре по номеру и марке (для открытия карточки со страницы ЧС).
 * Возвращает запись unique_car или null, если совпадения нет (404).
 */
export async function lookupUniqueCar({ number, mark }) {
  const qs = new URLSearchParams({ number, mark: mark || '' });
  const res = await apiRequest(`/unique-cars/lookup?${qs.toString()}`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`Ошибка поиска машины (${res.status})`);
  return res.json();
}
