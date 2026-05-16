class HeaderBar {
  constructor(page) {
    this.page = page;
    this.submitAppButton = page.getByTestId('header-button-submit-app');
    this.feedbackButton = page.getByTestId('header-button-feedback');
  }
}

module.exports = { HeaderBar };
