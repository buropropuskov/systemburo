const { expect } = require('@playwright/test');

class CabinetPage {
  constructor(page) {
    this.page = page;
    this.root = page.getByTestId('cabinet-page');
    this.username = page.getByTestId('cabinet-text-username');
    // Список своих заявок (UserApplications.vue). Строка без data-testid, номер
    // печатается внутри неё.
    this.applicationItems = page.locator('.application-item');
  }

  async goto() {
    await this.page.goto('/personal-cabinet');
  }

  applicationItem(applicationNumber) {
    return this.applicationItems.filter({ hasText: applicationNumber });
  }

  /** Открывает карточку своей заявки - только отсюда автору доступна кнопка «Дополнить». */
  async openApplication(applicationNumber) {
    await this.goto();
    const item = this.applicationItem(applicationNumber).first();
    await expect(item).toBeVisible();
    await item.click();
  }
}

module.exports = { CabinetPage };
