class AdminRolesPage {
  constructor(page) {
    this.page = page;
    this.title = page.getByRole('heading', { name: 'Роли пользователей' });
    this.createButton = page.getByRole('button', { name: '+ Создать роль' });
    this.cards = page.locator('.roles .cards .card');
    this.emptyState = page.locator('.roles .empty');

    this.metaModal = page.locator('.form-modal').filter({ hasText: /Новая роль|Редактировать роль/ });
    this.metaName = this.metaModal.getByRole('textbox', { name: 'Название' });
    this.metaCode = this.metaModal.getByRole('textbox', { name: /^Код/ });
    this.metaDescription = this.metaModal.getByRole('textbox', { name: 'Описание' });
    this.metaSave = this.metaModal.getByRole('button', { name: 'Сохранить' });
    this.metaCancel = this.metaModal.getByRole('button', { name: 'Отмена' });
  }

  async goto() {
    await this.page.goto('/admin/roles');
    await this.title.waitFor({ state: 'visible' });
  }

  card(code) {
    return this.cards.filter({ has: this.page.locator('code', { hasText: code }) });
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

  async openEditMeta(code) {
    await this.card(code).getByRole('button', { name: 'Изменить' }).click();
    await this.metaModal.waitFor({ state: 'visible' });
  }

  async clickDelete(code) {
    await this.card(code).getByRole('button', { name: 'Удалить' }).click();
  }
}

module.exports = { AdminRolesPage };
