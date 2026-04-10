class CabinetPage {
  constructor(page) {
    this.page = page;
    this.root = page.getByTestId('cabinet-page');
    this.username = page.getByTestId('cabinet-text-username');
  }

  async goto() {
    await this.page.goto('/personal-cabinet');
  }
}

module.exports = { CabinetPage };
