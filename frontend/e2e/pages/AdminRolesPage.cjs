class AdminRolesPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Роли пользователей' });
    this.createButton = page.getByTestId('role-add-btn');
    // Master-detail (эталон TableConstructor): слева строки-список, справа детали.
    this.cards = page.getByTestId('role-row');
    this.details = page.getByTestId('role-details');
    this.noSelection = page.locator('.no-selection-message');
    this.emptyState = page.locator('.no-results');

    // Модалка создания/копирования роли (Teleport, radius 30px).
    this.metaModal = page.getByTestId('role-modal');
    this.metaName = page.getByTestId('role-input-name');
    this.metaCode = page.getByTestId('role-input-code');
    this.metaDescription = page.getByTestId('role-input-description');
    this.metaSave = page.getByTestId('role-modal-save');
    this.metaCancel = page.getByTestId('role-modal-cancel');
    this.copyButton = page.getByTestId('role-copy');

    // Inline-редактирование выбранной роли в панели деталей.
    this.detailName = page.getByTestId('role-detail-name');
    this.detailDescription = page.getByTestId('role-detail-description');
    this.saveDetails = page.getByTestId('role-save');
    // Уведомление о сохранении - признак, что запрос прошёл (для правок, не видных в списке).
    this.savedToast = page.locator('.del-card', { hasText: 'Изменения сохранены' });
    this.deleteButton = page.getByTestId('role-delete');

    // Модалка «Права роли»: дефолтные группы (чекбоксы) + точечные права (тумблер-дерево).
    this.permsButton = page.getByTestId('role-perms-btn');
    this.permsModal = page.getByTestId('role-permissions-modal');
    this.permsSave = page.getByTestId('role-permissions-save');
    this.permsCancel = page.getByTestId('role-permissions-cancel');
  }

  async goto() {
    await this.page.goto('/admin/roles');
    await this.title.waitFor({ state: 'visible' });
    await this.page.locator('[data-testid="role-row"], .no-results').first()
      .waitFor({ state: 'visible' });
  }

  /** Строка списка по коду роли. */
  card(code) {
    return this.cards.filter({ has: this.page.locator('code', { hasText: code }) });
  }

  /** Выбрать роль кликом по строке -> открывается панель деталей. */
  async select(code) {
    await this.card(code).first().click();
    await this.details.waitFor({ state: 'visible' });
  }

  async openCreate() {
    await this.createButton.click();
    await this.metaModal.waitFor({ state: 'visible' });
  }

  async fillMeta({ name, code, description }) {
    if (name !== undefined) await this.metaName.fill(name);
    if (code !== undefined) await this.metaCode.fill(code);
    if (description !== undefined) await this.metaDescription.fill(description);
  }

  async submitMeta() {
    await this.metaSave.click();
    await this.metaModal.waitFor({ state: 'hidden' });
  }

  async createRole({ name, code, description }) {
    await this.openCreate();
    await this.fillMeta({ name, code, description });
    await this.submitMeta();
  }

  /**
   * Редактирование имени/описания роли: в master-detail это inline в панели деталей
   * (выбрать строку -> заполнить поля -> «Сохранить»), а не отдельная модалка.
   *
   * После клика метод ждёт, пока новое имя появится в строке списка. Без ожидания тест
   * читал состояние через API сразу за кликом и падал, когда сервер не успевал ответить
   * до следующей строки: сохранение идёт запросом, а список перечитывается уже после
   * него. Ждём то, что видит человек, - обновлённую строку, а не таймер.
   */
  async editMeta(code, { name, description }) {
    await this.select(code);
    if (name !== undefined) await this.detailName.fill(name);
    if (description !== undefined) await this.detailDescription.fill(description);
    await this.saveDetails.click();
    if (name !== undefined) {
      await this.card(code).filter({ hasText: name }).first().waitFor({ state: 'visible' });
    } else {
      // Правка только описания в списке не видна - ждём уведомление о сохранении.
      await this.savedToast.waitFor({ state: 'visible' });
    }
  }

  /** Выбрать роль и нажать «Удалить» (откроется ConfirmationModal). */
  async clickDelete(code) {
    await this.select(code);
    await this.deleteButton.click();
  }

  /** Выбрать роль и открыть модалку прав (группы + точечные права). */
  async openPerms(code) {
    await this.select(code);
    await this.permsButton.click();
    await this.permsModal.waitFor({ state: 'visible' });
  }

  /** Тумблер права по ключу каталога внутри модалки прав. */
  permToggle(key) {
    return this.permsModal.locator(`[data-key="${key}"] .tgl`);
  }

  /** Чекбокс дефолтной группы по id внутри модалки прав. */
  permGroup(id) {
    return this.permsModal.locator(`[data-testid="role-perms-group"][data-group-id="${id}"]`);
  }
}

module.exports = { AdminRolesPage };
