const { expect } = require('@playwright/test');

// POM страницы корзины таблицы (#186): /table/:tableName/trash.
class TrashPage {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.trash-view');
    this.title = page.locator('.trash-title');
    this.backBtn = page.getByTestId('trash-back');
    this.exportBtn = page.getByTestId('trash-export');
    this.clearBtn = page.getByTestId('trash-clear');
    this.restoreBtn = page.getByTestId('trash-restore-selected');
    this.historyBtn = page.getByTestId('trash-history');
    this.historyModal = page.locator('.trash-history-modal');
    this.historyClose = page.getByTestId('trash-history-close');
  }

  async goto(tableName) {
    await this.page.goto(`/table/${tableName}/trash`);
  }

  async expectLoaded() {
    await expect(this.root).toBeVisible();
    await expect(this.title).toContainText('Корзина');
    await expect(this.historyBtn).toBeVisible();
  }

  async openHistory() {
    await this.historyBtn.click();
    await expect(this.historyModal).toBeVisible();
  }

  async closeHistory() {
    await this.historyClose.click();
    await expect(this.historyModal).toBeHidden();
  }
}

module.exports = { TrashPage };
