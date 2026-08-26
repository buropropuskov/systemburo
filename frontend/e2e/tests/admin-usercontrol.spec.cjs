const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { UserControlPage } = require('../pages/UserControlPage');

// Покрытие страницы /admin/users (UserControl) взамен удалённых спеков AdminUsers (#510).
// UserControl - master-detail: список пользователей слева, панель деталей справа.
// Поиск клиентский, поэтому от сидов нужен лишь хотя бы один пользователь в списке.
test.describe('UserControl - страница /admin/users', () => {
  let users;

  test.beforeEach(async ({ page }) => {
    await loginAsSuperAdminUI(page);
    users = new UserControlPage(page);
    await users.goto();
  });

  test('страница открывается у супер-админа со списком пользователей', async () => {
    await users.expectLoaded();
    await expect(users.rows.first()).toBeVisible();
    expect(await users.rows.count()).toBeGreaterThan(0);
    // Футер отражает число строк - супер-админ видит ненулевой список.
    await expect(users.itemsCount).toContainText('Всего пользователей');
    // Пока пользователь не выбран, панель деталей скрыта (no-selection-заглушку убрали в #739).
    await expect(users.editModal).toBeHidden();
  });

  test('поиск сужает список до совпадающих строк', async () => {
    const totalBefore = await users.rows.count();
    expect(totalBefore).toBeGreaterThan(0);

    const login = (await users.firstRowLogin()).trim();
    await users.search(login);

    // Конкретная строка остаётся, общее число не превышает исходное.
    await expect(users.row(login)).toBeVisible();
    const filtered = await users.rows.count();
    expect(filtered).toBeGreaterThan(0);
    expect(filtered).toBeLessThanOrEqual(totalBefore);
    // Каждая оставшаяся строка содержит подстроку запроса в логине.
    const logins = await users.rows.locator('.user-login').allInnerTexts();
    for (const text of logins) {
      expect(text.toLowerCase()).toContain(login.toLowerCase());
    }
  });

  test('поиск без совпадений показывает empty-state', async () => {
    await expect(users.rows.first()).toBeVisible();

    await users.search('zzz_no_such_user_qwerty_0000');

    await expect(users.rows).toHaveCount(0);
    await expect(users.emptyState).toBeVisible();
    await expect(users.emptyState).toContainText('Пользователи не найдены');
  });

  test('выбор пользователя открывает detail-панель редактирования', async () => {
    // Дожидаемся загрузки списка, затем проверяем, что панель деталей пока скрыта.
    await expect(users.rows.first()).toBeVisible();
    await expect(users.editModal).toBeHidden();

    const login = (await users.firstRowLogin()).trim();
    await users.selectUser(login);

    await expect(users.editModal).toBeVisible();
    await expect(users.detailsTitle).toBeVisible();
    // Подзаголовок панели цитирует логин выбранной записи.
    await expect(users.editModal).toContainText(login);
  });
});
