import { apiRequest } from './client';

export async function getSystemTables() {
  const res = await apiRequest('/system-tables');
  return res.json();
}

export async function getSystemTable(id) {
  const res = await apiRequest(`/system-tables/${id}`);
  return res.json();
}

export async function createSystemTable(data) {
  const res = await apiRequest('/system-tables', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateSystemTable(id, data) {
  const res = await apiRequest(`/system-tables/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteSystemTable(id) {
  return apiRequest(`/system-tables/${id}`, {
    method: 'DELETE',
  });
}

export async function uploadTablePhotos(tableId, formData) {
  const res = await apiRequest(`/system-tables/${tableId}/photos`, {
    method: 'POST',
    body: formData,
    headers: {},
  });
  return res.json();
}

export async function deleteTablePhoto(tableId, photoId) {
  return apiRequest(`/system-tables/${tableId}/photos/${photoId}`, {
    method: 'DELETE',
  });
}

export async function setMainTablePhoto(tableId, photoId) {
  return apiRequest(`/system-tables/${tableId}/photos/${photoId}/main`, {
    method: 'POST',
  });
}
