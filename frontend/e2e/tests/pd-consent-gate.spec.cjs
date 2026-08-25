const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin, unwrap, e2eName } = require('../helpers/permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';
const SETTINGS = `${API_BASE}/settings/pd-consent`;

// Спека ВКЛЮЧАЕТ глобальное требование согласия, а оно закрывает систему всем
// сразу. Поэтому запускается только по явному флагу и обязана выключить тумблер
// в любом исходе - иначе останется закрытым общий стенд и все остальные спеки.
const ENABLED = process.env.E2E_PD_CONSENT === '1';

const LONG_TEXT = `<h2>Согласие на обработку персональных данных</h2>${
  Array.from(
    { length: 40 },
    (_, i) => `<p><strong>${i + 1}.</strong> Пункт ${i + 1}. Оператор обрабатывает `
      + 'персональные данные субъекта в объёме, необходимом для оформления и выдачи '
      + 'пропуска на территорию предприятия.</p>',
  ).join('')
}`;

const TEST_USER = {
  // e2eName, а не Date.now(): ретрай перезапускает весь serial-блок вместе с
  // beforeAll, и то же имя упёрлось бы в конфликт с недоудалённой учёткой.
  username: e2eName('consent'),
  // Пароль, заданный администратором, система считает временным: до смены она
  // отвечает 403 на защищённые методы. Учётная запись меняет его на свой сразу
  // после заведения, дальше в спеке используется только password. Значения
  // обязаны различаться - прежний пароль система повторно не примет.
  initialPassword: 'ConsentE2E-init-9137',
  password: 'ConsentE2E-own-4271',
};

let adminToken = null;
let userCreated = false;

async function setConsent(request, { text, required }) {
  const textRes = await request.put(`${SETTINGS}/text`, {
    headers: { Authorization: `Bearer ${adminToken}` },
    data: { text },
  });
  expect(textRes.ok(), await textRes.text()).toBeTruthy();
  const reqRes = await request.put(`${SETTINGS}/required`, {
    headers: { Authorization: `Bearer ${adminToken}` },
    data: { required },
  });
  expect(reqRes.ok(), await reqRes.text()).toBeTruthy();
}

async function disableConsent(request) {
  if (!adminToken) return;
  await request.put(`${SETTINGS}/required`, {
    headers: { Authorization: `Bearer ${adminToken}` },
    data: { required: false },
  }).catch(() => {});
  await request.put(`${SETTINGS}/text`, {
    headers: { Authorization: `Bearer ${adminToken}` },
    data: { text: '' },
  }).catch(() => {});
}

test.describe.serial('Гейт согласия на обработку ПД (#1567)', () => {
  test.skip(!ENABLED, 'E2E_PD_CONSENT=1 не задан - спека меняет глобальную настройку');

  test.beforeAll(async ({ request }) => {
    adminToken = await loginAsSuperAdmin(request);

    // Отдельная учётная запись: гонять согласие под общими демо-юзерами нельзя -
    // принятое согласие осталось бы у них и мешало повторным прогонам.
    const orgs = unwrap(await (await request.get(`${API_BASE}/organizations`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    })).json());
    const companies = unwrap(await (await request.get(`${API_BASE}/companies`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    })).json());
    const orgId = (Array.isArray(orgs) ? orgs : orgs.items || [])[0]?.id;
    const companyId = (Array.isArray(companies) ? companies : companies.items || [])[0]?.id;
    expect(orgId, 'нужна хотя бы одна организация').toBeTruthy();

    // Публичной регистрации в системе нет - учётку заводит администратор.
    const created = await request.post(`${API_BASE}/users`, {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: {
        username: TEST_USER.username,
        password: TEST_USER.initialPassword,
        type_id: 1,
        last_name: 'E2E',
        first_name: 'Consent',
        organization_id: orgId,
        company_id: companyId,
      },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    userCreated = true;

    // Первый вход: работник задаёт свой пароль взамен временного. Делается до
    // включения согласия - иначе гейт согласия закроет и смену пароля.
    const firstLogin = await request.post(`${API_BASE}/login`, {
      data: { username: TEST_USER.username, password: TEST_USER.initialPassword },
    });
    expect(firstLogin.ok(), await firstLogin.text()).toBeTruthy();
    const firstToken = unwrap(await firstLogin.json()).token;
    const changed = await request.put(`${API_BASE}/users/me/password`, {
      headers: { Authorization: `Bearer ${firstToken}` },
      data: {
        current_password: TEST_USER.initialPassword,
        new_password: TEST_USER.password,
      },
    });
    expect(changed.ok(), await changed.text()).toBeTruthy();

    // Гасим онбординг-тур: после подтверждения согласия он стартует и его оверлей
    // перехватывает клики, а проверяем мы здесь не тур.
    const login = await request.post(`${API_BASE}/login`, {
      data: { username: TEST_USER.username, password: TEST_USER.password },
    });
    expect(login.ok(), await login.text()).toBeTruthy();
    const userToken = unwrap(await login.json()).token;
    // Прогресс тура хранится по ключам, и непомеченный тур автозапустится (#1737).
    for (const tour of ['user', 'guard', 'approve', 'accept', 'admin']) {
      await request.post(`${API_BASE}/onboarding/complete`, {
        headers: { Authorization: `Bearer ${userToken}` },
        data: { tour, version: 99 },
      });
    }

    await setConsent(request, { text: LONG_TEXT, required: true });
  });

  test.afterAll(async ({ request }) => {
    // Выключаем ВСЕГДА: упавший тест не должен оставить стенд закрытым.
    await disableConsent(request);
    if (userCreated && adminToken) {
      await request.delete(`${API_BASE}/users/${TEST_USER.username}`, {
        headers: { Authorization: `Bearer ${adminToken}` },
      }).catch(() => {});
    }
  });

  test('без согласия protected-ручка отдаёт 403 с маркером, ручки согласия и выхода живы', async ({ request }) => {
    const login = await request.post(`${API_BASE}/login`, {
      data: { username: TEST_USER.username, password: TEST_USER.password },
    });
    expect(login.ok(), await login.text()).toBeTruthy();
    const token = unwrap(await login.json()).token;
    const auth = { Authorization: `Bearer ${token}` };

    const blocked = await request.get(`${API_BASE}/user-data`, { headers: auth });
    expect(blocked.status()).toBe(403);
    expect(blocked.headers()['x-pd-consent-required']).toBe('1');
    expect(await blocked.json()).toMatchObject({ consent_required: true });

    // Без этих ручек окно согласия - тупик.
    expect((await request.get(`${API_BASE}/consents/gate`, { headers: auth })).status()).toBe(200);
    expect((await request.get(`${API_BASE}/permissions/my`, { headers: auth })).status()).toBe(200);

    const accepted = await request.post(`${API_BASE}/consents/accept`, { headers: auth, data: {} });
    expect(accepted.ok(), await accepted.text()).toBeTruthy();

    // Доступ обязан открыться сразу, а не по истечении TTL кэша.
    expect((await request.get(`${API_BASE}/user-data`, { headers: auth })).status()).toBe(200);
  });

  test('супер-администратор при включённом гейте работает как обычно', async ({ request }) => {
    const res = await request.get(`${API_BASE}/user-data`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    });
    expect(res.status()).toBe(200);
  });

  test('подъём редакции снова закрывает доступ', async ({ request }) => {
    const login = await request.post(`${API_BASE}/login`, {
      data: { username: TEST_USER.username, password: TEST_USER.password },
    });
    const token = unwrap(await login.json()).token;
    const auth = { Authorization: `Bearer ${token}` };
    expect((await request.get(`${API_BASE}/user-data`, { headers: auth })).status()).toBe(200);

    const bumped = await request.post(`${SETTINGS}/require-again`, {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: {},
    });
    expect(bumped.ok(), await bumped.text()).toBeTruthy();

    const closed = await request.get(`${API_BASE}/user-data`, { headers: auth });
    expect(closed.status()).toBe(403);
    expect(closed.headers()['x-pd-consent-required']).toBe('1');
  });

  test('окно согласия: кнопка мертва до прокрутки, End доводит до конца, подтверждение открывает систему', async ({ page, request }) => {
    // Требование поднимаем сами: тест обязан работать и в одиночном прогоне.
    await request.post(`${SETTINGS}/require-again`, {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: {},
    });
    // Браузер логирует «Failed to load resource» на КАЖДЫЙ 403, даже когда клиент
    // обработал его молча, поэтому считаем не строки консоли, а сами запросы:
    // под окном согласия страничные данные не должны запрашиваться вовсе.
    const pageErrors = [];
    const forbidden = [];
    page.on('pageerror', (e) => pageErrors.push(e.message));
    let phase = 'до окна';
    page.on('response', (r) => {
      if (r.status() === 403) forbidden.push(`${phase}:${new URL(r.url()).pathname}`);
    });

    await page.goto('/');
    await page.getByTestId('login-input-username').fill(TEST_USER.username);
    await page.getByTestId('login-input-password').fill(TEST_USER.password);
    await page.getByTestId('login-button-submit').click();

    const accept = page.getByTestId('pdc-accept');
    await expect(accept).toBeVisible();
    phase = 'окно показано';
    // Оболочка не смонтирована: страница под окном не грузится.
    await expect(page.locator('.theheader')).toHaveCount(0);
    await expect(accept).toBeDisabled();
    await expect(page.getByTestId('pdc-agree')).toBeDisabled();

    await page.locator('.pdc-modal__doc').focus();
    await page.keyboard.press('End');
    await expect(page.getByTestId('pdc-agree')).toBeEnabled();
    await expect(accept).toBeDisabled();

    await page.getByTestId('pdc-agree').check();
    await expect(accept).toBeEnabled();
    await accept.click();

    await expect(page.locator('.pdc-overlay')).toHaveCount(0);
    await expect(page.locator('.theheader')).toBeVisible();
    expect(pageErrors, pageErrors.join(' | ')).toHaveLength(0);
    // Стена отказов вместо окна - главный симптом неверного белого списка.
    await expect(page.getByText('Недостаточно прав')).toHaveCount(0);
    // Данные страниц под окном не грузятся: router-view не смонтирован.
    const pageData = forbidden.filter((x) => /\/(news|notifications|system-tables|onboarding|applications|announcements|documents)/.test(x));
    expect(pageData, `403 под окном: ${forbidden.join(', ')}`).toHaveLength(0);
  });

  test('на телефоне лист не смахивается свайпом вниз', async ({ browser, request }) => {
    // Возвращаем требование: предыдущий тест его снял подтверждением.
    const bumped = await request.post(`${SETTINGS}/require-again`, {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: {},
    });
    expect(bumped.ok()).toBeTruthy();

    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
      hasTouch: true,
      isMobile: true,
      ...(process.env.E2E_HTTP_USER && {
        httpCredentials: {
          username: process.env.E2E_HTTP_USER,
          password: process.env.E2E_HTTP_PASSWORD,
        },
      }),
    });
    const page = await context.newPage();
    try {
      await page.goto('/');
      await page.getByTestId('login-input-username').fill(TEST_USER.username);
      await page.getByTestId('login-input-password').fill(TEST_USER.password);
      await page.getByTestId('login-button-submit').click();
      await expect(page.getByTestId('pdc-accept')).toBeVisible();

      const box = await page.locator('.pdc-modal').boundingBox();
      const cdp = await context.newCDPSession(page);
      const x = Math.round(box.x + box.width / 2);
      const y = Math.round(box.y + 24);
      await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y }] });
      for (const dy of [80, 180, 300]) {
        await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x, y: y + dy }] });
      }
      await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });

      await expect(page.locator('.pdc-overlay')).toBeVisible();
      const modal = await page.locator('.pdc-modal').boundingBox();
      expect(modal.y + modal.height).toBeLessThanOrEqual(844 + 1);
    } finally {
      await context.close();
    }
  });
});
