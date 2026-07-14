/**
 * Авто-проверка расписания места (time_slots) против срока заявки (#1183 S5).
 *
 * Слот расписания (`time_slots`): `day_of_week` (0=Пн..6=Вс), `open_time`/`close_time`
 * ("ЧЧ:ММ[:СС]"), `is_next_day` (интервал переходит через полночь), `is_active`.
 * Круглосуточный слот - `00:00`-`23:59` без `is_next_day`.
 *
 * Ядро `isSlotOpenAt`/`schedulePlaceStatusAt` - обобщение `isActiveSlot`
 * (UnloadPlaceModal.vue) и BE `computeUnloadPlaceStatusAt`: вместо `new Date()`
 * момент проверки инъектируется, поэтому расписание можно сверять не только с
 * «сейчас», а с границами срока заявки (въезд/выезд).
 *
 * Время считается по частям браузерной локали (`getDay`/`getHours`), как в S4
 * (`warningWindows.js`) и `isActiveSlot` - слоты заданы в московском дне, а
 * пользователи бюро работают в МСК, поэтому явной конверсии зоны нет (единая
 * конвенция с уже задеплоенным FE-показом расписания).
 */

import { projectDayOfWeek, toMinutes } from '@/utils/timeSlots';

/** Круглосуточный слот: 00:00-23:59 без перехода через полночь. */
function isRoundTheClock(slot) {
  return (
    slot.open_time && slot.close_time &&
    slot.open_time.slice(0, 5) === '00:00' &&
    slot.close_time.slice(0, 5) === '23:59' &&
    !slot.is_next_day
  );
}

/**
 * Открыт ли слот расписания в момент `at` (день недели + время суток).
 * Зеркалит `isActiveSlot` (UnloadPlaceModal) - тот же разбор `is_next_day`
 * (активен в [open,24:00) и [00:00,close] ТОГО ЖЕ дня), но момент явный.
 *
 * @param {{day_of_week:number, open_time:string, close_time:string, is_next_day:boolean, is_active:boolean}} slot
 * @param {Date} at
 * @returns {boolean}
 */
export function isSlotOpenAt(slot, at) {
  if (!slot || slot.is_active === false) return false;
  if (slot.day_of_week !== projectDayOfWeek(at)) return false;
  if (isRoundTheClock(slot)) return true;
  if (!slot.open_time || !slot.close_time) return false;

  const cur = at.getHours() * 60 + at.getMinutes();
  const open = toMinutes(slot.open_time);
  const close = toMinutes(slot.close_time);

  if (slot.is_next_day) {
    return cur >= open || cur <= close;
  }
  return cur >= open && cur <= close;
}

/**
 * Статус места по расписанию в момент `at`.
 * `no-schedule` - слоты не заданы (режим работы не указан -> предупреждать не о чём).
 * @param {Array} slots
 * @param {Date} at
 * @returns {'open'|'closed'|'no-schedule'}
 */
export function schedulePlaceStatusAt(slots, at) {
  if (!Array.isArray(slots) || slots.length === 0) return 'no-schedule';
  return slots.some((slot) => isSlotOpenAt(slot, at)) ? 'open' : 'closed';
}

function pad2(n) {
  return String(n).padStart(2, '0');
}

/** Date -> "дд.мм.гггг чч:мм" (локальные части). */
function formatMoment(dt) {
  return `${pad2(dt.getDate())}.${pad2(dt.getMonth() + 1)}.${dt.getFullYear()} ${pad2(dt.getHours())}:${pad2(dt.getMinutes())}`;
}

/**
 * Границу срока ("YYYY-MM-DD" + "ЧЧ:ММ[:СС]") -> Date по локальным частям.
 *
 * Дата и время в форме (`DateRangeSection`) - независимые поля, заполняются по
 * отдельности. Пока время не введено, граница НЕЗАВЕРШЕНА - возвращаем null
 * (проверка её пропускает), а НЕ дефолтим на полночь: иначе "дата есть, время нет"
 * дало бы ложное "закрыто в 00:00", пока пользователь ещё заполняет форму.
 *
 * @param {?string} dateStr
 * @param {?string} timeStr
 * @returns {?Date}
 */
function parseBoundary(dateStr, timeStr) {
  if (!dateStr || !timeStr) return null;
  const [year, month, day] = dateStr.split('-').map(Number);
  if (!year || !month || !day) return null;
  const [h, m] = timeStr.split(':').map(Number);
  const hours = Number.isFinite(h) ? h : 0;
  const minutes = Number.isFinite(m) ? m : 0;
  const dt = new Date(year, month - 1, day, hours, minutes, 0, 0);
  return Number.isNaN(dt.getTime()) ? null : dt;
}

/**
 * Предупреждения расписания места против срока заявки (#1183 S5), неблокирующе.
 *
 * Проверяются ГРАНИЦЫ срока - момент въезда (`date_from`+`time_from`) и выезда
 * (`date_to`+`time_to`). Если место по расписанию закрыто на границе -> строка
 * предупреждения. Промежуточные дни срока НЕ проверяются (граничная эвристика:
 * закрытый день ПОСЕРЕДИНЕ многодневного срока не ловится - это осознанное
 * упрощение для информационного hint, не блокирующая валидация).
 *
 * @param {Array} slots расписание места (`time_slots`)
 * @param {?{date_from:?string, date_to:?string, time_from:?string, time_to:?string}} period срок заявки (даты "YYYY-MM-DD", время "ЧЧ:ММ")
 * @returns {string[]}
 */
export function collectScheduleWarnings(slots, period) {
  if (!period) return [];

  const start = parseBoundary(period.date_from, period.time_from);
  const end = parseBoundary(period.date_to, period.time_to);

  const startClosed = start ? schedulePlaceStatusAt(slots, start) === 'closed' : false;
  const sameMoment = start && end && start.getTime() === end.getTime();
  const endClosed = end && !sameMoment ? schedulePlaceStatusAt(slots, end) === 'closed' : false;

  const messages = [];
  if (startClosed) {
    messages.push(`По графику работы закрыто в момент въезда (${formatMoment(start)}).`);
  }
  if (endClosed) {
    messages.push(`По графику работы закрыто в момент выезда (${formatMoment(end)}).`);
  }
  return messages;
}
