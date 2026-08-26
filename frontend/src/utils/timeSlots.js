/**
 * Общие хелперы разбора временных слотов/окон места (#1183).
 *
 * Единый источник конвенций для warningWindows.js (окна-предупреждения) и
 * scheduleCheck.js (расписание time_slots) - чтобы день недели и парсинг времени
 * не разошлись между двумя файлами при будущей правке.
 */

/** JS `Date.getDay()` (0=Вс..6=Сб) -> проектная конвенция (0=Пн..6=Вс). */
export function projectDayOfWeek(date) {
  const jsDay = date.getDay();
  return jsDay === 0 ? 6 : jsDay - 1;
}

/** "ЧЧ:ММ[:СС]" -> минуты от начала суток. */
export function toMinutes(timeStr) {
  const [hours, minutes] = timeStr.split(':').map(Number);
  return hours * 60 + minutes;
}
