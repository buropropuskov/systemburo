const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { apiGet } = require('../helpers/permissions');
const { withSystemTable } = require('../helpers/systemTables');
const { TablesPage } = require('../pages/TablesPage');

async function openTable(page, table) {
  const tables = new TablesPage(page);
  await tables.goto(table.name);
  await tables.expectLoaded();
  return tables;
}

test.describe('Фильтры страницы таблицы: мультивыбор (#1398)', () => {
  test('меню фильтра открывается и при пустом выборе показывает «Ничего не выбрано»', async ({ page, request }) => {
    await withSystemTable(request, async (table) => {
      await loginAsSuperAdminUI(page);
      const tables = await openTable(page, table);

      await expect(tables.getSelectedFilterLabel('org')).toHaveText('Все организации');

      await tables.openFilterMenu('org');
      // Строка сброса присутствует всегда, но при пустом выборе выключена.
      await expect(tables.clearOption).toHaveText('Ничего не выбрано');
      await expect(tables.clearOption).toBeDisabled();
    });
  });

  test('клик по пункту не закрывает меню, второй выбор даёт счётчик «Организация: 2»', async ({ page, request }) => {
    await withSystemTable(request, async (table, token) => {
      // Порог считаем по справочнику, а не по числу пунктов в меню: пустое меню
      // при непустом справочнике - это провал фильтра, и он должен краснеть,
      // а не выглядеть как «мало данных».
      const orgs = await apiGet(request, token, '/organizations');
      test.skip(orgs.length < 2, `в справочнике ${orgs.length} организаций, для счётчика нужно 2`);

      await loginAsSuperAdminUI(page);
      const tables = await openTable(page, table);

      await tables.openFilterMenu('org');
      await expect(tables.menuItems.nth(1)).toBeVisible();

      const firstLabel = (await tables.getOptionLabel(0).innerText()).trim();
      await tables.checkOption(0);

      // Контракт multiple: меню остаётся открытым, кнопка показывает имя единственного.
      await expect(tables.menu).toBeVisible();
      await expect(tables.getSelectedFilterLabel('org')).toHaveText(firstLabel);
      await expect(tables.clearOption).toHaveText('Сбросить выбор (1)');

      await tables.checkOption(1);
      await expect(tables.menu).toBeVisible();
      await expect(tables.getSelectedFilterLabel('org')).toHaveText('Организация: 2');
      await expect(tables.clearOption).toHaveText('Сбросить выбор (2)');
    });
  });

  test('выбранная организация из справочника попадает в подпись, сброс возвращает плейсхолдер', async ({ page, request }) => {
    await withSystemTable(request, async (table, token) => {
      // Имя берём из справочника, а не из меню: так проверяется, что в список
      // фильтра попадают именно записи /organizations.
      const orgs = await apiGet(request, token, '/organizations');
      test.skip(!orgs?.length, 'справочник организаций пуст');

      await loginAsSuperAdminUI(page);
      const tables = await openTable(page, table);

      await tables.selectFilterOption('org', orgs[0].name);
      await expect(tables.getSelectedFilterLabel('org')).toHaveText(orgs[0].name);

      await tables.resetFilter();
      await expect(tables.getSelectedFilterLabel('org')).toHaveText('Все организации');
      await expect(tables.clearOption).toHaveText('Ничего не выбрано');
      await expect(tables.clearOption).toBeDisabled();
    });
  });

  test('на мобилке тот же мультивыбор работает внутри bottom-sheet', async ({ page, request }) => {
    await withSystemTable(request, async (table) => {
      await page.setViewportSize({ width: 390, height: 844 });
      await loginAsSuperAdminUI(page);
      const tables = await openTable(page, table);

      // Десктопного ряда фильтров на мобилке нет - они за кнопкой «Фильтр».
      await expect(tables.getFilterDropdown('org')).toBeHidden();
      await tables.openFilterSheet();

      await tables.openFilterMenu('org');
      const firstLabel = (await tables.getOptionLabel(0).innerText()).trim();
      await tables.checkOption(0);

      await expect(tables.menu).toBeVisible();
      await expect(tables.getSelectedFilterLabel('org')).toHaveText(firstLabel);
    });
  });
});
