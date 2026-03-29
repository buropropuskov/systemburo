import { apiRequest } from './client';

export async function getOrganizations() {
  const res = await apiRequest('/organizations');
  return res.json();
}

export async function createOrganization(data) {
  const res = await apiRequest('/organizations', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateOrganization(id, data) {
  const res = await apiRequest(`/organizations/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteOrganization(id) {
  return apiRequest(`/organizations/${id}`, { method: 'DELETE' });
}

export async function getOrganizationsWithUsers() {
  const res = await apiRequest('/organizations/with-users');
  return res.json();
}

export async function getMyOrganization() {
  const res = await apiRequest('/get-organization');
  return res.json();
}

export async function getCompanies() {
  const res = await apiRequest('/companies');
  return res.json();
}

export async function createCompany(data) {
  const res = await apiRequest('/companies', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateCompany(id, data) {
  const res = await apiRequest(`/companies/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteCompany(id) {
  return apiRequest(`/companies/${id}`, { method: 'DELETE' });
}

export async function getCompaniesWithUsers() {
  const res = await apiRequest('/companies/with-users');
  return res.json();
}
