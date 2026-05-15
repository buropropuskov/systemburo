class AdminAccessDenialsPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Журнал отказов' });
    this.userFilter = page.getByRole('textbox', { name: /ID пользователя|Пользователь/ });
    this.resourceFilter = page.getByRole('textbox', { name: /Ресурс|Permission/ });
    this.applyFilters = page.getByRole('button', { name: /Применить|Поиск/ });
    this.resetFilters = page.getByRole('button', { name: /Сбросить|Очистить/ });
    this.rows = page.locator('.access-denials table tbody tr');
    this.emptyState = page.locator('.access-denials .empty');
    this.tabActive = page.getByRole('tab', { name: 'Активные' });
    this.tabArchive = page.getByRole('tab', { name: 'Архив' });
  }

  async goto() {
    await this.page.goto('/admin/access-denials');
    await this.title.waitFor({ state: 'visible' });
  }

  async filterByUser(userId) {
    await this.userFilter.fill(String(userId));
    await this.applyFilters.click();
  }

  async expectRowsCount(count) {
    const expect = require('@playwright/test').expect;
    await expect(this.rows).toHaveCount(count);
  }
}

module.exports = { AdminAccessDenialsPage };
