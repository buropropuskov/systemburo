const { test, expect } = require('@playwright/test');

async function openLogin(page, viewport) {
  await page.setViewportSize(viewport);
  await page.route((url) => url.pathname.startsWith('/api/'), async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/settings/contacts') {
      return route.fulfill({ json: { success: true, data: { email: 'layout-preview@example.org', phone: '+7 (000) 000-00-00' } } });
    }
    if (path === '/api/login') {
      return route.fulfill({ status: 400, json: { success: false, error: 'Учебная ошибка входа для проверки расположения кнопки' } });
    }
    if (path === '/api/refresh-token' || path === '/api/users/me') {
      return route.fulfill({ status: 401, json: { success: false, error: 'Unauthorized' } });
    }
    return route.fulfill({ json: { success: true, data: {} } });
  });
  await page.goto('/');
  await expect(page.getByPlaceholder('Логин', { exact: true })).toBeVisible();
  await expect(page.locator('.contact__text').first()).toHaveText('layout-preview@example.org');
  await expect(page.locator('.login__info')).toHaveCSS('opacity', '1');
  await page.evaluate(() => document.fonts.ready);
}

async function geometry(page) {
  return page.evaluate(() => {
    const box = (selector) => {
      const r = document.querySelector(selector).getBoundingClientRect();
      return { x: r.x, y: r.y, width: r.width, height: r.height, right: r.right, bottom: r.bottom };
    };
    return {
      active: box('.login__active'), info: box('.login__info'), hours: box('.time'),
      button: box('.login__button'), forgot: box('.remember-password'),
      intro: box('.info__text'), help: box('.info__title.help'),
      overflow: document.documentElement.scrollWidth > window.innerWidth,
    };
  });
}

for (const viewport of [
  { width: 768, height: 1024 }, { width: 1024, height: 768 },
  { width: 1024, height: 1366 }, { width: 1366, height: 1024 },
]) {
  test(`Login tablet ${viewport.width}x${viewport.height}: natural spacing and visible contacts`, async ({ page }) => {
    await openLogin(page, viewport);
    const g = await geometry(page);
    expect(g.overflow).toBe(false);
    expect(g.hours.bottom).toBeLessThanOrEqual(g.info.bottom - 10);
    expect(g.hours.right).toBeLessThanOrEqual(g.info.right);
    expect(g.button.y - g.forgot.bottom).toBeLessThanOrEqual(64);
    expect(g.help.y - g.intro.bottom).toBeLessThanOrEqual(48);
    if (viewport.height > viewport.width) {
      expect(g.info.y).toBeGreaterThanOrEqual(g.active.bottom);
      expect(Math.abs(g.info.width - g.active.width)).toBeLessThan(2);
    } else {
      expect(g.info.x).toBeGreaterThan(g.active.right);
      expect(Math.abs(g.info.y - g.active.y)).toBeLessThan(2);
    }
    await expect(page.getByPlaceholder('Логин', { exact: true })).toBeEditable();
    await expect(page.getByPlaceholder('Пароль', { exact: true })).toBeEditable();
  });
}

for (const [viewport, titleSize, inputHeight] of [
  [{ width: 390, height: 844 }, '32px', 56],
  [{ width: 1440, height: 900 }, '60px', 70],
]) {
  test(`Login ${viewport.width}px: preserve existing phone/desktop sizes`, async ({ page }) => {
    await openLogin(page, viewport);
    await expect(page.locator('.login__title')).toHaveCSS('font-size', titleSize);
    const box = await page.locator('.login__input').first().boundingBox();
    expect(box.height).toBeCloseTo(inputHeight, 0);
    expect((await geometry(page)).overflow).toBe(false);
  });
}

test('Login tablet: error text does not overlap submit button', async ({ page }) => {
  await openLogin(page, { width: 1024, height: 600 });
  await page.getByPlaceholder('Логин', { exact: true }).fill('layout-preview');
  await page.getByPlaceholder('Пароль', { exact: true }).fill('ExampleOnly123');
  await page.getByTestId('login-button-submit').click();
  const error = page.getByTestId('login-error-message');
  await expect(error).toBeVisible();
  const errorBox = await error.boundingBox();
  const g = await geometry(page);
  expect(errorBox.y + errorBox.height).toBeLessThanOrEqual(g.button.y + 1);
  expect(g.hours.bottom).toBeLessThanOrEqual(g.info.bottom - 10);
  expect(g.overflow).toBe(false);
});
