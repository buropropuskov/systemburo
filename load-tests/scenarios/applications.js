import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, defaultOptions } from '../k6-config.js';

export const options = defaultOptions;

function login(username) {
  const res = http.post(`${BASE_URL}/login`, JSON.stringify({
    username: username,
    password: 'LoadTest123456789012345678901234',
  }), { headers: { 'Content-Type': 'application/json' } });

  if (res.status === 200) {
    try { return JSON.parse(res.body).data.token; } catch { return null; }
  }
  return null;
}

export default function () {
  const userIndex = Math.floor(Math.random() * 1000);
  const token = login(`loadtest_user_${userIndex}`);
  if (!token) return;

  const headers = {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  };

  const listRes = http.get(`${BASE_URL}/applications?page=1&limit=20`, { headers });
  check(listRes, {
    'list status 200': (r) => r.status === 200,
  });

  const unreadRes = http.get(`${BASE_URL}/applications/unread-count`, { headers });
  check(unreadRes, {
    'unread status 200': (r) => r.status === 200,
  });

  sleep(Math.random() * 3 + 1);
}
