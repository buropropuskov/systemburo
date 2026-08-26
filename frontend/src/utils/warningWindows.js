/**
 * Предупреждения у мест разгрузки и таблиц проезда/прохода (#1183).
 *
 * Окно-предупреждение (`warning_windows`) описывает нюанс места на день/интервал:
 * `day_of_week` (null = каждый день, 0..6 = Пн..Вс), `time_from`/`time_to`
 * (null = весь день, иначе "ЧЧ:ММ"), `is_next_day` (интервал переходит через
 * полночь), `message`, `is_active`.
 *
 * В S4 «релевантным» считается окно, действующее в переданный момент `at`
 * (по умолчанию — сейчас). S5 обобщил на границы срока заявки для расписания
 * (`scheduleCheck.js`); момент здесь — явный аргумент по той же причине.
 */

import { projectDayOfWeek, toMinutes } from '@/utils/timeSlots';

/**
 * Действует ли окно-предупреждение в момент `at`.
 *
 * Для конкретного `day_of_week` окно оценивается только в ЭТОТ день недели -
 * как слоты расписания на бэке (`computeUnloadPlaceStatusAt`, `computeWorkModeStatus`):
 * `is_next_day`-интервал активен в [time_from,24:00) и [00:00,time_to] того же дня,
 * а НЕ переносится на следующий календарный день. Для `day_of_week=null` (каждый
 * день) это ограничение не проявляется. Конвенция намеренно зеркалит бэк, чтобы
 * FE-показ и BE-статус не разъезжались.
 *
 * @param {{day_of_week: (number|null), time_from: (string|null), time_to: (string|null), is_next_day: boolean, is_active: boolean}} win
 * @param {Date} [at] момент проверки (по умолчанию — текущий)
 * @returns {boolean}
 */
export function isWindowActiveAt(win, at = new Date()) {
  if (!win || win.is_active === false) return false;

  const dayEveryDay = win.day_of_week === null || win.day_of_week === undefined;
  if (!dayEveryDay && win.day_of_week !== projectDayOfWeek(at)) return false;

  // null-границы = весь день; день уже совпал -> окно действует.
  if (!win.time_from || !win.time_to) return true;

  const nowMin = at.getHours() * 60 + at.getMinutes();
  const fromMin = toMinutes(win.time_from);
  const toMin = toMinutes(win.time_to);

  if (win.is_next_day) {
    return nowMin >= fromMin || nowMin <= toMin;
  }
  return nowMin >= fromMin && nowMin <= toMin;
}

/**
 * Собирает предупреждения места/таблицы, релевантные на момент `at`:
 * свободный текст (всегда, если задан) + сообщения активных сейчас окон.
 * @param {{warning: (string|null), warning_windows: (Array|undefined)}} entity нормализованный объект (для таблицы `warning` брать из `table.warning`)
 * @param {Date} [at] момент проверки
 * @returns {{free: (string|null), windows: string[]}}
 */
export function collectActiveWarnings(entity, at = new Date()) {
  const free = entity && typeof entity.warning === 'string' && entity.warning.trim()
    ? entity.warning.trim()
    : null;

  const windows = Array.isArray(entity && entity.warning_windows)
    ? entity.warning_windows
        .filter((win) => isWindowActiveAt(win, at))
        .map((win) => (win.message || '').trim())
        .filter(Boolean)
    : [];

  return { free, windows };
}
