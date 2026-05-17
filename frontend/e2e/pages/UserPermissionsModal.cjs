class UserPermissionsModal {
  constructor(page) {
    this.page = page;
    // UserPermissionsModal использует .modal-overlay + .modal-content, без специфического класса.
    // Идентифицируем по h3 "Права пользователя «...»".
    this.root = page.locator('.modal-overlay').filter({
      has: page.getByRole('heading', { level: 3, name: /Права пользователя/ }),
    }).first();
    this.roleSelect = this.root.locator('select.lk-select').first();
    this.groupCheckboxes = this.root.locator('.group-row input[type="checkbox"]');
    this.saveButton = this.root.getByRole('button', { name: 'Сохранить' });
    this.cancelButton = this.root.getByRole('button', { name: /Отмена|Закрыть|×/ }).first();
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
