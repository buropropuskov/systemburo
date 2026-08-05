const { LoginPage } = require('../pages/LoginPage');

// /api префикс для backend (router.Setup использует api := e.Group("/api")).
// Для e2e хелпера зовём через абсолютный URL на localhost:8080 (docker compose dev);
// в CI и staging тесты идут через playwright baseURL + /api относительно.
const API_BASE = 'http://localhost:8080/api';

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

// Landing после логина зависит от роли и Pinia state: super-admin и
// часть админов идут на /news (обзор и новости), обычный юзер - на
// /personal-cabinet. Ждём любой из них.
const POST_LOGIN_URL = /\/(personal-cabinet|news)(\?|$|\/)/;

async function loginAsAdmin(page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(SEED_ADMIN.username, SEED_ADMIN.password);
  await page.waitForURL(POST_LOGIN_URL);
}

/**
 * Вход через форму под произвольной учёткой - для сценариев, где роли разведены по
 * созданным тестом пользователям, а не по фиксированным сид-аккаунтам.
 */
async function loginAsUI(page, username, password) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(username, password);
  await page.waitForURL(POST_LOGIN_URL);
}

async function loginAsUser(page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(SEED_USER.username, SEED_USER.password);
  await page.waitForURL(POST_LOGIN_URL);
}

const SUPER_ADMIN_UI = {
  username: process.env.E2E_SUPERADMIN_USER || 'buropropuskov',
  password: process.env.E2E_SUPERADMIN_PASSWORD || 'admin123',
};

async function loginAsSuperAdminUI(page) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(SUPER_ADMIN_UI.username, SUPER_ADMIN_UI.password);
  await page.waitForURL(POST_LOGIN_URL);
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
  loginAsUI,
  loginAsSuperAdminUI,
  setAuthTokens,
  SEED_ADMIN,
  SEED_USER,
  SUPER_ADMIN_UI,
  unwrap,
  POST_LOGIN_URL,
};
