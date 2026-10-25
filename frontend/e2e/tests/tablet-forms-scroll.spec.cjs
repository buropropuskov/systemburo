const { test, expect } = require('@playwright/test');
const { tabletFixtures, CLOCK } = require('../helpers/tablet-fixtures.cjs');

const noOverflow = page => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth);

test('tablet application: four attachment categories, full-width fields and long attachment title', async ({ page }) => {
  await tabletFixtures(page);
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto('/new-application');
  await expect(page.locator('.category')).toHaveCount(4);
  await page.locator('.category').filter({ hasText: 'Сотрудники' }).getByRole('button', { name: 'Добавить', exact: true }).click();
  await expect(page.getByPlaceholder('Введите фамилию', { exact: true })).toBeVisible();
  await expect(page.locator('.recipients-chips > .recipient-chip')).toHaveCount(1);
  await page.locator('.attachment.selected').hover();
  await page.locator('.attachment.selected .edit-btn').click();
  const name = page.locator('.attachment-name-input');
  await name.fill('Учебное очень длинное название вложения для проверки границ и сохранности элементов');
  await name.press('Enter');
  for (const viewport of [{ width: 768, height: 1024 }, { width: 1024, height: 768 }, { width: 1366, height: 1024 }]) {
    await page.setViewportSize(viewport);
    await expect.poll(() => noOverflow(page)).toBe(true);
    const boxes = await page.evaluate(() => {
      const rect = selector => document.querySelector(selector).getBoundingClientRect();
      const selector = rect('.selector'), form = rect('.create__form'), fields = rect('.data__completion');
      return { selector: selector.width, form: form.width, fields: fields.width, above: selector.bottom <= form.top };
    });
    expect(boxes.above).toBe(true);
    expect(Math.abs(boxes.selector - boxes.form)).toBeLessThan(2);
    expect(Math.abs(boxes.fields - boxes.form)).toBeLessThan(3);
    await expect(page.locator('.create__blank-title--header')).toHaveCSS('white-space', 'nowrap');
  }
});

test('tablet registry: scroll page, load another page on demand, preserve desktop scroll', async ({ page }) => {
  const requests = await tabletFixtures(page);
  for (const [path, body, rows, endpoint] of [
    ['/carsview', '.cars-body', '.car-row', '/unique-cars'],
    ['/employeesview', '.employees-body', '.employee-row', '/unique-employees'],
  ]) {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto(path);
    await expect(page.locator(rows)).toHaveCount(30);
    await page.waitForTimeout(300);
    expect(requests.filter(r => r.path === endpoint).map(r => r.page)).toEqual([1]);
    expect(await page.locator(body).evaluate(e => e.scrollHeight <= e.clientHeight + 1)).toBe(true);
    await page.locator(rows).first().hover();
    await page.mouse.wheel(0, 10000);
    await expect.poll(() => requests.some(r => r.path === endpoint && r.page === 2)).toBe(true);
    await expect.poll(() => page.locator(rows).count()).toBeGreaterThanOrEqual(60);
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect.poll(() => page.locator(body).evaluate(e => e.scrollHeight > e.clientHeight + 1)).toBe(true);
    await page.setViewportSize({ width: 768, height: 1024 });
    await expect.poll(() => page.locator(body).evaluate(e => e.scrollHeight <= e.clientHeight + 1)).toBe(true);
    expect(await noOverflow(page)).toBe(true);
  }
});

