/**
 * Донастройка съёмочного стенда поверх наливки `server fake`.
 *
 * Наливка заводит организации, работников, заявки, чёрные списки, таблицы
 * постов и отметки проходов, но двух вещей не делает: не выдаёт охраннику прав
 * на таблицы поста (проходы в наливке отмечает администратор партии) и не
 * заполняет инструкцию поста. Без первого охранник не увидит ни одной таблицы,
 * без второго снимок инструкции нечего показывать.
 *
 * Всё делается методами программного интерфейса, а не правкой базы: так стенд
 * проходит те же проверки, что и работа руками.
 *
 * Запуск: node Документация/src/shots/setup-stand.mjs [--api=http://localhost:8095/api]
 */

import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const HERE = path.dirname(fileURLToPath(import.meta.url));

/**
 * Права, которые получает охранник на каждую таблицу поста. Полный набор
 * глаголов шире (история, версии, корзина, удаление), но эти четыре - работа
 * администратора, а не поста, и в руководстве охранника не описываются.
 */
const GUARD_TABLE_VERBS = ['view', 'entry', 'exit', 'detail', 'report', 'export'];

const POST_INSTRUCTION =
  'Пропуск по действующей заявке. Сверьте государственный регистрационный знак ' +
  'с записью в таблице и срок действия пропуска. Отметьте въезд сразу после ' +
  'проезда машины, выезд - после того, как машина покинула территорию. ' +
  'Машину, которой нет в таблице, на территорию не пропускать: обратитесь в бюро ' +
  'пропусков по телефону 2-15.';

function arg(name, fallback) {
  const found = process.argv.find((value) => value.startsWith(`--${name}=`));
  return found ? found.slice(name.length + 3) : fallback;
}

async function api(base, token, method, endpoint, body) {
  const response = await fetch(`${base}${endpoint}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${method} ${endpoint} -> ${response.status}: ${text.slice(0, 300)}`);
  }
  return text ? JSON.parse(text) : null;
}

const unwrap = (body) => (body && typeof body === 'object' && 'data' in body ? body.data : body);

async function main() {
  const apiBase = arg('api', 'http://localhost:8095/api');
  const accounts = JSON.parse(await readFile(path.join(HERE, 'accounts.json'), 'utf8'));

  const login = unwrap(
    await api(apiBase, null, 'POST', '/login', {
      username: accounts.superAdmin.username,
      password: accounts.superAdmin.password,
    }),
  );
  const token = login.token;

  /*
   * Список таблиц приходит вложенным: каждый элемент - {table: {...}} поверх
   * общей обёртки ответа. Без разворота имя таблицы читается как undefined, и
   * ключи прав собираются вида table.undefined.view.
   */
  const tables = unwrap(await api(apiBase, token, 'GET', '/system-tables')).map(
    (item) => item.table ?? item,
  );
  const active = tables.filter((table) => table.is_active !== false);
  if (active.length === 0) throw new Error('на стенде нет ни одной таблицы поста');

  // Охранник: персональные разрешения на таблицы постов.
  const users = unwrap(await api(apiBase, token, 'GET', '/users/all'));
  const list = Array.isArray(users) ? users : (users.users ?? users.items ?? []);
  const guard = list.find((user) => user.username === accounts.roles.guard.username);
  if (!guard) throw new Error(`охранник ${accounts.roles.guard.username} не найден`);

  const permissions = [{ key: 'page.tables', value: 'allow' }];
  for (const table of active) {
    for (const verb of GUARD_TABLE_VERBS) {
      permissions.push({ key: `table.${table.name}.${verb}`, value: 'allow' });
    }
  }
  await api(apiBase, token, 'PUT', `/permissions/user/${guard.id}`, { permissions });
  console.log(
    `Охраннику ${guard.username} выдано разрешений: ${permissions.length} (таблиц: ${active.length})`,
  );

  // Таблица «по факту» и инструкция поста - на первой таблице машин.
  const carsTable = active.find((table) => table.table_type === 'cars') ?? active[0];
  await api(apiBase, token, 'PUT', `/system-tables/${carsTable.id}`, {
    show_fact_table: true,
    instruction: POST_INSTRUCTION,
  });
  console.log(`Таблица «${carsTable.display_name}»: включён список по факту, заполнена инструкция`);
}

main().catch((error) => {
  console.error(`Донастройка стенда не удалась: ${error.message}`);
  process.exit(1);
});
