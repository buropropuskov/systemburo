const { expect } = require('@playwright/test');

/**
 * POM таблицы проходной для людей (PeopleTable.vue на роуте /table/:tableName).
 * Отдельно от TablesPage: тот про фильтры-справочники и общий каркас страницы, здесь
 * нужны строки допущенных.
 *
 * Строка попадает сюда, только когда сотрудник активирован (`employees.status = 1`),
 * его вложение привязано к этой таблице и заявка согласована и в работе. Строки
 * непринятого раунда дополнения бэк намеренно не поднимает - на этом и держится
 * сценарий #1685.
 */
class PeopleTablePage {
  constructor(page) {
    this.page = page;
    this.root = page.getByTestId('people-table');
    // Заголовок таблицы использует те же классы колонок, что и данные, поэтому
    // колонку берём строго изнутри .item-row - иначе в выборку попадёт «Фамилия».
    this.rows = this.root.locator('.item-row');
    this.lastNameCells = this.rows.locator('.col.last-name-col');
  }

  async goto(tableName) {
    await this.page.goto(`/table/${tableName}`);
    await expect(this.root).toBeVisible();
  }

  row(lastName) {
    return this.rows.filter({ hasText: lastName });
  }

  /**
   * Полный состав допущенных: проверять надо именно перечнем, а не наличием одной
   * строки. Отсутствие второй фамилии само по себе доказывается только вместе с
   * тем, что первая на месте, а всего строк ровно столько, сколько ожидалось.
   */
  async expectAdmitted(lastNames) {
    await expect(this.rows).toHaveCount(lastNames.length);
    for (const lastName of lastNames) {
      await expect(this.row(lastName)).toHaveCount(1);
    }
  }
}

module.exports = { PeopleTablePage };
