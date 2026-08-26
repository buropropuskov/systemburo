/**
 * Группировка списка заявок по периодам подачи для визуальных разделителей
 * («Сегодня», «Вчера», «На этой неделе», ...). Один источник для Центра заявок
 * и списка заявок в личном кабинете - подписи и границы периодов в обоих
 * списках обязаны совпадать.
 */

const MS_IN_DAY = 86400000;

function startOfDay(date) {
  const x = new Date(date);
  x.setHours(0, 0, 0, 0);
  return x;
}

/**
 * Подпись периода, в который попадает дата.
 *
 * @param {string|Date} dateStr дата подачи заявки
 * @param {Date} [now] «сейчас» (для тестов и одного значения на весь список)
 * @returns {string} подпись периода
 */
export function applicationPeriodLabel(dateStr, now) {
  const d = startOfDay(new Date(dateStr));
  const today = startOfDay(now || new Date());
  const yesterday = new Date(today.getTime() - MS_IN_DAY);
  const dow = (today.getDay() + 6) % 7; // 0 = понедельник
  const thisWeekStart = new Date(today.getTime() - dow * MS_IN_DAY);
  const lastWeekStart = new Date(thisWeekStart.getTime() - 7 * MS_IN_DAY);
  const thisMonthStart = new Date(today.getFullYear(), today.getMonth(), 1);
  const lastMonthStart = new Date(today.getFullYear(), today.getMonth() - 1, 1);
  if (d.getTime() >= today.getTime()) return 'Сегодня';
  if (d.getTime() === yesterday.getTime()) return 'Вчера';
  if (d >= thisWeekStart) return 'На этой неделе';
  if (d >= lastWeekStart) return 'На прошлой неделе';
  if (d >= thisMonthStart) return 'В этом месяце';
  if (d >= lastMonthStart) return 'В прошлом месяце';
  return 'Ранее';
}

/**
 * Разбивает УЖЕ отсортированный список на группы по периоду подачи. Новая группа
 * начинается там, где меняется подпись периода, поэтому порядок групп следует
 * текущей сортировке. При сортировке НЕ по дате разделители бессмысленны -
 * возвращается одна группа без подписи.
 *
 * @param {Array<{ sending_datetime: string }>} applications отсортированный список
 * @param {boolean} sortedByDate сортировка идёт по дате подачи
 * @param {Date} [now] «сейчас»
 * @returns {Array<{ label: string|null, key: string, apps: Array }>} группы
 */
export function groupApplicationsByPeriod(applications, sortedByDate, now) {
  if (!sortedByDate) return [{ label: null, key: 'all', apps: applications }];
  const moment = now || new Date();
  const groups = [];
  let current = null;
  for (const application of applications) {
    const label = applicationPeriodLabel(application.sending_datetime, moment);
    if (!current || current.label !== label) {
      current = { label, key: `grp-${label}`, apps: [] };
      groups.push(current);
    }
    current.apps.push(application);
  }
  return groups;
}
