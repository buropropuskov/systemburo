import { apiRequest } from './client';

export async function createFeedback(data) {
  const res = await apiRequest('/feedback', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function getAllFeedback() {
  const res = await apiRequest('/feedback/all');
  return res.json();
}

export async function getMyFeedback() {
  const res = await apiRequest('/feedback/my');
  return res.json();
}

export async function getFeedbackStats() {
  const res = await apiRequest('/feedback/stats');
  return res.json();
}

export async function updateFeedbackStatus(id, data) {
  const res = await apiRequest(`/feedback/${id}/status`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function markFeedbackAsRead(id) {
  return apiRequest(`/feedback/${id}/read`, { method: 'PUT' });
}
