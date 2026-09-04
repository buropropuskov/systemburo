/**
 * Показ записей журнала обращений: длительности, даты, классы метода и статуса.
 * Вынесено из RequestsView.vue - к состоянию компонента ничего из этого
 * отношения не имеет.
 *
 * Время московское (#2298): выгрузка того же журнала печатается по Москве, и
 * расхождение экрана с файлом читалось бы как потерянные записи.
 */
import { formatMoscow } from '@/utils/serverTime';

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
  return formatMoscow(new Date(timestamp), {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

/**
 * Отметка строки журнала: день и время. Одного времени мало - подробные записи
 * живут 30 суток и отбираются по датам, а по «14:32:05» не понять, к какому дню
 * относится строка. Год не показывается: он один на весь срок хранения, а место
 * в колонке узкое; полная дата остаётся во всплывающей подсказке и в окне деталей.
 * @param {string} timestamp
 * @returns {string}
 */
export function formatStamp(timestamp) {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return '';
  const day = formatMoscow(date, { day: '2-digit', month: '2-digit' });
  return `${day} ${formatTime(timestamp)}`;
}

/**
 * @param {string} timestamp
 * @returns {string}
 */
export function formatFullDate(timestamp) {
  if (!timestamp) return '';
  return formatMoscow(new Date(timestamp), {
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

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * Сутки ответа (2026-08-19) числом времени. Разбор идёт по UTC: ряд перечисляет
 * календарные даты, и переход на летнее время в местной зоне не должен смещать
 * шаг на час.
 * @param {string} day
 * @returns {number} NaN для неразобранного дня
 */
function dayToUtc(day) {
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(day || ''));
  if (!parts) return NaN;
  return Date.UTC(Number(parts[1]), Number(parts[2]) - 1, Number(parts[3]));
}

/**
 * @param {number} utc
 * @returns {string} сутки в записи ответа
 */
function utcToDay(utc) {
  return new Date(utc).toISOString().slice(0, 10);
}

/**
 * Ряд столбиков по суткам: от первого дня с записями до последнего, без
 * пропусков в календаре.
 *
 * Сервер отдаёт только сутки, в которых записи есть. Если рисовать ряд как
 * пришёл, дни без обращений схлопываются и соседние столбики встают вплотную:
 * 10 июля и 19 августа оказываются рядом, а расстояние по оси перестаёт значить
 * время. Пропущенные сутки добавляются с `null`, а не с нулём: после отсева
 * проверок доступности (S2) сутки без единой записи означают, что системой не
 * пользовались, и столбик нулевой высоты читался бы как измеренный ноль.
 *
 * @param {Array<{day: string, requests: number, errors: number}>} daily ряд из ответа истории
 * @returns {Array<{day: string, requests: number|null, errors: number|null}>}
 */
export function dailyChartPoints(daily) {
  const known = new Map();
  (daily || []).forEach((point) => {
    const utc = dayToUtc(point && point.day);
    if (!Number.isNaN(utc)) known.set(utc, point);
  });
  if (!known.size) return [];

  const days = [...known.keys()].sort((a, b) => a - b);
  const points = [];
  for (let utc = days[0]; utc <= days[days.length - 1]; utc += DAY_MS) {
    const point = known.get(utc);
    points.push({
      day: utcToDay(utc),
      requests: point ? Number(point.requests) || 0 : null,
      errors: point ? Number(point.errors) || 0 : null
    });
  }
  return points;
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
    {
      label: 'Запросов в журнале', value: formatNum(s.total),
      hint: 'Подробные записи журнала за срок их хранения. На вкладке «Аналитика · история» итог считается за выбранный период и включает свёрнутые сутки, поэтому там число больше.'
    },
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
