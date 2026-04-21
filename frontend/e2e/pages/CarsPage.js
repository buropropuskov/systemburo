class CarsPage {
  constructor(page) {
    this.page = page;
    this.root = page.getByTestId('cars-page');
    this.modalCloseButton = page.getByTestId('modal-button-close');
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
