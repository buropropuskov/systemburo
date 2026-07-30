const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { TablesPage } = require('../pages/TablesPage');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

// /system-tables вкладывает таблицу в {table:{...}, fields:[...]} ПОВЕРХ envelope,
// поэтому table_type лежит на вложенном объекте, а не на элементе списка.
function unwrapTable(item) {
  return (item && item.table) || item;
}

// Фильтры организации/компании показывает только таблица cars или people, место
// разгрузки - только cars. Берём cars, если есть: на ней виден весь набор.
async function findFilterableTable(request, token) {
  const res = await request.get(`${API_BASE}/system-tables`, { headers: headers(token) });
  if (!res.ok()) return null;
  const tables = ((await res.json()).data || []).map(unwrapTable).filter(Boolean);
  return tables.find((t) => t.table_type === 'cars') || tables.find((t) => t.table_type === 'people') || null;
}

// На чистом seed таблиц нет вообще (cmd/seed их не создаёт), поэтому в CI фикстуру
// заводим сами - иначе спека уходила бы в skip именно там, где гоняется на каждый PR.
async function createFilterableTable(request, token) {
  const name = `e2e_tblfilters_${Date.now()}`;
  const res = await request.post(`${API_BASE}/system-tables`, {
    headers: headers(token),
    data: { name, display_name: 'E2E фильтры', table_type: 'cars' },
  });
  if (!res.ok()) return null;
  const id = (await res.json()).data?.id;
  return id ? { id, name, table_type: 'cars' } : null;
}

async function withFilterableTable(request, run) {
  const token = await loginAsSuperAdmin(request);
  const existing = await findFilterableTable(request, token);
  const created = existing ? null : await createFilterableTable(request, token);
  test.skip(!existing && !created, 'нет таблицы cars/people и создать её не удалось');

  try {
    await run(existing || created);
  } finally {
    if (created) {
      await request.delete(`${API_BASE}/system-tables/${created.id}`, { headers: headers(token) });
    }
  }
}

async function openTable(page, table) {
  const tables = new TablesPage(page);
  await tables.goto(table.name);
  await tables.expectLoaded();
  return tables;
}

test.describe('Фильтры страницы таблицы: мультивыбор (#1398)', () => {
  test('меню фильтра открывается и при пустом выборе показывает «Ничего не выбрано»', async ({ page, request }) => {
    await withFilterableTable(request, async (table) => {
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
    await withFilterableTable(request, async (table) => {
      await loginAsSuperAdminUI(page);
      const tables = await openTable(page, table);

      await tables.openFilterMenu('org');
      const optionCount = await tables.menuItems.count();
      test.skip(optionCount < 2, `в справочнике организаций ${optionCount} записей, для счётчика нужно 2`);

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

  test('сброс выбора возвращает плейсхолдер фильтра', async ({ page, request }) => {
    await withFilterableTable(request, async (table) => {
      await loginAsSuperAdminUI(page);
      const tables = await openTable(page, table);

      await tables.openFilterMenu('org');
      await tables.checkOption(0);
      await expect(tables.getSelectedFilterLabel('org')).not.toHaveText('Все организации');

      await tables.resetFilter();
      await expect(tables.getSelectedFilterLabel('org')).toHaveText('Все организации');
      await expect(tables.clearOption).toHaveText('Ничего не выбрано');
      await expect(tables.clearOption).toBeDisabled();
    });
  });

  test('на мобилке тот же мультивыбор работает внутри bottom-sheet', async ({ page, request }) => {
    await withFilterableTable(request, async (table) => {
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
