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

/** Методы в фильтре. Пустое значение - «все». */
export const METHOD_OPTIONS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'];

/**
 * Значения фильтра по коду ответа. Кроме точных кодов есть классы: оператору
 * важно отличить «клиент шлёт мусор» от «мы падаем», а перебирать коды по одному
 * ради этого он не станет.
 */
export const STATUS_OPTIONS = [
  { value: 'errors', label: 'Все ошибки (4xx и 5xx)' },
  { value: '4xx', label: '4xx: ошибки клиента' },
  { value: '5xx', label: '5xx: ошибки сервера' },
  { value: '200', label: '200 OK' },
  { value: '400', label: '400 Bad Request' },
  { value: '401', label: '401 Unauthorized' },
  { value: '403', label: '403 Forbidden' },
  { value: '404', label: '404 Not Found' },
  { value: '500', label: '500 Server Error' }
];

const STATUS_VALUES = STATUS_OPTIONS.map(o => o.value);

/** Параметры журнала, которые живут в адресной строке. */
export const JOURNAL_QUERY_KEYS = [
  'search', 'method', 'status', 'user', 'from', 'to', 'since', 'min_duration',
  'sort', 'order', 'page', 'per_page'
];

/** Отбор «медленные ответы»: порог в миллисекундах. */
export const SLOW_REQUEST_MS = 1000;

/** Отбор «последний час»: ширина окна в миллисекундах. */
export const LAST_HOUR_MS = 60 * 60 * 1000;

/**
 * Быстрый отбор одной кнопкой. Каждый пресет выражается обычными фильтрами и
 * ложится в те же ключи адреса: ссылка на «медленные за час» открывается у
 * соседа тем же экраном.
 */
