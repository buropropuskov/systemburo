const { test, expect } = require('@playwright/test');
const { loginAsUser } = require('../helpers/auth');
const { CreateApplicationPage } = require('../pages/CreateApplicationPage');

// Helper: go to submit-form and wait for categories to load from API
async function gotoSubmitForm(page) {
  await page.goto('/submit-form');
  try {
    await page.locator('.category').first().waitFor({ state: 'visible', timeout: 15000 });
  } catch {
    await page.reload();
    await page.locator('.category').first().waitFor({ state: 'visible', timeout: 15000 });
  }
}

// Helper: go to submit-form, add first attachment, wait for form
async function gotoSubmitFormWithAttachment(page) {
  await gotoSubmitForm(page);
  await page.locator('.add-btn').first().click();
  await expect(page.locator('.create__form')).toBeVisible();
}

test.describe('Application Creation', () => {
  test('submit-form page loads for authenticated user', async ({ page }) => {
    const username = `e2e_create_load_${Date.now()}`;
    await loginAsUser(page, username);

    await page.goto('/submit-form');
    await expect(page).toHaveURL(/submit-form/);
    await expect(page.locator('.create')).toBeVisible();
  });

  test('blank selector shows categories with add buttons', async ({ page }) => {
    const username = `e2e_create_blanks_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitForm(page);
    await expect(page.locator('.category')).toHaveCount(3);
  });

  test('clicking add button creates attachment and shows form', async ({ page }) => {
    const username = `e2e_create_form_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitFormWithAttachment(page);
    await expect(page.locator('.attachment')).toBeVisible();
  });

  test('consent checkbox is present in form', async ({ page }) => {
    const username = `e2e_create_consent_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitFormWithAttachment(page);
    const createAppPage = new CreateApplicationPage(page);
    await expect(createAppPage.consentCheckbox).not.toBeChecked();
  });

  test('cover letter textarea is present in form', async ({ page }) => {
    const username = `e2e_create_textarea_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitFormWithAttachment(page);
    await expect(page.locator('.form__textarea')).toBeVisible();
  });

  test('date range section is present', async ({ page }) => {
    const username = `e2e_create_dates_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitFormWithAttachment(page);
    await expect(page.locator('.form__info-row')).toBeVisible();
  });

  test('user info section is present in form', async ({ page }) => {
    const username = `e2e_create_userinfo_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitFormWithAttachment(page);
    await expect(page.locator('.create__form')).toBeVisible();
  });

  test('add button creates additional attachment in category', async ({ page }) => {
    const username = `e2e_create_addbtn_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitForm(page);

    await page.locator('.add-btn').first().click();
    const count1 = await page.locator('.attachment').count();

    await page.locator('.add-btn').first().click();
    const count2 = await page.locator('.attachment').count();
    expect(count2).toBe(count1 + 1);
  });

  test('switching between attachments updates selection', async ({ page }) => {
    const username = `e2e_create_switch_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitForm(page);

    // Create two attachments
    await page.locator('.add-btn').first().click();
    await page.locator('.add-btn').first().click();

    const attachments = page.locator('.attachment');
    expect(await attachments.count()).toBe(2);

    await attachments.nth(1).click();
    await expect(attachments.nth(1)).toHaveClass(/selected/);
  });

  test('form placeholder shown when no attachment selected', async ({ page }) => {
    const username = `e2e_create_placeholder_${Date.now()}`;
    await loginAsUser(page, username);

    await gotoSubmitForm(page);
    const form = page.locator('.create__form');
    await expect(form).not.toBeVisible();
  });
});
