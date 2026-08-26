class AdminPermissionGroupsPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Группы прав доступа' });
    this.createButton = page.getByTestId('group-add-btn');
    // Master-detail (эталон TableConstructor): слева строки-список, справа детали.
    this.cards = page.getByTestId('group-row');
    this.details = page.getByTestId('group-details');
    this.noSelection = page.locator('.no-selection-message');
    this.emptyState = page.locator('.no-results');

    // Модалка создания/копирования группы (Teleport, radius 30px).
    this.renameModal = page.getByTestId('group-modal');
    this.renameName = page.getByTestId('group-input-name');
    this.renameDescription = page.getByTestId('group-input-description');
    this.renameSubmitCreate = page.getByTestId('group-modal-save');
    this.renameCancel = page.getByTestId('group-modal-cancel');
    this.copyButton = page.getByTestId('group-copy');

    // Inline-редактирование выбранной группы в панели деталей.
    this.detailName = page.getByTestId('group-detail-name');
    this.detailDescription = page.getByTestId('group-detail-description');
    this.saveDetails = page.getByTestId('group-save');
    this.editPermsButton = page.getByTestId('group-edit-perms');
    this.deleteButton = page.getByTestId('group-delete');

    // GroupPermissionsModal - тумблер-дерево прав (data-key + .tgl, aria-pressed).
    this.treeModal = page.getByTestId('group-permissions-modal');
    this.treeSearch = page.getByTestId('group-permissions-search');
    this.treeSave = page.getByTestId('group-permissions-save');
    this.treeCancel = page.getByTestId('group-permissions-cancel');
  }

  /** Тумблер права по ключу (например 'page.cars'). Клик переключает право. */
  treeKey(key) {
    return this.treeModal.locator(`[data-key="${key}"] .tgl`);
  }

  /** Совместимость: тумблер-дерево не сворачивает категории. */
  async expandGroup() {
    /* no-op */
  }

  async goto() {
    await this.page.goto('/admin/permission-groups');
    await this.title.waitFor({ state: 'visible' });
    // Дожидаемся отрисовки списка (строка или пустое состояние), чтобы assert
    // по строке не упирался в таймаут, пока идёт GET /permission-groups.
    await this.page.locator('[data-testid="group-row"], .no-results').first()
      .waitFor({ state: 'visible' });
  }

  /** Строка списка по имени группы. */
  card(name) {
    return this.cards.filter({ hasText: name });
  }

  /** Выбрать группу кликом по строке -> открывается панель деталей. */
  async select(name) {
    await this.card(name).first().click();
    await this.details.waitFor({ state: 'visible' });
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
    // После создания панель деталей авто-открывает редактор прав (openPermissions) -
    // он нам не нужен, закрываем.
    await this.treeModal.waitFor({ state: 'visible', timeout: 4000 }).catch(() => {});
    if (await this.treeModal.isVisible().catch(() => false)) {
      await this.treeCancel.click().catch(() => {});
      await this.treeModal.waitFor({ state: 'hidden' }).catch(() => {});
    }
  }

  async togglePermissionKey(key) {
    await this.treeKey(key).click();
  }

  /** Выбрать группу и открыть редактор прав (GroupPermissionsModal). */
  async clickEditTree(name) {
    await this.select(name);
    await this.editPermsButton.click();
    await this.treeModal.waitFor({ state: 'visible' });
  }

  /** Выбрать группу и нажать «Удалить» (откроется ConfirmationModal). */
  async clickDelete(name) {
    await this.select(name);
    await this.deleteButton.click();
  }
}

module.exports = { AdminPermissionGroupsPage };
