class LoginPage {
  constructor(page) {
    this.page = page;
    this.form = page.getByTestId('login-form');
    this.usernameInput = page.getByTestId('login-input-username');
    this.passwordInput = page.getByTestId('login-input-password');
    this.submitButton = page.getByTestId('login-button-submit');
    this.errorMessage = page.getByTestId('login-error-message');
  }

  async goto() {
    await this.page.goto('/');
  }

  async login(username, password) {
    await this.usernameInput.fill(username);
    await this.passwordInput.fill(password);
    await this.submitButton.click();
  }
}

module.exports = { LoginPage };
