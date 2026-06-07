class AdminPermissionGroupsPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Группы прав' });
    this.createButton = page.getByRole('button', { name: '+ Создать группу' });
    this.cards = page.locator('.permission-groups .cards .card');
    this.emptyState = page.locator('.permission-groups .empty');
    this.loadingIndicator = page.locator('.permission-groups .loading');

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

    // PermissionTreeModal - дерево прав. Используем data-testid (стабильные)
    // потому что CSS-классы пересекаются с другими modal-content в приложении.
    this.treeModal = page.getByTestId('permission-tree-modal');
    this.treeSearch = page.getByTestId('permission-tree-search');
    this.treeSave = page.getByTestId('permission-tree-save');
    this.treeCancel = page.getByTestId('permission-tree-cancel');
  }

  /**
   * Возвращает локатор для tree-item по permission key (например 'page.cars').
   * Гарантировано видим - метод сам разворачивает collapsed-группу если надо.
   */
  treeKey(key) {
    return this.page.getByTestId(`permission-tree-key-${key}`);
  }

  /**
   * Раскрывает группу по prefix (без точки), если она collapsed.
   * Например expandGroup('page') / expandGroup('entity').
   */
  async expandGroup(prefix) {
    const toggle = this.page.getByTestId(`permission-tree-group-toggle-${prefix}`);
    if (await toggle.isVisible({ timeout: 1000 }).catch(() => false)) {
      // Если у заголовка нет класса collapsed - группа уже открыта.
      const collapsed = await toggle.evaluate(el => el.classList.contains('tree-group__header--collapsed')).catch(() => false);
      if (collapsed) await toggle.click();
    }
  }

  async goto() {
    await this.page.goto('/admin/permission-groups');
    await this.title.waitFor({ state: 'visible' });
    // Дожидаемся завершения загрузки списка (спиннер исчез). Без этого assert
    // карточки упирается в 5с-таймаут expect, пока под нагрузкой CI ещё идёт
    // GET /permission-groups - источник флака shard 4 (#413/#437).
    await this.loadingIndicator.waitFor({ state: 'hidden' });
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

  /**
   * Открыть rename-модалку для редактирования имени/описания группы.
   * В AdminPermissionGroups.vue нет отдельной кнопки rename - "Редактировать"
   * открывает PermissionTreeModal. Тест только подтверждает что нажатие
   * корректно открывает дерево.
   */
  async clickEditTree(name) {
    await this.card(name).getByRole('button', { name: 'Редактировать' }).click();
    await this.treeModal.waitFor({ state: 'visible' });
  }

  async clickDelete(name) {
    await this.card(name).getByRole('button', { name: 'Удалить' }).click();
  }
}

module.exports = { AdminPermissionGroupsPage };
