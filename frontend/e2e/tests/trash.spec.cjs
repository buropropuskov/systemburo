const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { withSystemTable } = require('../helpers/systemTables');
const { TrashPage } = require('../pages/TrashPage');

test.describe('Корзина таблицы (#186)', () => {
  test('страница корзины рендерится, модалка истории открывается и закрывается', async ({ page, request }) => {
    await withSystemTable(request, async (table) => {
      await loginAsSuperAdminUI(page);

      const trash = new TrashPage(page);
      await trash.goto(table.name);

      // Страница корзины загрузилась с базовыми контролами.
      await trash.expectLoaded();
      await expect(trash.exportBtn).toBeVisible();
      await expect(trash.clearBtn).toBeVisible();

      // Модалка истории корзины открывается и закрывается.
      await trash.openHistory();
      await expect(trash.historyModal).toContainText('История корзины');
      await trash.closeHistory();
    });
  });

  test('ссылка "Назад" возвращает на страницу таблицы', async ({ page, request }) => {
    await withSystemTable(request, async (table) => {
      await loginAsSuperAdminUI(page);

      const trash = new TrashPage(page);
      await trash.goto(table.name);
      await trash.expectLoaded();

      await trash.backBtn.click();
      await expect(page).toHaveURL(new RegExp(`/table/${table.name}(\\?|$|/)`));
    });
  });
});
