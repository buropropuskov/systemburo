/**
 * POM страницы /admin/users (#510) - кабинетный UserControl, поднятый в
 * UserControlView. Master-detail: слева список .user-item, справа панель
 * .user-details-panel (появляется по клику на строку). Поиск клиентский -
 * фильтрует уже загруженный список без запроса к API.
 */
class UserControlPage {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.user-management');
    this.title = page.getByRole('heading', { name: 'Учётные записи пользователей' });
    this.searchInput = this.root.locator('.search .search__input');
    this.createButton = this.root.getByRole('button', { name: 'Создать' });

    this.rows = this.root.locator('.user-item');
    this.itemsCount = this.root.locator('.users-footer .items-count');

    // .no-users рендерится при пустом filteredUsers: при активном поиске текст
    // "Пользователи не найдены", без поиска - "Пользователи отсутствуют".
    this.emptyState = this.root.locator('.no-users');

    // Редактирование переехало в модалку (#739): клик по строке открывает BaseModal
    // c content-class user-edit-modal (телепортируется на body - ищем от page).
    this.editModal = page.locator('.user-edit-modal');
    this.detailsTitle = this.editModal.getByRole('heading', { name: 'Редактирование' });
  }

  async goto() {
    await this.page.goto('/admin/users');
    await this.title.waitFor({ state: 'visible' });
  }

  row(username) {
    return this.rows.filter({ has: this.page.locator('.user-login', { hasText: username }) });
  }

  async search(term) {
    await this.searchInput.fill(term);
  }

  async clearSearch() {
    await this.searchInput.fill('');
  }

  async selectUser(username) {
    await this.row(username).click();
    await this.editModal.waitFor({ state: 'visible' });
  }

  async firstRowLogin() {
    const shown = await this.rows.first().locator('.user-login').innerText();
    // В списке логин подписан с собачкой (#1567), а искать и сравнивать надо сам логин.
    return shown.replace(/^\s*@/, '');
  }

  async expectLoaded() {
    await this.title.waitFor({ state: 'visible' });
    await this.root.waitFor({ state: 'visible' });
  }
}

module.exports = { UserControlPage };
