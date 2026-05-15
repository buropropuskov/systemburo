class UserPermissionsModal {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.user-permissions-modal, .modal').filter({ hasText: /Роль и группы|Права пользователя/ }).first();
    this.roleSelect = this.root.getByRole('combobox', { name: /Роль/ }).first();
    this.groupCheckboxes = this.root.locator('input[type="checkbox"][data-group-id], .group-checkbox');
    this.banToggle = this.root.getByRole('switch', { name: /Забанить|Блокировка/ }).first();
    this.saveButton = this.root.getByRole('button', { name: 'Сохранить' });
    this.cancelButton = this.root.getByRole('button', { name: /Отмена|Закрыть/ });
  }

  async waitForOpen() {
    await this.root.waitFor({ state: 'visible' });
  }

  async waitForClose() {
    await this.root.waitFor({ state: 'hidden' });
  }

  async selectRole(roleCode) {
    await this.roleSelect.selectOption({ value: roleCode }).catch(async () => {
      await this.roleSelect.selectOption({ label: roleCode });
    });
  }

  async toggleGroupById(groupId) {
    const cb = this.root.locator(`input[type="checkbox"][data-group-id="${groupId}"]`);
    await cb.click();
  }

  async save() {
    await this.saveButton.click();
    await this.waitForClose();
  }
}

module.exports = { UserPermissionsModal };
