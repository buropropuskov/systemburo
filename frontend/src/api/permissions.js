import { apiRequest } from './client';

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

export async function getPermissionTree() {
  const res = await apiRequest('/permissions/tree');
  return res.json();
}