test('phone application: readable inputs, phone/plate keyboards and nested modal retain input', async ({ page }) => {
  await tabletFixtures(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/new-application');
  await page.getByTestId('picker-add').click();
  await page.locator('.picker-add-menu__item').filter({ hasText: 'Сотрудники' }).click();
  const lastName = page.getByPlaceholder('Введите фамилию', { exact: true });
  const passport = page.getByPlaceholder('Введите паспортные данные', { exact: true });
  await lastName.fill('Учебный');
  await passport.fill('0000 000000');
  await expect(lastName).toHaveCSS('font-size', '16px');
  const phone = page.getByPlaceholder('Номер телефона', { exact: true });
  await expect(phone).toHaveAttribute('type', 'tel');
  await expect(phone).toHaveAttribute('inputmode', 'tel');
  await phone.fill('79990000000');
  await expect(phone).toHaveValue('+7 (999) 000 00-00');
  await page.setViewportSize({ width: 390, height: 420 });
  await page.getByTestId('ob-form-existing').click();
  await expect(page.locator('.modal-overlay:visible')).toHaveCount(1);
  await page.keyboard.press('Escape');
  await expect(page.locator('.modal-overlay:visible')).toHaveCount(0);
  await expect(lastName).toHaveValue('Учебный');
  await expect(passport).toHaveValue('0000 000000');
  await page.clock.setFixedTime(new Date(Date.parse(CLOCK) + 1000));
  await page.getByTestId('picker-add').click();
  await page.locator('.picker-add-menu__item').filter({ hasText: 'Автозаявка' }).click();
  const numbers = page.locator('.number__input:not([readonly])');
  await expect(numbers).toHaveCount(4);
  await expect(numbers.nth(0)).toHaveAttribute('inputmode', 'text');
  await expect(numbers.nth(1)).toHaveAttribute('inputmode', 'numeric');
  for (const [i, value] of ['А', '123', 'АА', '77'].entries()) await numbers.nth(i).fill(value);
  expect(await numbers.evaluateAll(es => es.map(e => e.value))).toEqual(['А', '123', 'АА', '77']);
  expect(await noOverflow(page)).toBe(true);
});

test('tablet passage tables: long lists use page scrolling in both orientations', async ({ page }) => {
  await tabletFixtures(page);
  for (const path of ['/table/layout-cars', '/table/layout-people']) {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto(path);
    await expect(page.locator('.selected-table-card .items-body .rt-row')).toHaveCount(45);
    await expect(page.locator('.selected-table-card .item-row').last()).toHaveCSS('opacity', '1');
    for (const viewport of [{ width: 1024, height: 768 }, { width: 768, height: 1024 }]) {
      await page.setViewportSize(viewport);
      await expect.poll(() => page.locator('.selected-table-card').evaluate(e => e.clientHeight)).toBeGreaterThan(575);
      expect(await page.locator('.selected-table-card .items-body').evaluate(e => e.scrollHeight <= e.clientHeight + 1)).toBe(true);
      expect(await noOverflow(page)).toBe(true);
      await expect(page.locator('.tables__header')).toBeVisible();
    }
  }
});

test('phone consent: document scroll reaches 100% and enables agreement', async ({ page }) => {
  await tabletFixtures(page, { consent: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/new-application');
  const checkbox = page.locator('.pdc-modal input[type=checkbox]');
  await expect(checkbox).toBeDisabled();
  await page.locator('.pdc-modal__columns').hover();
  await page.mouse.wheel(0, 20000);
  await expect(checkbox).toBeEnabled();
  await expect(page.locator('.pdc-progress-count')).toHaveText('100%');
  await checkbox.check();
  await expect(page.getByRole('button', { name: 'Подтверждаю согласие', exact: true })).toBeEnabled();
  expect(await noOverflow(page)).toBe(true);
});

test('phone mandatory password: readable input and reachable actions on short viewport', async ({ page }) => {
  await tabletFixtures(page, { password: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/new-application');
  const modal = page.getByTestId('change-password-modal');
  await expect(modal).toBeVisible();
  const input = page.getByPlaceholder('Придумайте новый пароль', { exact: true });
  await expect(input).toHaveCSS('font-size', '16px');
  await input.fill('SyntheticNew-2030!');
  await page.setViewportSize({ width: 390, height: 420 });
  await page.locator('.cp-form').hover();
  await page.mouse.wheel(0, 1500);
  const button = page.getByRole('button', { name: 'Сменить пароль', exact: true });
  await expect.poll(() => button.evaluate(e => e.getBoundingClientRect().bottom <= innerHeight)).toBe(true);
  await page.keyboard.press('Escape');
  await expect(modal).toBeVisible();
  await expect(input).toHaveValue('SyntheticNew-2030!');
  expect(await noOverflow(page)).toBe(true);
});
