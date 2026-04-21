const { LoginPage } = require('../pages/LoginPage');

const API_BASE = 'http://localhost:8080';

async function registerUser(username, password, typeId = 1, orgId = 0, companyId = 0) {
  const response = await fetch(`${API_BASE}/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      username,
      password,
      type_id: typeId,
      organization_id: orgId,
      company_id: companyId,
    }),
  });
  return response;
}

async function loginViaAPI(username, password) {
  const response = await fetch(`${API_BASE}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  return response.json();
}

async function loginAsAdmin(page) {
  await registerUser('e2e_admin', 'testpass123', 6);
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login('e2e_admin', 'testpass123');
  await page.waitForURL('/personal-cabinet');
}

async function loginAsUser(page, username = 'e2e_user') {
  await registerUser(username, 'testpass123', 1);
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(username, 'testpass123');
  await page.waitForURL('/personal-cabinet');
}

async function setAuthTokens(page, username, password) {
  const data = await loginViaAPI(username, password);
  await page.goto('/');
  await page.evaluate(({ token, refreshToken }) => {
    localStorage.setItem('token', token);
    localStorage.setItem('refreshToken', refreshToken);
  }, { token: data.token, refreshToken: data.refreshToken || data.refresh_token });
}

module.exports = { registerUser, loginViaAPI, loginAsAdmin, loginAsUser, setAuthTokens };
