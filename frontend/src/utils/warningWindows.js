/**
 * Предупреждения у мест разгрузки и таблиц проезда/прохода (#1183).
 *
 * Окно-предупреждение (`warning_windows`) описывает нюанс места на день/интервал:
 * `day_of_week` (null = каждый день, 0..6 = Пн..Вс), `time_from`/`time_to`
 * (null = весь день, иначе "ЧЧ:ММ"), `is_next_day` (интервал переходит через
 * полночь), `message`, `is_active`.
 *
 * В S4 «релевантным» считается окно, действующее в переданный момент `at`
 * (по умолчанию — сейчас). S5 обобщит: вместо одного момента подставит границы
 * срока действия заявки, поэтому момент здесь — явный аргумент.
 */

/** JS `Date.getDay()` (0=Вс..6=Сб) -> проектная конвенция (0=Пн..6=Вс). */
function projectDayOfWeek(date) {
  const jsDay = date.getDay();
  return jsDay === 0 ? 6 : jsDay - 1;
}

/** "ЧЧ:ММ[:СС]" -> минуты от начала суток. */
function toMinutes(timeStr) {
  const [hours, minutes] = timeStr.split(':').map(Number);
  return hours * 60 + minutes;
}

/**
 * Действует ли окно-предупреждение в момент `at`.
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
