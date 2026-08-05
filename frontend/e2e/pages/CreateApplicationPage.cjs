const { expect } = require('@playwright/test');
const { EmployeeFormSection } = require('./EmployeeFormSection');

/**
 * POM формы подачи заявки (CreateApplication.vue). `/submit-form` - редирект на
 * `/new-application`, где форма и живёт.
 *
 * Вложение выбирается не списком типов, а кнопкой «Добавить» в нужной категории
 * BlankSelector: тип (people/cars/items) наследуется от шаблона этой категории.
 */
class CreateApplicationPage {
  constructor(page) {
    this.page = page;
    this.submitButton = page.getByTestId('create-app-button-submit');
    this.consentCheckbox = page.getByTestId('create-app-consent-checkbox');
    this.attachmentForm = page.getByTestId('ob-app-form');
    this.categories = page.locator('.category');
    this.phoneInput = page.getByPlaceholder('Номер телефона');
    this.recipientChips = page.locator('.recipient-chip__name');
    this.quickDatesButton = page.locator('button.qd-trigger');
    this.quickDateItems = page.locator('.qd-item');
    this.timeInputs = page.locator('input.input__time');
    // Промежуточный шаг перед отправкой: новых людей и машины предлагают сохранить
    // в справочник. Для сценария привязка не нужна - уходим кнопкой «без привязки».
    this.skipBindingButton = page.locator('button.btn.skip-btn');
    this.successNumber = page.locator('.application-number .number');
    this.successCloseButton = page.locator('button.btn.close-btn');
    this.employeeForm = new EmployeeFormSection(page.locator('.create__form'));
  }

  async goto() {
    await this.page.goto('/submit-form');
  }

  async expectLoaded() {
    await expect(this.categories.first()).toBeVisible();
  }

  async addAttachment(categoryTitle) {
    // Заголовок категории выводится в верхнем регистре, hasText регистрозависим.
    await this.categories.filter({ hasText: categoryTitle.toUpperCase() }).locator('.add-btn').click();
    await expect(this.attachmentForm).toBeVisible();
  }

  /** Быстрый выбор даты: «Сегодня» - однодневная заявка на текущую дату. */
  async pickQuickDate(label) {
    await this.quickDatesButton.first().click();
    await this.quickDateItems.filter({ hasText: label }).first().click();
    await expect(this.quickDateItems.first()).toBeHidden();
  }

  async setTimeRange(from, to) {
    await this.timeInputs.nth(0).fill(from);
    await this.timeInputs.nth(1).fill(to);
  }

  /**
   * Отправляет заявку и возвращает её номер из модалки успеха.
   * @returns {Promise<string>}
   */
  async submitAndGetNumber() {
    await this.consentCheckbox.check();
    await expect(this.submitButton).toBeEnabled();
    await this.submitButton.click();
    await this.skipBindingButton.first().click();
    await expect(this.successNumber).toBeVisible();
    const number = (await this.successNumber.textContent()).trim();
    await this.successCloseButton.first().click();
    return number;
  }
}

module.exports = { CreateApplicationPage };
