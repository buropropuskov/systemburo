import { apiRequest } from './client';

export async function getApplications(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequest(`/applications${query ? '?' + query : ''}`);
  return res.json();
}

export async function getUserApplications(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequest(`/applications/user${query ? '?' + query : ''}`);
  return res.json();
}

export async function getApplicationById(id) {
  const res = await apiRequest(`/applications/${id}`);
  return res.json();
}

export async function createApplication(data) {
  const res = await apiRequest('/applications', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function submitCompleteApplication(data) {
  const res = await apiRequest('/applications/submit-complete-application', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateApplication(id, data) {
  const res = await apiRequest(`/applications/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function forwardApplication(id, data) {
  const res = await apiRequest(`/applications/${id}/forward`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function approveApplication(id, data) {
  const res = await apiRequest(`/applications/${id}/approve`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function takeToWork(id) {
  const res = await apiRequest(`/applications/${id}/take-to-work`, {
    method: 'POST',
  });
  return res.json();
}

export async function revokeFromWork(id) {
  const res = await apiRequest(`/applications/${id}/revoke-from-work`, {
    method: 'POST',
  });
  return res.json();
}

export async function markAsRead(id) {
  return apiRequest(`/applications/${id}/read`, { method: 'POST' });
}

export async function getUnreadCount() {
  const res = await apiRequest('/applications/unread-count');
  return res.json();
}

export async function getApplicationHistory(id) {
  const res = await apiRequest(`/applications/${id}/history`);
  return res.json();
}

export async function getApplicationDetails(id) {
  const res = await apiRequest(`/applications/${id}/details`);
  return res.json();
}

export async function getApplicationAttachments(id) {
  const res = await apiRequest(`/applications/${id}/attachments`);
  return res.json();
}
