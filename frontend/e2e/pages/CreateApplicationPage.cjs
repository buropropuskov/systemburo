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
    this.oneDayCheckbox = page.locator('.one-day input.one-day__checkbox');
    this.dateInputs = page.locator('input.input__date');
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

  /**
   * Срок действия вложения диапазоном, обе даты в формате дд.мм.гггг.
   *
   * Быстрый выбор «Сегодня» тут намеренно не используется, хотя он в один клик: он
   * ставит однодневную заявку на текущую дату, а таблица поста показывает только
   * пропуска, действующие сегодня (CURRENT_DATE BETWEEN entry_date_from AND
   * entry_date_to). Прогон, начавшийся под полночь и перешагнувший её, увидел бы
   * пустую таблицу и упал бы при исправной фиче. Диапазон в два дня это снимает.
   *
   * Календарь открывается по фокусу и накрывает страницу оверлеем - закрываем его
   * Escape, иначе следующий клик уйдёт в оверлей.
   */
  async setDateRange(from, to) {
    // Свежее вложение приходит с isOneDay: false, то есть парой полей. Не чиним
    // состояние молча: сменится дефолт формы - тест обязан это показать, а не
    // подстроиться и увести срок обратно в один день.
    await expect(this.oneDayCheckbox).not.toBeChecked();
    await expect(this.dateInputs).toHaveCount(2);

    await this.dateInputs.nth(0).fill(from);
    await this.page.keyboard.press('Escape');
    await this.dateInputs.nth(1).fill(to);
    await this.page.keyboard.press('Escape');

    // Прошедшую дату форма молча стирает на blur - проверяем, что обе на месте.
    await expect(this.dateInputs.nth(0)).toHaveValue(from);
    await expect(this.dateInputs.nth(1)).toHaveValue(to);
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
