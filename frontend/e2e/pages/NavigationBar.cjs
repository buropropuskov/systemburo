class NavigationBar {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.nav-menu');
    this.centerLink = page.getByTestId('nav-link-center');
    this.cabinetLink = page.getByTestId('nav-link-cabinet');
    this.carsLink = page.getByTestId('nav-link-cars');
    this.employeesLink = page.getByTestId('nav-link-employees');
    this.newsLink = page.getByTestId('nav-link-news');
    this.logoutButton = page.getByTestId('nav-button-logout');
  }

  async navigateTo(link) {
    await this.root.hover();
    await link.click();
  }

  async logout() {
    await this.root.hover();
    await this.logoutButton.click();
  }
}

module.exports = { NavigationBar };
