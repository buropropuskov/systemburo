/**
 * Авто-проверка расписания места (time_slots) против срока заявки (#1183 S5).
 *
 * Слот расписания (`time_slots`): `day_of_week` (0=Пн..6=Вс), `open_time`/`close_time`
 * ("ЧЧ:ММ[:СС]"), `is_next_day` (интервал переходит через полночь), `is_active`.
 * Круглосуточный слот - `00:00`-`23:59` без `is_next_day`.
 *
 * СЕМАНТИКА (важно): "Время пребывания (проезда) с-по" в заявке - это НЕ два момента
 * въезда/выезда, а ОКНО пребывания, применяемое к каждому дню срока. Предупреждение
 * нужно, когда окно пребывания [time_from, time_to] НЕ ПЕРЕСЕКАЕТСЯ с графиком работы
 * места в этот день (пример: место работает 10:00-12:00, пребывание 13:00-14:00 -
 * пересечения нет -> закрыто). Отчёт содержит режим работы по каждому дню периода,
 * чтобы показать его пользователю.
 *
 * Время считается по частям браузерной локали, как в S4 (`warningWindows.js`) и
 * `isActiveSlot` - слоты заданы в московском дне, пользователи бюро в МСК, явной
 * конверсии зоны нет (единая конвенция с задеплоенным FE-показом расписания).
 */

import { projectDayOfWeek, toMinutes } from '@/utils/timeSlots';

const WEEKDAY_SHORT = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
const DAY_MINUTES = 24 * 60;

/** Круглосуточный слот: 00:00-23:59 без перехода через полночь. */
function isRoundTheClock(slot) {
  return (
    slot.open_time && slot.close_time &&
    slot.open_time.slice(0, 5) === '00:00' &&
    slot.close_time.slice(0, 5) === '23:59' &&
    !slot.is_next_day
  );
}

function pad2(n) {
  return String(n).padStart(2, '0');
}

/** "ЧЧ:ММ[:СС]" -> "ЧЧ:ММ". */
function hhmm(t) {
  return (t || '').slice(0, 5);
}

/** "YYYY-MM-DD" -> Date (локальная полночь) или null. */
function parseDate(dateStr) {
  if (!dateStr) return null;
  const [year, month, day] = dateStr.split('-').map(Number);
  if (!year || !month || !day) return null;
  const dt = new Date(year, month - 1, day, 0, 0, 0, 0);
  return Number.isNaN(dt.getTime()) ? null : dt;
}

/**
 * Рабочие интервалы места (минуты от начала суток) для конкретного дня недели.
 * `is_next_day`-слот раскрывается в [open, 24:00) и [00:00, close] ТОГО ЖЕ дня
 * (конвенция FE `isActiveSlot`). Круглосуточный - весь день.
 */
function intervalsForWeekday(slots, weekday) {
  const res = [];
  for (const s of slots) {
    if (s.is_active === false || s.day_of_week !== weekday) continue;
    if (isRoundTheClock(s)) { res.push([0, DAY_MINUTES]); continue; }
    if (!s.open_time || !s.close_time) continue;
    const open = toMinutes(s.open_time);
    const close = toMinutes(s.close_time);
    if (s.is_next_day) {
      res.push([open, DAY_MINUTES]);
      res.push([0, close]);
    } else {
      res.push([open, close]);
    }
  }
  return res;
}

/**
 * Интервалы работы дня как массив строк (панель рендерит по строке на интервал):
 * ["10:00—12:00", "17:00—18:00"] / ["круглосуточно"] / ["не работает"].
 */
function dayHours(slots, weekday) {
  const active = slots.filter((s) => s.is_active !== false && s.day_of_week === weekday);
  if (!active.length) return ['не работает'];
  if (active.some(isRoundTheClock)) return ['круглосуточно'];
  return active
    .slice()
    .sort((a, b) => toMinutes(a.open_time) - toMinutes(b.open_time))
    .map((s) => `${hhmm(s.open_time)}—${hhmm(s.close_time)}`);
}

/** Пересекается ли хотя бы одна пара [рабочий интервал] x [интервал пребывания]. */
function overlapsAny(workIntervals, presenceIntervals) {
  return workIntervals.some(([wf, wt]) =>
    presenceIntervals.some(([pf, pt]) => pf < wt && pt > wf));
}

/**
 * Отчёт о совпадении окна пребывания срока с графиком места по дням периода.
 *
 * @param {Array} slots расписание места (`time_slots`)
 * @param {?{date_from:?string, date_to:?string, time_from:?string, time_to:?string}} period
 *   срок: даты "YYYY-MM-DD", время пребывания "ЧЧ:ММ".
 * @returns {?{presence:string, days:{weekday:number,label:string,hours:string[],open:boolean}[], anyClosed:boolean}}
 *   null, если у места нет расписания или срок неполный (проверять нечего).
 */
export function buildScheduleReport(slots, period) {
  if (!Array.isArray(slots) || slots.length === 0) return null; // режим работы не указан
  if (!period) return null;

  const from = parseDate(period.date_from);
  const to = parseDate(period.date_to);
  if (!from || !to || to.getTime() < from.getTime()) return null;
  if (!period.time_from || !period.time_to) return null; // окно пребывания ещё не задано

  const pf = toMinutes(period.time_from);
  const pt = toMinutes(period.time_to);
  if (!Number.isFinite(pf) || !Number.isFinite(pt) || pt === pf) return null;

  // Окно пребывания в минутах суток. pt < pf -> переходит через полночь (ночные
  // многодневные работы: заезд вечером, выезд утром) - раскрываем в [pf,24:00)+[00:00,pt],
  // как is_next_day-слоты, иначе такое окно молча не проверялось бы.
  const overnight = pt < pf;
  const presence = overnight ? [[pf, DAY_MINUTES], [0, pt]] : [[pf, pt]];

  const singleDay = from.getTime() === to.getTime();
  const seen = new Set();
  const days = [];
  // Идём по календарным дням периода в хронологическом порядке, по одному
  // представителю на день недели (без пересортировки - порядок как в заявке).
  for (let cursor = new Date(from), guard = 0;
    cursor.getTime() <= to.getTime() && guard < 370;
    cursor = new Date(cursor.getTime() + 86400000), guard++) {
    const weekday = projectDayOfWeek(cursor);
    if (seen.has(weekday)) continue;
    seen.add(weekday);
    const intervals = intervalsForWeekday(slots, weekday);
    days.push({
      weekday,
      label: singleDay
        ? `${WEEKDAY_SHORT[weekday]} ${pad2(cursor.getDate())}.${pad2(cursor.getMonth() + 1)}`
        : WEEKDAY_SHORT[weekday],
      hours: dayHours(slots, weekday),
      open: overlapsAny(intervals, presence),
    });
  }

  return {
    presence: `${hhmm(period.time_from)}—${hhmm(period.time_to)}`,
    days,
    anyClosed: days.some((d) => !d.open),
  };
}
