const { expect } = require('@playwright/test');

// POM страницы таблицы КПП: /table/:tableName.
//
// Фильтры-справочники (организация/компания/место разгрузки) - мультивыбор
// BaseDropdown (#1398). data-testid у десктопного ряда и мобильного bottom-sheet
// общий (`table-sheet-*`): ветки взаимоисключающие по 768px, дублей в DOM нет.
// Меню телепортится в body, поэтому ищется от page, а не от корня дропдауна.
class TablesPage {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.tables');
    this.title = page.locator('.tables__title');
    // Мобильная ветка: вторичные фильтры свёрнуты в кнопку «Фильтр».
    this.filterButton = page.getByTestId('table-filter-btn');
    this.filterSheet = page.locator('.filter-sheet');
    this.menu = page.locator('.base-dropdown__menu');
    this.menuItems = this.menu.locator('.base-dropdown__item');
    // Строка сброса живёт в открытом меню и присутствует всегда: при пустом
    // выборе она disabled с текстом «Ничего не выбрано» (#1405).
    this.clearOption = page.getByTestId('base-dropdown-clear');
  }

  async goto(tableName) {
    await this.page.goto(`/table/${tableName}`);
  }

  async expectLoaded() {
    await expect(this.root).toBeVisible();
    await expect(this.title).toContainText('Таблица');
  }

  // name: org | company | place - хвост data-testid из directoryFilters.
  getFilterDropdown(name) {
    return this.page.getByTestId(`table-sheet-${name}`);
  }

  // Подпись кнопки: плейсхолдер при пустом выборе, имя единственного выбранного,
  // «{summaryLabel}: N» при N > 1.
  getSelectedFilterLabel(name) {
    return this.getFilterDropdown(name).locator('.base-dropdown__text');
  }

  async openFilterMenu(name) {
    await this.getFilterDropdown(name).locator('.base-dropdown__button').click();
    await expect(this.menu).toBeVisible();
  }

  // Отметить пункт в УЖЕ открытом меню. При multiple меню не закрывается,
  // поэтому несколько пунктов отмечаются подряд без повторного открытия.
  async checkOption(index) {
    await this.menuItems.nth(index).click();
  }

  getOptionLabel(index) {
    return this.menuItems.nth(index).locator('.base-dropdown__item-text');
  }

  // Выбор по подписи. Совпадение точное: имена справочников бывают вложены одно
  // в другое («Отдел аренды» / «Отдел аренды и логистики»), и подстрочный матч
  // отметил бы соседний пункт.
  async selectFilterOption(name, optionLabel) {
    await this.openFilterMenu(name);
    await this.menu.getByText(optionLabel, { exact: true }).click();
  }

  async resetFilter() {
    await this.clearOption.click();
  }

  async openFilterSheet() {
    await this.filterButton.click();
    await expect(this.filterSheet).toBeVisible();
  }
}

module.exports = { TablesPage };
