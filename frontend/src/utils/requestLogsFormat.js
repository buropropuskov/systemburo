/**
 * Показ записей журнала обращений: длительности, даты, классы метода и статуса.
 * Вынесено из RequestsView.vue - к состоянию компонента ничего из этого
 * отношения не имеет.
 */

const METHOD_CLASSES = {
  GET: 'method-get',
  POST: 'method-post',
  PUT: 'method-put',
  DELETE: 'method-delete',
  PATCH: 'method-patch'
};

/**
 * Длительность в миллисекундах: до сотни - с одним знаком после запятой,
 * дальше целыми. Ответы быстрее миллисекунды раньше показывались нулём, потому
 * что бэк округлял их вниз.
 * @param {number} ms
 * @param {boolean} [withUnit] дописать единицы измерения
 * @returns {string}
 */
export function formatMs(ms, withUnit = true) {
  const value = Number(ms) || 0;
  const rounded = value >= 100 ? Math.round(value) : Math.round(value * 10) / 10;
  return withUnit ? rounded + 'мс' : String(rounded);
}

/**
 * Длительность записи журнала: микросекунды точнее, миллисекунды остаются для
 * записей, сделанных до перехода на них.
 * @param {{duration_us?: number, duration_ms?: number}} log
 * @returns {string}
 */
export function formatDuration(log) {
  if (!log) return '';
  if (log.duration_us != null) return formatMs(log.duration_us / 1000);
  return formatMs(log.duration_ms || 0);
}

/**
 * Сутки из ответа (2026-08-19) в привычный вид (19.08.2026).
 * @param {string} day
 * @returns {string}
 */
export function formatDay(day) {
  const parts = String(day || '').split('-');
  return parts.length === 3 ? `${parts[2]}.${parts[1]}.${parts[0]}` : String(day || '');
}

/**
 * @param {number} n
 * @returns {string}
 */
export function formatNum(n) {
  return (n || 0).toLocaleString('ru-RU');
}

/**
 * @param {string} timestamp
 * @returns {string}
 */
export function formatTime(timestamp) {
  if (!timestamp) return '';
  return new Date(timestamp).toLocaleTimeString('ru-RU', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

/**
 * @param {string} timestamp
 * @returns {string}
 */
export function formatFullDate(timestamp) {
  if (!timestamp) return '';
  return new Date(timestamp).toLocaleString('ru-RU', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

/**
 * @param {string} path
 * @returns {string}
 */
export function truncatePath(path) {
  if (!path) return '';
  return path.length > 40 ? path.substring(0, 37) + '...' : path;
}

/**
 * Тело запроса или ответа с отступами, если это разбираемый JSON.
 * @param {string} text
 * @returns {string}
 */
export function formatJson(text) {
  if (!text) return '';
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

/**
 * @param {string} method
 * @returns {string}
 */
export function getMethodClass(method) {
  return METHOD_CLASSES[method] || 'method-other';
}

/**
 * @param {number} status
 * @returns {string}
 */
export function getStatusClass(status) {
  if (!status) return 'status-unknown';
  if (status < 300) return 'status-success';
  if (status < 400) return 'status-redirect';
  if (status < 500) return 'status-client-error';
  return 'status-server-error';
}

/**
 * Что именно показано на вкладке «Аналитика»: запрошенный период, сутки с
 * записями и источник чисел. Пустой месяц без этой подписи выглядел как
 * «запросов не было», хотя данные просто ещё не свёрнуты.
 * @param {object|null} coverage блок coverage из ответа истории
 * @returns {string}
 */
export function coverageNote(coverage) {
  if (!coverage || !coverage.requested_from) return '';

  const period = `${formatDay(coverage.requested_from)} - ${formatDay(coverage.requested_to)}`;
  if (!coverage.days) {
    return coverage.aggregated_through
      ? `За период ${period} записей нет.`
      : `За период ${period} записей нет: журнал ещё не сворачивался в суточные итоги.`;
  }

  const sources = {
    aggregates: 'по свёрнутым итогам суток',
    detailed: 'по подробным записям журнала',
    mixed: 'по свёрнутым итогам и подробным записям'
  };
  const covered = coverage.from === coverage.to
    ? formatDay(coverage.from)
    : `${formatDay(coverage.from)} - ${formatDay(coverage.to)}`;
  const source = sources[coverage.source] ? `, ${sources[coverage.source]}` : '';
  return `Запрошен период ${period}. Записи есть за ${covered}, суток с данными: ${coverage.days}${source}.`;
}

/**
 * Оговорка про перцентиль. Пустая, когда весь период посчитан по самим записям
 * и p95 честный.
 * @param {object|null} coverage
 * @returns {string}
 */
export function p95Note(coverage) {
  if (!coverage || !coverage.days || coverage.exact_p95) return '';
  return 'За свёрнутые сутки показано наибольшее суточное значение: отдельных длительностей у них уже нет.';
}

/**
 * Высота столбика ряда по суткам в процентах: самый высокий занимает колонку
 * целиком, пустой день остаётся видимой полоской.
 * @param {number} value
 * @param {number} max
 * @returns {number}
 */
export function barHeight(value, max) {
  return Math.max(4, Math.round((value / Math.max(max, 1)) * 100));
}
