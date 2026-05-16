class ApplicationCenterPage {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.center');
    this.searchInput = page.getByTestId('center-input-search');
    this.resetFiltersButton = page.getByTestId('center-button-reset-filters');
    this.unreadBadge = page.getByTestId('center-badge-unread');
  }

  async goto() {
    await this.page.goto('/center');
  }

  async search(query) {
    await this.searchInput.fill(query);
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
