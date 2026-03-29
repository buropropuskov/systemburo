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
