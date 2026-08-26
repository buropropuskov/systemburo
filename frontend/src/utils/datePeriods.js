/**
 * Быстрые периоды календаря DateFilter: реестр кнопок и границы каждого периода.
 * Живут отдельно от компонента - его скриптовый блок и так вдвое сверх порога.
 */

/**
 * Кнопки быстрого выбора в порядке показа. single: true - период укладывается в
 * один день, такие доступны и в single-режиме; остальные предлагаются только
 * там, где родитель принимает диапазон.
 *
 * @type {Array<{ key: string, label: string, single?: boolean }>}
 */
export const QUICK_PERIODS = [
  { key: 'today', label: 'Сегодня', single: true },
  { key: 'yesterday', label: 'Вчера', single: true },
  { key: 'tomorrow', label: 'Завтра', single: true },
  { key: 'dayBeforeYesterday', label: 'Позавчера', single: true },
  { key: 'dayAfterTomorrow', label: 'Послезавтра', single: true },
  { key: 'thisWeek', label: 'Эта неделя' },
  { key: 'lastWeek', label: 'Прошлая неделя' },
  { key: 'nextWeek', label: 'Следующая неделя' },
  { key: 'thisMonth', label: 'Этот месяц' },
  { key: 'lastMonth', label: 'Прошлый месяц' },
  { key: 'nextMonth', label: 'Следующий месяц' },
  { key: 'thisYear', label: 'Этот год' },
  { key: 'lastYear', label: 'Прошлый год' },
];

export function isSingleDayPeriod(key) {
  return QUICK_PERIODS.some((p) => p.key === key && p.single);
}

function shiftDays(from, days) {
  const date = new Date(from);
  date.setDate(date.getDate() + days);
  return date;
}

// Понедельник недели, сдвинутой на weeks относительно недели даты from.
function weekStart(from, weeks) {
  const start = new Date(from);
  const weekday = start.getDay() || 7; // воскресенье считаем седьмым днём
  start.setDate(start.getDate() - weekday + 1 + weeks * 7);
  return start;
}

const BOUNDS = {
  today: (t) => [t, t],
  yesterday: (t) => [shiftDays(t, -1), shiftDays(t, -1)],
  tomorrow: (t) => [shiftDays(t, 1), shiftDays(t, 1)],
  dayBeforeYesterday: (t) => [shiftDays(t, -2), shiftDays(t, -2)],
  dayAfterTomorrow: (t) => [shiftDays(t, 2), shiftDays(t, 2)],
  thisWeek: (t) => [weekStart(t, 0), shiftDays(weekStart(t, 0), 6)],
  lastWeek: (t) => [weekStart(t, -1), shiftDays(weekStart(t, -1), 6)],
  nextWeek: (t) => [weekStart(t, 1), shiftDays(weekStart(t, 1), 6)],
  thisMonth: (t) => [new Date(t.getFullYear(), t.getMonth(), 1), new Date(t.getFullYear(), t.getMonth() + 1, 0)],
  lastMonth: (t) => [new Date(t.getFullYear(), t.getMonth() - 1, 1), new Date(t.getFullYear(), t.getMonth(), 0)],
  nextMonth: (t) => [new Date(t.getFullYear(), t.getMonth() + 1, 1), new Date(t.getFullYear(), t.getMonth() + 2, 0)],
  thisYear: (t) => [new Date(t.getFullYear(), 0, 1), new Date(t.getFullYear(), 11, 31)],
  lastYear: (t) => [new Date(t.getFullYear() - 1, 0, 1), new Date(t.getFullYear() - 1, 11, 31)],
};

/**
 * Границы быстрого периода: начало первого дня и конец последнего.
 *
 * @param {string} key ключ из QUICK_PERIODS
 * @param {Date} [now] точка отсчёта, по умолчанию сегодня
 * @returns {[Date, Date] | null} null для неизвестного ключа
 */
export function periodBounds(key, now = new Date()) {
  if (!BOUNDS[key]) return null;
  const today = new Date(now);
  today.setHours(0, 0, 0, 0);
  const [start, end] = BOUNDS[key](today);
  const from = new Date(start);
  from.setHours(0, 0, 0, 0);
  const to = new Date(end);
  to.setHours(23, 59, 59, 999);
  return [from, to];
}
