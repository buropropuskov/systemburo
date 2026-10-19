import { apiRequest } from './client';

export function createRegistryCar(data) {
  return apiRequest('/unique-cars', { method: 'POST', body: JSON.stringify(data) });
}

export function updateRegistryCar(id, data) {
  return apiRequest(`/unique-cars/${id}`, { method: 'PUT', body: JSON.stringify(data) });
}

export function deleteRegistryCar(id) {
  return apiRequest(`/unique-cars/${id}`, { method: 'DELETE' });
}

export function createRegistryEmployee(data) {
  return apiRequest('/unique-employees', { method: 'POST', body: JSON.stringify(data) });
}

export function updateRegistryEmployee(id, data) {
  return apiRequest(`/unique-employees/${id}`, { method: 'PUT', body: JSON.stringify(data) });
}

export function deleteRegistryEmployee(id) {
  return apiRequest(`/unique-employees/${id}`, { method: 'DELETE' });
}
