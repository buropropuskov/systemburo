import { apiRequest } from './client';

export async function getUsers() {
  const res = await apiRequest('/users/all');
  return res.json();
}

export async function updateUserType(username, typeId) {
  const res = await apiRequest(`/users/${username}/type`, {
    method: 'PUT',
    body: JSON.stringify({ type_id: typeId }),
  });
  return res.json();
}

export async function updateUserPassword(username, password) {
  const res = await apiRequest(`/users/${username}/password`, {
    method: 'PUT',
    body: JSON.stringify({ password }),
  });
  return res.json();
}

export async function updateUserInfo(username, data) {
  const res = await apiRequest(`/users/${username}/info`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateUserOrganization(username, organizationId) {
  const res = await apiRequest(`/users/${username}/organization`, {
    method: 'PUT',
    body: JSON.stringify({ organization_id: organizationId }),
  });
  return res.json();
}

export async function updateUserCompany(username, companyId) {
  const res = await apiRequest(`/users/${username}/company`, {
    method: 'PUT',
    body: JSON.stringify({ company_id: companyId }),
  });
  return res.json();
}

export async function deleteUser(username) {
  return apiRequest(`/users/${username}`, { method: 'DELETE' });
}
