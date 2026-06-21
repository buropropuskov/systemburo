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

    // GroupPermissionsModal - тумблер-дерево прав (стиль карточки прав пользователя).
    // Каждое право - строка с data-key и тумблером .tgl (aria-pressed = вкл/выкл).
    this.treeModal = page.getByTestId('group-permissions-modal');
    this.treeSearch = page.getByTestId('group-permissions-search');
    this.treeSave = page.getByTestId('group-permissions-save');
    this.treeCancel = page.getByTestId('group-permissions-cancel');
  }

  /**
   * Тумблер права по ключу (например 'page.cars'). Клик переключает право.
   */
  treeKey(key) {
    return this.treeModal.locator(`[data-key="${key}"] .tgl`);
  }

  /**
   * Совместимость со старым деревом: в тумблер-дереве категории всегда раскрыты,
   * сворачивания нет -- ничего не делаем.
   */
  async expandGroup() {
    /* no-op: тумблер-дерево не сворачивает категории */
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
    await this.treeKey(key).click();
  }

  /**
   * "Редактировать" в карточке группы открывает редактор прав (GroupPermissionsModal)
   * с тумблер-деревом. Метод ждёт появления модалки.
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
