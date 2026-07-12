class ApplicationCenterPage {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.center');
    this.searchInput = page.getByTestId('center-input-search');
    // Вторичные фильтры (сегодня/подтверждение/статус/теги/сброс) живут в модалке,
    // открываемой кнопкой «Фильтр» (#1097 W3.6) - к ним обращаться после openFilters().
    this.filterButton = page.getByTestId('center-button-filter');
    this.filterModal = page.locator('.filter-modal');
    this.resetFiltersButton = page.getByTestId('center-button-reset-filters');
    this.unreadBadge = page.getByTestId('center-badge-unread');
  }

  async goto() {
    await this.page.goto('/center');
  }

  async search(query) {
    await this.searchInput.fill(query);
  }

  async openFilters() {
    await this.filterButton.click();
    await this.filterModal.waitFor({ state: 'visible' });
  }

  async resetFilters() {
    await this.resetFiltersButton.click();
  }

  getRow(id) {
    return this.page.getByTestId(`center-row-${id}`);
  }

  getAllRows() {
    return this.page.locator('[data-testid^="center-row-"]');
  }

  getStatusButton(status) {
    return this.page.getByTestId(`center-button-status-${status}`);
  }
}

module.exports = { ApplicationCenterPage };
