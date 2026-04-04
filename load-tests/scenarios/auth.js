import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, defaultOptions } from '../k6-config.js';

export const options = {
  ...defaultOptions,
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 200 },
    { duration: '30s', target: 0 },
  ],
};

export default function () {
  const userIndex = Math.floor(Math.random() * 1000);
  const username = `loadtest_user_${userIndex}`;
  const password = 'LoadTest123456789012345678901234';

  const loginRes = http.post(`${BASE_URL}/login`, JSON.stringify({
    username: username,
    password: password,
  }), { headers: { 'Content-Type': 'application/json' } });

  check(loginRes, {
    'login status 200': (r) => r.status === 200,
    'has token': (r) => {
      try { return !!JSON.parse(r.body).data.token; } catch { return false; }
    },
  });

  if (loginRes.status === 200) {
    let token;
    try { token = JSON.parse(loginRes.body).data.token; } catch { return; }

    const userDataRes = http.get(`${BASE_URL}/users/me`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    check(userDataRes, {
      'user-data status 200': (r) => r.status === 200,
    });
  }

  sleep(Math.random() * 2 + 1);
}
