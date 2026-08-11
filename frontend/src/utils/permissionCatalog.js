/**
 * Общие операции над каталогом прав (GET /permissions/catalog).
 *
 * Каталог приходит плоским массивом узлов {key, display_name, category,
 * super_only, children}. Редакторы прав роли, группы и пользователя работают с
 * ним одинаково: разворачивают в плоский список для расчёта состояний и
 * фильтруют поиском -- до #1880 обе функции лежали копиями в
 * RolePermissionsModal и GroupPermissionsModal.
 */

/**
 * Глаголы прав системных таблиц (ключ table.<slug>.<verb>) и подписи действий.
 * Зеркало tableVerbs из internal/services/permission_service.go; порядок здесь =
 * порядок отображения на бэкенде. Состав и подписи сверяет
 * utils/__tests__/permissionCatalogVerbs.spec.js, читая сам Go-файл: без этого
 * новый глагол в Go молча выпал бы в «прочее».
 */
export const TABLE_VERB_TITLES = {
  view: 'Доступ к таблице',
  entry: 'Отметка въезда/входа',
  exit: 'Отметка выезда/выхода',
  detail: 'Открытие карточки из таблицы',
  history: 'История таблицы',
  versions: 'Сохранённые версии',
  export: 'Экспорт',
  report: 'Отчёт по проходам',
  trash: 'Корзина',
  delete: 'Удаление записи',
};

/** Порядок глаголов в UI. */
export const TABLE_VERB_ORDER = Object.keys(TABLE_VERB_TITLES);

/**
 * Префикс ключей прав системных таблиц. Зеркало PrefixTable из
 * internal/services/permission_keys.go: бэкенд считает валидным для назначения
 * любой ключ с этим префиксом, не сверяясь с каталогом.
 */
export const TABLE_KEY_PREFIX = 'table.';

/**
 * Плоский список узлов каталога вместе с детьми (один уровень вложенности --
 * глубже бэкенд не отдаёт).
 * @param {Array} catalog
 * @returns {Array}
 */
export function flattenCatalog(catalog) {
  const flat = [];
  for (const node of catalog || []) {
    flat.push(node);
    for (const child of node.children || []) flat.push(child);
  }
  return flat;
}

/**
 * Фильтр каталога по подстроке: имя права, ключ или название категории.
 * Пустой запрос возвращает исходный массив как есть.
 * @param {Array} catalog
 * @param {string} query
 * @returns {Array}
 */
export function filterCatalog(catalog, query) {
  const q = (query || '').trim().toLowerCase();
  if (!q) return catalog;
  const match = (n) =>
    (n.display_name || '').toLowerCase().includes(q) || (n.key || '').toLowerCase().includes(q);
  const out = [];
  for (const node of catalog || []) {
    const kids = (node.children || []).filter(match);
    if (match(node) || (node.category || '').toLowerCase().includes(q)) {
      out.push(node);
    } else if (kids.length) {
      out.push({ ...node, children: kids });
    }
  }
  return out;
}

/**
 * Слаг и глагол из ключа права таблицы -- разбор чисто структурный, состав
 * глаголов не проверяется. Нужен там, где право надо отнести к своей таблице
 * даже с незнакомым глаголом: слаг известен из ключа, а подпись возьмётся от
 * бэкенда.
 *
 * @param {string} key
 * @returns {{slug: string, verb: string}|null}
 */
export function parseTableKey(key) {
  if (!key || !key.startsWith(TABLE_KEY_PREFIX)) return null;

  const rest = key.slice(TABLE_KEY_PREFIX.length);
  const dot = rest.lastIndexOf('.');
  if (dot <= 0 || dot === rest.length - 1) return null;

  return { slug: rest.slice(0, dot), verb: rest.slice(dot + 1) };
}

/**
 * Полный разбор права системной таблицы.
 *
 * Слаг и глагол берутся из самого ключа, а имя таблицы -- отрезанием от
 * display_name суффикса «: <название действия>». Именно так, а не split по
 * первому двоеточию: в имени таблицы бывает своя пунктуация («ПОСТ №72 (АВТО)»),
 * а суффикс известен точно. Не совпал суффикс или незнаком глагол -- возвращаем
 * null: имя таблицы и короткая подпись действия отсюда не следуют. Отнести такое
 * право к своей таблице всё равно можно -- по parseTableKey.
 *
 * @param {{key?: string, display_name?: string}} node
 * @returns {{slug: string, verb: string, verbTitle: string, tableName: string}|null}
 */
export function parseTablePermission(node) {
  const parsed = parseTableKey(node?.key || '');
  if (!parsed) return null;

  const { slug, verb } = parsed;
  const verbTitle = TABLE_VERB_TITLES[verb];
  if (!verbTitle) return null;

  const suffix = `: ${verbTitle}`;
  const displayName = node.display_name || '';
  if (!displayName.endsWith(suffix)) return null;

  const tableName = displayName.slice(0, displayName.length - suffix.length).trim();
  if (!tableName) return null;

  return { slug, verb, verbTitle, tableName };
}
