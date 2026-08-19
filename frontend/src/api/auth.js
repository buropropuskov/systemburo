import { apiRequest } from './client';

export async function login(username, password) {
  const res = await apiRequest('/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
  return res.json();
}

export async function refreshToken(token) {
  const res = await apiRequest('/refresh-token', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: token }),
  });
  return res.json();
}

export async function getMe() {
  const res = await apiRequest('/users/me');
  return res.json();
}

export async function getUserData() {
  const res = await apiRequest('/user-data');
  return res.json();
}
