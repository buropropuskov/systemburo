const { test, expect } = require('@playwright/test');
const { loginAsSuperAdmin } = require('../helpers/permissions');
const { loginAsSuperAdminUI } = require('../helpers/auth');
const { TrashPage } = require('../pages/TrashPage');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

// Находит первую таблицу типа cars/people - только они поддерживают корзину.
// Элемент списка приходит двойной обёрткой {table:{...}, fields:[...]} поверх
// envelope, поэтому table_type лежит на вложенном объекте: без разворачивания
// поиск не находил ничего и оба теста молча уходили в skip.
async function firstTrashableTable(request, token) {
  const res = await request.get(`${API_BASE}/system-tables`, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
  });
  if (!res.ok()) return null;
  const tables = ((await res.json()).data || []).map((item) => (item && item.table) || item);
  return tables.find((t) => t && (t.table_type === 'cars' || t.table_type === 'people')) || null;
}

test.describe('Корзина таблицы (#186)', () => {
  test('страница корзины рендерится, модалка истории открывается и закрывается', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const table = await firstTrashableTable(request, token);
    test.skip(!table, 'нет таблицы типа cars/people в seed-данных');

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

  test('ссылка "Назад" возвращает на страницу таблицы', async ({ page, request }) => {
    const token = await loginAsSuperAdmin(request);
    const table = await firstTrashableTable(request, token);
    test.skip(!table, 'нет таблицы типа cars/people в seed-данных');

    await loginAsSuperAdminUI(page);

    const trash = new TrashPage(page);
    await trash.goto(table.name);
    await trash.expectLoaded();

    await trash.backBtn.click();
    await expect(page).toHaveURL(new RegExp(`/table/${table.name}(\\?|$|/)`));
  });
});
