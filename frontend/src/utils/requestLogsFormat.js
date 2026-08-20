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

/**
 * Человеческий текст отказа вместо пустого экрана. До #2125 сбой чтения журнала
 * гасился молча (`if (!response.ok) return`), и раздел без прав выглядел как
 * раздел без записей - неотличимо от пустого отбора.
 *
 * @param {{status?: number}} source ответ fetch или ошибка с кодом
 * @param {string} action что именно не получилось: «загрузить журнал»
 * @returns {string} строка для экрана
 */
export function describeLoadError(source, action) {
  const status = source && typeof source.status === 'number' ? source.status : 0;
  if (status === 401) return `Не удалось ${action}: сессия истекла, войдите заново.`;
  if (status === 403) return `Не удалось ${action}: нет прав на раздел.`;
  if (status >= 500) return `Не удалось ${action}: сервер ответил ошибкой ${status}.`;
  if (status) return `Не удалось ${action}: сервер ответил ${status}.`;
  return `Не удалось ${action}: нет связи с сервером.`;
}

/**
 * Показатели шапки вкладки «Аналитика» одним перечнем: четыре почти одинаковых
 * блока в разметке отличались только подписью и способом форматирования.
 *
 * @param {{requests?: number, error_rate?: number, avg_duration_ms?: number, errors?: number}} totals
 * @returns {Array<{label: string, value: string, bad?: boolean, hint?: string}>}
 */
export function analyticsKpis(totals) {
  const t = totals || {};
  return [
    { label: 'Запросов за период', value: formatNum(t.requests) },
    { label: 'Доля ошибок', value: `${Number(t.error_rate || 0).toFixed(2)}%`, bad: Number(t.error_rate) > 1 },
    {
      label: 'Средн. длительность', value: formatMs(t.avg_duration_ms),
      hint: 'Средняя взвешена по числу запросов. Долгоживущие подписки на события в неё не входят: у них в журнале записано время жизни соединения.'
    },
    { label: 'Ошибок всего', value: formatNum(t.errors) }
  ];
}

/**
 * Показатели шапки раздела. Отдаются тем же перечнем, что и сводка аналитики:
 * ряд карточек в шапке и ряд карточек на вкладке рисует один компонент.
 * Порог доли ошибок здесь 5%: шапка считает за последний час, где одиночный
 * сбой даёт заметный процент, а сводка - за период в сутках. По той же причине
 * час назван прямо в подписях: на вкладке аналитики рядом стоит карточка с той
 * же долей ошибок за выбранный период, и одинаковая подпись читалась бы как
 * расхождение чисел.
 *
 * @param {{total?: number, today?: number, median_duration?: number, p95_duration?: number,
 *          error_rate?: number, requests_per_minute?: number}} stats показатели за час
 * @param {{last_second_count?: number, last_minute_count?: number}} realtime счётчики ленты
 * @returns {Array<{label: string, value: string, sub?: string, hint?: string, bad?: boolean, live?: boolean}>}
 */
export function headerKpis(stats, realtime) {
  const s = stats || {};
  const live = realtime || {};
  const kpis = [
    { label: 'Запросов всего', value: formatNum(s.total) },
    { label: 'Запросов сегодня', value: formatNum(s.today) },
    {
      label: 'Отклик за час, медиана и p95',
      value: `${formatMs(s.median_duration, false)} / ${formatMs(s.p95_duration)}`,
      hint: 'Медиана и 95-й перцентиль времени ответа за последний час. Долгоживущие подписки на события не учитываются: у них в журнале записано время жизни соединения, а не время ответа.'
    },
    {
      label: 'Доля ошибок за час', value: `${Number(s.error_rate || 0).toFixed(1)}%`,
      bad: Number(s.error_rate) > 5, hint: 'Доля ответов с кодом 4xx и 5xx за последний час.'
    },
    {
      label: 'Запросов в минуту', value: Number(s.requests_per_minute || 0).toFixed(1),
      hint: 'Средний темп обращений за последний час.'
    }
  ];
  // Счётчик ленты появляется, только когда сервер его прислал: до первого
  // ответа пустая карточка «0/с» выглядит как затишье, а не как «ещё не знаем».
  if (live.last_minute_count != null) {
    kpis.push({
      label: 'Сейчас', value: `${live.last_second_count || 0}/с`,
      sub: `${live.last_minute_count || 0}/мин`, live: true,
      hint: 'Обращений за последнюю секунду и за последнюю минуту.'
    });
  }
  return kpis;
}

/**
 * Сообщение об итоге выгрузки. Обрезанный файл проговаривается словами: сервер
 * отдаёт не больше десяти тысяч строк, и по неполному файлу человек считал бы
 * итоги за период, не зная об остатке.
 *
 * @param {{rows: number, total: number, truncated: boolean}} res охват выгрузки
 * @returns {{prefix: string, bold: string, suffix?: string, type: string}}
 */
export function exportNotice({ rows, total, truncated }) {
  if (!truncated) return { prefix: 'Журнал выгружен, ', bold: `записей: ${rows}`, type: 'success' };
  return {
    prefix: 'Выгружены первые ', bold: `${rows} записей из ${total}`,
    suffix: '. Сузьте период или отбор, чтобы файл покрыл всё.', type: 'warning'
  };
}

/** Пустая аналитика: тем же видом экран начинает жизнь и встречает пустой ответ. */
export function emptyHistory() {
  return {
    totals: { requests: 0, errors: 0, error_rate: 0, avg_duration_ms: 0 },
    coverage: null,
    daily: [],
    top_endpoints: [],
    top_users: []
  };
}

/**
 * Ответ истории в том виде, в каком его показывает вкладка аналитики.
 * @param {object} data
 * @returns {object}
 */
export function historyFromResponse(data) {
  const empty = emptyHistory();
  return {
    totals: data.totals || empty.totals,
    coverage: data.coverage || null,
    daily: data.daily || [],
    top_endpoints: data.top_endpoints || [],
    top_users: data.top_users || []
  };
}

/**
 * Показ значений журнала одним набором - экран подмешивает его в methods
 * целиком, а не перечисляет функции по одной.
 */
export const LOG_FORMATTERS = {
  formatMs, formatDuration, formatDay, formatNum, formatTime, formatFullDate,
  truncatePath, formatJson, getMethodClass, getStatusClass
};
