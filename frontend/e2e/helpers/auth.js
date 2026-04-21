const { LoginPage } = require('../pages/LoginPage');

const API_BASE = 'http://localhost:8080';

// Сеид-пользователи создаются cmd/seed (с SEED_E2E_USERS=true в CI).
const SEED_ADMIN = { username: 'e2e_admin', password: 'testpass123' };
const SEED_USER = { username: 'e2e_user', password: 'testpass123' };

/**
 * No-op stub для совместимости с существующими тестами, которые вызывают
 * registerUser(name, password, typeId). В бекенде нет /register — все юзеры
 * приходят из seed (cmd/seed с SEED_E2E_USERS=true). Аргументы игнорируются;
 * login* функции используют фиксированные seed-аккаунты.
 */
async function registerUser() {
  return new Response(null, { status: 204 });
}

function unwrap(body) {
  if (body && typeof body === 'object' && 'success' in body) {
    return body.data;
  }
  return body;
}

async function loginViaAPI(username, password) {
  const response = await fetch(`${API_BASE}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!response.ok) {
    throw new Error(`loginViaAPI(${username}) failed: ${response.status}`);
  }
  return unwrap(await response.json());
}

async function loginAsAdmin(page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(SEED_ADMIN.username, SEED_ADMIN.password);
  await page.waitForURL('/personal-cabinet');
}

async function loginAsUser(page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(SEED_USER.username, SEED_USER.password);
  await page.waitForURL('/personal-cabinet');
}

async function setAuthTokens(page, username, password) {
  const data = await loginViaAPI(username, password);
  await page.goto('/');
  await page.evaluate(
    ({ token, refreshToken }) => {
      localStorage.setItem('token', token);
      localStorage.setItem('refreshToken', refreshToken);
    },
    { token: data.token, refreshToken: data.refreshToken || data.refresh_token }
  );
}

module.exports = {
  registerUser,
  loginViaAPI,
  loginAsAdmin,
  loginAsUser,
  setAuthTokens,
  SEED_ADMIN,
  SEED_USER,
  unwrap,
};
