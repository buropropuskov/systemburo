const { expect } = require('@playwright/test');

/**
 * Форма сотрудника (EmployeeForm.vue + EmployeesList.vue). Живёт в двух местах с
 * одинаковой разметкой - на странице подачи заявки и внутри модалки дополнения,
 * поэтому принимает scope: корневой локатор той области, где отрисована. Искать от
 * page нельзя: открытая модалка дополнения держит в DOM обе копии полей.
 *
 * @param {import('@playwright/test').Locator} scope
 */
class EmployeeFormSection {
  constructor(scope) {
    this.scope = scope;
    this.lastNameInput = scope.getByPlaceholder('Введите фамилию');
    this.firstNameInput = scope.getByPlaceholder('Введите имя');
    this.positionInput = scope.getByPlaceholder('Введите должность');
    this.passportInput = scope.getByPlaceholder('Введите паспортные данные');
    this.citizenshipText = scope.locator('.citizenship__dropdown .dropdown__button .button__text');
    this.passageTiles = scope.locator('.passage__item');
    this.addButton = scope.locator('.data__completion button.add-button');
    // Отметка согласия субъекта на обработку персональных данных: без неё «Добавить»
    // остаётся заблокированной. Бюро может убрать поле настройкой вида вложения,
    // поэтому в addEmployee отметка ставится по факту наличия.
    this.pdConsentCheckbox = scope.getByTestId('employee-pd-consent');
    this.rows = scope.locator('.employees-table .table-row.rt-row');
  }

  /** Плитка места прохода выбирается по display_name таблицы: людских таблиц в базе несколько. */
  passageTile(displayName) {
    return this.passageTiles.filter({ hasText: displayName });
  }

  row(lastName) {
    return this.rows.filter({ hasText: lastName });
  }

  /**
   * Заполняет обязательные поля и добавляет строку в список.
   * Гражданство не трогаем - форма подставляет is_default (или единственное) сама.
   */
  async addEmployee({ lastName, firstName = 'Иван', position = 'Монтажник', passport, passageTable }) {
    await this.lastNameInput.fill(lastName);
    await this.firstNameInput.fill(firstName);
    await this.positionInput.fill(position);
    await this.passportInput.fill(passport);
    // Автоподстановка гражданства асинхронна (GET /citizenships): пока она не приехала,
    // кнопка держит плейсхолдер, а «Добавить» отобьётся на обязательном поле.
    await expect(this.citizenshipText.first()).not.toHaveText('Выберите гражданство');

    await this.passageTile(passageTable).first().click();
    await expect(this.passageTile(passageTable).first()).toHaveClass(/passage__item--active/);

    if (await this.pdConsentCheckbox.count() > 0) {
      await this.pdConsentCheckbox.first().check();
    }

    await this.addButton.first().click();
    await expect(this.row(lastName)).toHaveCount(1);
  }
}

module.exports = { EmployeeFormSection };
