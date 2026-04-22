class CarsPage {
  constructor(page) {
    this.page = page;
    this.root = page.getByTestId('cars-page');
    this.addButton = page.getByTestId('cars-view-add-button');
    this.modal = page.getByTestId('cars-view-modal');
    this.modalCloseButton = page.getByTestId('cars-view-modal-close');
    this.formatDropdown = page.getByTestId('cars-view-format-dropdown');
  }

  async goto() {
    await this.page.goto('/carsview');
  }

  getFilterTab(name) {
    return this.page.getByTestId(`filter-tab-${name}`);
  }

  getAllFilterTabs() {
    return this.page.locator('[data-testid^="filter-tab-"]');
  }
}

module.exports = { CarsPage };
