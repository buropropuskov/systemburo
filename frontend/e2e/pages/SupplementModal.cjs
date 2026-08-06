const { expect } = require('@playwright/test');
const { EmployeeFormSection } = require('./EmployeeFormSection');

/**
 * POM модалки дополнения заявки (SupplementModal.vue, #1685).
 *
 * Всё ищется от корня модалки: под ней открыта карточка заявки со своими полями.
 * Комментарий к решению по раунду называется иначе (supplement-decision-comment) -
 * раньше оба поля носили одно имя, и локатор указывал на два разных поля сразу.
 */
class SupplementModal {
  constructor(page) {
    this.page = page;
    this.root = page.getByTestId('supplement-modal');
    this.period = this.root.getByTestId('supplement-period');
    this.attachmentDropdown = this.root.getByTestId('supplement-attachment');
    this.comment = this.root.getByTestId('supplement-comment');
    this.submitButton = this.root.getByTestId('supplement-submit');
    this.closeButton = this.root.getByTestId('supplement-button-close');
    // Меню BaseDropdown телепортируется в body, вне корня модалки.
    this.dropdownMenu = page.locator('.base-dropdown__menu');
    this.employeeForm = new EmployeeFormSection(this.root);
  }

  async expectOpen() {
    await expect(this.root).toBeVisible();
  }

  async selectAttachment(label) {
    await this.attachmentDropdown.locator('.base-dropdown__button').click();
    await expect(this.dropdownMenu).toBeVisible();
    await this.dropdownMenu.locator('.base-dropdown__item').filter({ hasText: label }).first().click();
    await expect(this.employeeForm.lastNameInput).toBeVisible();
  }

  async submit(comment) {
    await this.comment.fill(comment);
    await expect(this.submitButton).toBeEnabled();
    await this.submitButton.click();
    await expect(this.root).toBeHidden();
  }
}

module.exports = { SupplementModal };
