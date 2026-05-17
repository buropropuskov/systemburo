class AdminUsersPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Управление пользователями' });
    this.searchInput = page.locator('.admin-users__search-input');
    this.rows = page.locator('.admin-users table tbody tr');
    this.emptyMessage = page.locator('.admin-users').getByText(/Пользователи не найдены|Нет пользователей/);

    // Right-side detail panel когда выбран юзер
    this.detail = page.locator('.admin-users__detail, .admin-users__detail-panel').first();
    this.tabInfo = page.getByRole('button', { name: 'Основные' });
    this.tabPermissions = page.getByRole('button', { name: 'Разрешения' });
    this.permissionsRoleGroupsButton = page.getByRole('button', { name: 'Роль и группы прав' });
    this.resetPasswordButton = page.getByRole('button', { name: 'Сбросить пароль' });
    this.deleteButton = page.getByRole('button', { name: 'Удалить' });

    // Reset password modal
    this.passwordModal = page.getByRole('dialog', { name: /Сброс пароля/ });
    this.passwordInput = this.passwordModal.getByRole('textbox', { name: /пароль/i });
    this.passwordSave = this.passwordModal.getByRole('button', { name: 'Сохранить' });
    this.passwordCancel = this.passwordModal.getByRole('button', { name: /Отмена|Закрыть/ });
  }

  async goto() {
    await this.page.goto('/admin/users');
    await this.title.waitFor({ state: 'visible' });
  }

  async search(query) {
    await this.searchInput.fill(query);
    await this.page.waitForTimeout(200);
  }

  rowByLogin(login) {
    return this.page.locator(`tr:has-text("${login}")`);
  }

  async selectUserByLogin(login) {
    await this.rowByLogin(login).click();
    await this.tabInfo.waitFor({ state: 'visible' });
  }
}

module.exports = { AdminUsersPage };
