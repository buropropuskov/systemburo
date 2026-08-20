/**
 * Живая лента журнала обращений: как часто список перечитывает себя сам и когда
 * обязан остановиться. Вынесено из RequestsView.vue отдельно от отбора - это
 * правила поведения экрана, а не параметры запроса.
 */

/** Как часто лента перечитывает список. */
export const JOURNAL_REFRESH_MS = 10000;

/**
 * Почему живая лента сейчас стоит. Пустая строка - обновление идёт.
 *
 * Причин четыре, и все они про потерю того, что человек уже держит на экране:
 * из-под открытого окна уезжает строка, на второй странице обновление
 * подменяет содержимое под курсором, а фоновая вкладка просто жжёт запросы -
 * ровно тот шум, который убирали из журнала.
 * @param {{tab: string, hidden: boolean, hasSelection: boolean, page: number}} view
 * @returns {string}
 */
export function journalRefreshBlock({ tab, hidden, hasSelection, page }) {
  if (tab !== 'journal') return 'открыта аналитика';
  if (hidden) return 'вкладка в фоне';
  if (hasSelection) return 'открыто окно запроса';
  if (page > 1) return 'открыта не первая страница';
  return '';
}

/**
 * Периоды графика: шаг сетки и число точек. Шаг от суток сервер считает по
 * суточной свёртке, поэтому «Месяц» и «Год» показывают историю, а не срок
 * хранения подробных записей.
 */
export const CHART_PERIODS = [
  { key: 'last-1m', label: 'Минута', title: 'за последнюю минуту', interval: 1, limit: 60, intervalHuman: '1 секунда', xAxisLabel: 'с' },
  { key: 'last-10m', label: '10 минут', title: 'за последние 10 минут', interval: 10, limit: 60, intervalHuman: '10 секунд', xAxisLabel: '10с' },
  { key: 'last-30m', label: '30 минут', title: 'за последние 30 минут', interval: 30, limit: 60, intervalHuman: '30 секунд', xAxisLabel: '30с' },
  { key: 'last-1h', label: '1 час', title: 'за последний час', interval: 60, limit: 60, intervalHuman: '1 минута', xAxisLabel: 'мин' },
  { key: 'last-24h', label: '24 часа', title: 'за последние 24ч', interval: 3600, limit: 24, intervalHuman: '1 час', xAxisLabel: 'ч' },
  { key: 'last-week', label: 'Неделя', title: 'за последнюю неделю', interval: 21600, limit: 28, intervalHuman: '6 часов', xAxisLabel: '6ч' },
  { key: 'last-month', label: 'Месяц', title: 'за последний месяц', interval: 86400, limit: 30, intervalHuman: '1 сутки', xAxisLabel: 'сут' },
  { key: 'last-year', label: 'Год', title: 'за последний год', interval: 604800, limit: 52, intervalHuman: '1 неделя', xAxisLabel: 'нед' }
];

/** Период по умолчанию: сутки часовыми столбиками. */
export const DEFAULT_CHART_PERIOD = 'last-24h';
