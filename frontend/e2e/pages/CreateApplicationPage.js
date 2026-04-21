class CreateApplicationPage {
  constructor(page) {
    this.page = page;
    this.submitButton = page.getByTestId('create-app-button-submit');
    this.consentCheckbox = page.getByTestId('create-app-consent-checkbox');
  }

  async goto() {
    await this.page.goto('/submit-form');
  }
}

module.exports = { CreateApplicationPage };
