/**
 * Отбор журнала обращений (раздел мониторинга): что показывает экран и что
 * лежит в адресной строке. Вынесено из RequestsView.vue - чистые функции
 * проверяются без монтирования компонента, а сам файл и без того велик.
 */

/**
 * Колонки, по которым журнал упорядочивается. Имена полей совпадают с тем, что
 * принимает сервер: порядок строк задаёт он, а не клиент - в списке лежит одна
 * страница из сотен тысяч записей.
 */
export const SORTABLE_COLUMNS = [
  { field: 'created_at', label: 'Время', cls: 'time-col' },
  { field: 'method', label: 'Метод', cls: 'method-col' },
  { field: 'url', label: 'URL', cls: 'path-col' },
  { field: 'status', label: 'Статус', cls: 'status-col' },
  { field: 'username', label: 'Пользователь', cls: 'user-col' },
  { field: 'duration', label: 'Отклик', cls: 'duration-col' }
];

export const SORTABLE_FIELDS = SORTABLE_COLUMNS.map(c => c.field);

/** Размеры страницы из выпадающего списка. */
export const PAGE_SIZES = ['20', '50', '100'];

/** Параметры журнала, которые живут в адресной строке. */
export const JOURNAL_QUERY_KEYS = ['search', 'method', 'status', 'user', 'from', 'to', 'sort', 'order', 'page', 'per_page'];

const DEFAULT_SORT = 'created_at';
const DEFAULT_ORDER = 'desc';
const DEFAULT_PER_PAGE = 20;

/**
 * @typedef {object} JournalState
 * @property {string} search
 * @property {string} method
 * @property {string} status
 * @property {string} user
 * @property {string} from
 * @property {string} to
 * @property {string} sort
 * @property {string} order
 * @property {number} page
 * @property {number} perPage
 */

/**
 * Читает отбор из адресной строки. Присланная ссылка открывает тот же экран, а
 * обновление страницы его не теряет.
 * @param {Record<string, string|string[]>} [query]
 * @returns {JournalState}
 */
export function journalStateFromQuery(query = {}) {
  const str = key => String((Array.isArray(query[key]) ? query[key][0] : query[key]) ?? '');

  // Поле сортировки сверяется со списком колонок: чужое имя оставило бы стрелку
  // без заголовка, а сервер всё равно вернул бы порядок по времени.
  const sort = str('sort');
  const known = SORTABLE_FIELDS.includes(sort);
  const page = parseInt(str('page'), 10);
  const perPage = str('per_page');

  return {
    search: str('search'),
    method: str('method').toUpperCase(),
    status: str('status'),
    user: str('user'),
    from: str('from'),
    to: str('to'),
    sort: known ? sort : DEFAULT_SORT,
    order: known && str('order') === 'asc' ? 'asc' : DEFAULT_ORDER,
    page: page > 0 ? page : 1,
    perPage: PAGE_SIZES.includes(perPage) ? parseInt(perPage, 10) : DEFAULT_PER_PAGE
  };
}

/**
 * Складывает отбор обратно в параметры адреса. Значения по умолчанию не
 * пишутся, чтобы ссылка на журнал без фильтров оставалась короткой.
 * @param {JournalState} state
 * @returns {Record<string, string>}
 */
export function journalQueryFromState(state) {
  const query = {};
  if (state.search) query.search = state.search;
  if (state.method) query.method = state.method;
  if (state.status) query.status = String(state.status);
  if (state.user) query.user = String(state.user);
  if (state.from) query.from = state.from;
  if (state.to) query.to = state.to;
  if (state.sort !== DEFAULT_SORT || state.order !== DEFAULT_ORDER) {
    query.sort = state.sort;
    query.order = state.order;
  }
  if (state.page > 1) query.page = String(state.page);
  if (state.perPage !== DEFAULT_PER_PAGE) query.per_page = String(state.perPage);
  return query;
}

/**
 * Новый адрес с учётом текущего: свои ключи перезаписываются, чужие (deep-link
 * соседнего экрана) остаются. Возвращает null, когда менять нечего - лишний
 * replace на каждом обновлении списка ни к чему.
 * @param {Record<string, string>} current
 * @param {JournalState} state
 * @returns {Record<string, string>|null}
 */
export function mergeJournalQuery(current, state) {
  const desired = journalQueryFromState(state);
  const next = { ...current };
  JOURNAL_QUERY_KEYS.forEach(key => {
    if (desired[key] === undefined) delete next[key];
    else next[key] = desired[key];
  });

  const same = Object.keys(next).length === Object.keys(current).length
    && Object.keys(next).every(key => next[key] === current[key]);
  return same ? null : next;
}
