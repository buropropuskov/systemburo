import { apiRequest } from './client';

export async function getActiveCarsForTables() {
  const res = await apiRequest('/cars/active-for-tables');
  return res.json();
}

export async function getFactCarsForTables() {
  const res = await apiRequest('/cars/fact-for-tables');
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
