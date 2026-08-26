const { expect } = require('@playwright/test');

class ApplicationCenterPage {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.center');
    this.searchInput = page.getByTestId('center-input-search');
    // Вторичные фильтры (сегодня/подтверждение/статус/теги/сброс) живут в модалке,
    // открываемой кнопкой «Фильтр» (#1097 W3.6) - к ним обращаться после openFilters().
    this.filterButton = page.getByTestId('center-button-filter');
    this.filterModal = page.locator('.filter-modal');
    this.resetFiltersButton = page.getByTestId('center-button-reset-filters');
    this.unreadBadge = page.getByTestId('center-badge-unread');
  }

  async goto() {
    await this.page.goto('/center');
  }

  async search(query) {
    await this.searchInput.fill(query);
  }

  async openFilters() {
    await this.filterButton.click();
    await this.filterModal.waitFor({ state: 'visible' });
  }

  async resetFilters() {
    await this.resetFiltersButton.click();
  }

  getRow(id) {
    return this.page.getByTestId(`center-row-${id}`);
  }

  getAllRows() {
    return this.page.locator('[data-testid^="center-row-"]');
  }

  /**
   * Находит заявку по номеру и открывает её карточку. Номер уникален, поэтому поиска
   * достаточно - id заявки тесту знать неоткуда, он приходит из модалки успеха.
   */
  async openApplication(applicationNumber) {
    await this.goto();
    await this.search(applicationNumber);
    const row = this.getAllRows().first();
    await expect(row).toBeVisible();
    await row.click();
  }

  // Статусы, подтверждения, теги и справочники (организации/компании/места/проходы)
  // выбираются мультивыбором в дропдауне (#1398): открыть кнопку фильтра, затем
  // отметить пункт по подписи. Меню телепортится в body, поэтому ищем его от page.
  getFilterDropdown(name) {
    return this.page.getByTestId(`center-filter-${name}`);
  }

  async selectFilterOption(name, optionLabel) {
    await this.getFilterDropdown(name).locator('.base-dropdown__button').click();
    const menu = this.page.locator('.base-dropdown__menu');
    await menu.waitFor({ state: 'visible' });
    await menu.locator('.base-dropdown__item', { hasText: optionLabel }).first().click();
  }

  getSelectedFilterLabel(name) {
    return this.getFilterDropdown(name).locator('.base-dropdown__text');
  }
}

module.exports = { ApplicationCenterPage };
