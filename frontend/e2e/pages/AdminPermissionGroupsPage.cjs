class AdminPermissionGroupsPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Группы прав' });
    this.createButton = page.getByRole('button', { name: '+ Создать группу' });
    this.cards = page.locator('.permission-groups .cards .card');
    this.emptyState = page.locator('.permission-groups .empty');

    this.renameModal = page.locator('.form-modal').filter({ hasText: /Новая группа|Переименовать/ });
    this.renameName = this.renameModal.getByRole('textbox', { name: 'Название' });
    this.renameDescription = this.renameModal.getByRole('textbox', { name: 'Описание' });
    this.renameSave = this.renameModal.getByRole('button', { name: 'Сохранить' });
    this.renameCancel = this.renameModal.getByRole('button', { name: 'Отмена' });

    this.treeModal = page.locator('.permission-tree-modal');
    this.treeSearch = this.treeModal.getByRole('textbox', { name: /Поиск/ });
    this.treeSave = this.treeModal.getByRole('button', { name: 'Сохранить' });
    this.treeCancel = this.treeModal.getByRole('button', { name: 'Отмена' });
  }

  async goto() {
    await this.page.goto('/admin/permission-groups');
    await this.title.waitFor({ state: 'visible' });
  }

  card(name) {
    return this.cards.filter({ hasText: name });
  }

  async openCreate() {
    await this.createButton.click();
    await this.renameModal.waitFor({ state: 'visible' });
  }

  async createGroup({ name, description = '' }) {
    await this.openCreate();
    await this.renameName.fill(name);
    if (description) await this.renameDescription.fill(description);
    await this.renameSave.click();
    await this.renameModal.waitFor({ state: 'hidden' });
  }

  async togglePermissionKey(key) {
    const item = this.treeModal.locator(`[data-key="${key}"]`).first();
    if (await item.count()) {
      await item.click();
      return;
    }
    await this.treeModal.getByText(key, { exact: false }).first().click();
  }
}

module.exports = { AdminPermissionGroupsPage };
