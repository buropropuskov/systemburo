class AdminPermissionGroupsPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Группы прав' });
    this.createButton = page.getByRole('button', { name: '+ Создать группу' });
    this.cards = page.locator('.permission-groups .cards .card');
    this.emptyState = page.locator('.permission-groups .empty');

    // Модалка имеет класс .rename-modal в AdminPermissionGroups.vue,
    // h3 - "Новая группа прав" (create) или "Имя и описание" (edit).
    this.renameModal = page.locator('.rename-modal');
    this.renameName = this.renameModal.getByRole('textbox', { name: 'Название' });
    this.renameDescription = this.renameModal.getByRole('textbox', { name: /Описание/ });
    // Кнопка submit зависит от режима: create → "Создать и редактировать ключи",
    // edit → "Сохранить".
    this.renameSubmitCreate = this.renameModal.getByRole('button', { name: /Создать/ });
    this.renameSubmitEdit = this.renameModal.getByRole('button', { name: 'Сохранить' });
    this.renameCancel = this.renameModal.getByRole('button', { name: 'Отмена' });

    // После create открывается PermissionTreeModal - дерево прав. Тест
    // создания группы не должен его заполнять, просто закрыть.
    this.treeModal = page.locator('.permission-tree-modal');
    this.treeSearch = this.treeModal.getByRole('textbox', { name: /Поиск/ });
    this.treeSave = this.treeModal.getByRole('button', { name: 'Сохранить' });
    this.treeCancel = this.treeModal.getByRole('button', { name: /Отмена|Закрыть/ });
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
    await this.renameSubmitCreate.click();
    await this.renameModal.waitFor({ state: 'hidden' });
    // После клика "Создать и редактировать ключи" открывается treeModal.
    // Закрываем его (cancel), нам нужна только сама группа.
    if (await this.treeModal.isVisible().catch(() => false)) {
      await this.treeCancel.click().catch(() => {});
      await this.treeModal.waitFor({ state: 'hidden' }).catch(() => {});
    }
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
