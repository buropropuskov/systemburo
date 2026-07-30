const { test } = require('@playwright/test');
const { loginAsSuperAdmin, E2E_PREFIX } = require('./permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

function headers(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

// /system-tables вкладывает таблицу в {table:{...}, fields:[...]} ПОВЕРХ envelope,
// поэтому table_type лежит на вложенном объекте, а не на элементе списка.
function unwrapTable(item) {
  return (item && item.table) || item;
}

/**
 * Первая таблица типа cars/people. Только они поддерживают корзину и показывают
 * фильтры справочников; место разгрузки есть лишь у cars, поэтому cars в приоритете.
 *
 * Чужие фикстуры (префикс e2e_) пропускаем намеренно: спеки разных файлов идут в
 * параллельных воркерах на одной базе, и подхваченная чужая таблица уедет в архив
 * прямо посреди теста - её удалит тот, кто её создал.
 */
async function findFilterableTable(request, token) {
  const res = await request.get(`${API_BASE}/system-tables`, { headers: headers(token) });
  // Сорванный запрос - это провал, а не «таблиц нет»: вернув null, спека ушла бы
  // в skip и зелёный прогон скрыл бы недоступный список таблиц.
  if (!res.ok()) throw new Error(`GET /system-tables failed: ${res.status()}`);
  const tables = ((await res.json()).data || [])
    .map(unwrapTable)
    .filter((t) => t && !String(t.name || '').startsWith(E2E_PREFIX));
  return tables.find((t) => t.table_type === 'cars') || tables.find((t) => t.table_type === 'people') || null;
}

async function createCarsTable(request, token) {
  // Имя уникально на воркер: соседний воркер той же базы заводит свою фикстуру.
  const name = `${E2E_PREFIX}tbl_${process.pid}_${Date.now()}`;
  const res = await request.post(`${API_BASE}/system-tables`, {
    headers: headers(token),
    data: { name, display_name: 'E2E таблица', table_type: 'cars' },
  });
  if (!res.ok()) return null;
  const id = (await res.json()).data?.id;
  return id ? { id, name, table_type: 'cars' } : null;
}

/**
 * Даёт тесту таблицу cars/people: берёт существующую, а если таблиц нет вовсе -
 * заводит свою и убирает за собой.
 *
 * Фикстура нужна именно для чистого стека: `cmd/seed` системных таблиц не создаёт,
 * поэтому в CI без неё спека уходила бы в skip там, где гоняется на каждый PR.
 * DELETE у таблиц мягкий (is_active=false), но накопления архивных строк это не
 * даёт: создание срабатывает только когда таблиц нет ни одной, то есть на пустой
 * эфемерной базе - в живой БД (staging, dev с данными) всегда берётся готовая.
 *
 * @param {import('@playwright/test').APIRequestContext} request
 * @param {(table: {name: string, table_type: string}, token: string) => Promise<void>} run
 */
async function withSystemTable(request, run) {
  const token = await loginAsSuperAdmin(request);
  const existing = await findFilterableTable(request, token);
  const created = existing ? null : await createCarsTable(request, token);
  test.skip(!existing && !created, 'нет таблицы cars/people и создать её не удалось');

  try {
    await run(existing || created, token);
  } finally {
    if (created) {
      await request.delete(`${API_BASE}/system-tables/${created.id}`, { headers: headers(token) });
    }
  }
}

module.exports = { findFilterableTable, withSystemTable, unwrapTable };