export const JOURNAL_PRESETS = [
  { key: 'errors', label: 'Только ошибки', title: 'Ответы с кодом 400 и выше' },
  { key: 'slow', label: 'Медленнее 1 с', title: 'Ответы дольше секунды; подписки на события сюда не идут' },
  { key: 'hour', label: 'Последний час', title: 'Обращения с момента нажатия минус час' }
];

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
 * @property {string} since
 * @property {string} minDuration
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
  const status = str('status');
  const since = str('since');
  const minDuration = str('min_duration');

  return {
    search: str('search'),
    method: str('method').toUpperCase(),
    // Чужое значение отбрасывается: код ответа уходит в запрос числом, и мусор
    // из ссылки вернулся бы отказом вместо списка.
    status: STATUS_VALUES.includes(status) ? status : '',
    user: str('user'),
    from: str('from'),
    to: str('to'),
    since: Number.isNaN(Date.parse(since)) ? '' : since,
    minDuration: /^\d+$/.test(minDuration) ? minDuration : '',
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
  if (state.since) query.since = state.since;
  if (state.minDuration) query.min_duration = String(state.minDuration);
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

/**
 * Параметры кода ответа для запроса к серверу. Класс статусов разворачивается в
 * границы диапазона: точным `status` его не выразить, а сервер ждёт числа.
 * @param {string} status
 * @returns {Record<string, number>}
 */
export function statusFilterParams(status) {
  if (!status) return {};
  if (status === 'errors') return { status_min: 400 };

  const cls = /^([1-5])xx$/.exec(status);
  if (cls) {
    const base = Number(cls[1]) * 100;
    return { status_min: base, status_max: base + 99 };
  }
  return { status: Number(status) };
}

/**
 * Включён ли быстрый отбор. Состояние пресета не хранится отдельно: он и есть
 * обычные фильтры, поэтому кнопка светится ровно тогда, когда они выставлены.
 * @param {JournalState} state
 * @param {string} key
 * @returns {boolean}
 */
export function isJournalPresetOn(state, key) {
  switch (key) {
    case 'errors': return state.status === 'errors';
    case 'slow': return String(state.minDuration) === String(SLOW_REQUEST_MS);
    case 'hour': return Boolean(state.since);
    default: return false;
  }
}

/**
 * Включает или снимает быстрый отбор, возвращая новый отбор целиком.
 *
 * Взаимоисключения разводятся здесь же: «только ошибки» поверх выбранного кода
 * оставило бы на экране один этот код, а момент «час назад» и день из поля «с»
 * борются за одну и ту же границу периода.
 * @param {JournalState} state
 * @param {string} key
 * @param {Date} [now]
 * @returns {JournalState}
 */
export function toggleJournalPreset(state, key, now = new Date()) {
  const on = isJournalPresetOn(state, key);
  const next = { ...state, page: 1 };

  if (key === 'errors') {
    next.status = on ? '' : 'errors';
  } else if (key === 'slow') {
    next.minDuration = on ? '' : String(SLOW_REQUEST_MS);
  } else if (key === 'hour') {
    next.since = on ? '' : new Date(now.getTime() - LAST_HOUR_MS).toISOString();
    if (!on) {
      next.from = '';
      next.to = '';
    }
  }
  return next;
}

/**
 * Отбор журнала в виде параметров запроса. Списком и выгрузкой пользуется один
 * и тот же набор: файл обязан покрывать ровно то, что видно на экране.
 *
 * @param {{search?: string, method?: string, status?: string, user?: string|number,
 *   from?: string, to?: string, since?: string, minDuration?: string|number}} state
 * @returns {Record<string, string|number>}
 */
export function filterParamsFromState(state) {
  const params = { ...statusFilterParams(state.status) };
  if (state.search) params.search = state.search;
  if (state.method) params.method = state.method;
  if (state.user) params.user_id = state.user;
  if (state.minDuration) params.min_duration_ms = state.minDuration;
  // Момент быстрого отбора перебивает день из поля «с»: одну границу периода
  // сервер принимает один раз.
  if (state.since) params.from_date = state.since;
  else if (state.from) params.from_date = state.from;
  if (state.to) params.to_date = state.to;
  return params;
}

/**
 * Пункты выпадающих списков отбора. BaseDropdown ждёт объекты, а пустое
 * значение первым пунктом - это «все»: сбросить фильтр иначе нечем, у списка
 * нет собственной кнопки очистки.
 */
export const METHOD_FILTER_OPTIONS = [
  { value: '', label: 'Все методы' },
  ...METHOD_OPTIONS.map(m => ({ value: m, label: m }))
];

export const STATUS_FILTER_OPTIONS = [
  { value: '', label: 'Все статусы' },
  ...STATUS_OPTIONS
];

export const PAGE_SIZE_OPTIONS = PAGE_SIZES.map(size => ({ value: size, label: `${size} на странице` }));

/**
 * День из поля календаря в 'YYYY-MM-DD'. Части берутся ЛОКАЛЬНЫЕ: `toISOString`
 * сдвигает полночь в UTC и у восточных зон отдаёт предыдущий день.
 * @param {Date|null} date
 * @returns {string}
 */
export function dateToYmd(date) {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) return '';
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

/**
 * Обратный разбор для календаря. Строка собирается по частям, а не через
 * `new Date('2026-08-20')`: тот читает день как UTC-полночь и западнее Гринвича
 * показывает предыдущее число.
 * @param {string} value
 * @returns {Date|null}
 */
export function ymdToDate(value) {
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(value || ''));
  if (!parts) return null;
  const date = new Date(Number(parts[1]), Number(parts[2]) - 1, Number(parts[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/**
 * Списки отбора журнала одним перечнем: разметка у трёх выпадающих списков
 * общая, различаются только источник пунктов и поле отбора. `key` - имя поля в
 * состоянии журнала, экран пишет выбранное значение прямо по нему.
 * @param {JournalState} state
 * @param {Array<{id: number|string, username: string, is_active?: boolean}>} users
 * @param {(login: string) => string} formatUser подпись пользователя
 * @returns {Array<object>}
 */
export function journalFilterDropdowns(state, users, formatUser) {
  // Идентификатор приводится к строке: в адресе он строка, а список сверяет
  // выбранное значение строго.
  //
  // Порядок задаёт сервер: активные, следом архивные. Здесь только пометка -
  // без неё уволенный неотличим от работающего, и выбор выглядит ошибкой.
  // Помечается ЯВНОЕ false: у ответа без поля признака (старый кэш ответа)
  // пометка не нужна, иначе весь список окажется «в архиве».
  const userOptions = [
    { value: '', label: 'Все пользователи' },
    ...users.map(u => ({
      value: String(u.id),
      label: u.is_active === false ? `${formatUser(u.username)} (архив)` : formatUser(u.username)
    }))
  ];
  return [
    { key: 'method', value: state.method, options: METHOD_FILTER_OPTIONS, placeholder: 'Все методы', searchable: false },
    { key: 'status', value: state.status, options: STATUS_FILTER_OPTIONS, placeholder: 'Все статусы', searchable: false },
    { key: 'user', value: String(state.user || ''), options: userOptions, placeholder: 'Все пользователи', searchable: true, wide: true }
  ];
}
