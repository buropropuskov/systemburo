import { moscowParts, serverNow } from '@/utils/serverTime';
import { formatMomentDate } from '@/utils/datetime';

/**
 * Правила заявки с машиной «По факту» на стороне формы (#2320).
 *
 * Сами правила живут на бэкенде и проверяются при подаче - здесь их зеркало,
 * чтобы человек узнавал об ограничении в момент действия, а не тостом после того,
 * как заполнил всю форму. Владелец так и наткнулся: добавил две машины «По факту»
 * на месяц и закрыл форму, не дойдя до отправки.
 *
 * Тексты держим рядом с проверками: они показываются прямо в форме, и правило
 * должно объясняться, а не просто запрещать.
 */

/** Номер, который форма пишет при включённом тумблере «по факту». */
export const BY_FACT_PLATE = 'По факту';

/** Почему нельзя добавить вторую такую машину. */
export const BY_FACT_ALREADY_ADDED = 'в заявке уже есть машина «По факту» - допускается одна';

/** Правило срока: первая строка предупреждения. */
export const BY_FACT_PERIOD_RULE = 'Пропуск «По факту» оформляется на срок до суток.';

/**
 * Крайняя дата окончания заявки «По факту» на текущий момент.
 *
 * Конец суток, в которые попадает «сейчас плюс 24 часа»: в 17:38 пятого числа это
 * шестое. Округление вверх до конца дня и есть тот запас, без которого предел
 * сползал бы каждую минуту - заявка, оформленная в 17:31, через минуту оказывалась
 * бы просроченной. Считается по московским часам, как и весь срок в заявке.
 *
 * @param {Date} [now] момент отсчёта
 * @returns {string} ДД.ММ.ГГГГ
 */
export function byFactDeadline(now = serverNow()) {
  return formatMomentDate(new Date(now.getTime() + 24 * 60 * 60 * 1000));
}

/** Подсказка про крайний срок: вторая строка предупреждения. */
export function byFactDeadlineHint(now = serverNow()) {
  return `Сейчас заявку можно оформить по ${byFactDeadline(now)} включительно.`;
}

/**
 * Машина записана как «По факту»? Сравнение нормализованное: значение приходит и
 * из формы, и из ранее сохранённых данных.
 *
 * Полей три, потому что номер зовётся по-разному на разных отрезках пути: форма
 * подачи держит его как `plateNumber` (VehicleForm), API заявки отдаёт
 * `car_number`, реестр уникальных машин - `number`. Проверять одно из них значит
 * работать в тестах и молчать в браузере.
 *
 * @param {{plateNumber?: string, car_number?: string, number?: string}} vehicle
 * @returns {boolean}
 */
export function isByFactVehicle(vehicle) {
  const plate = String(vehicle?.plateNumber ?? vehicle?.car_number ?? vehicle?.number ?? '');
  return plate.replace(/\s+/g, '').toLowerCase() === BY_FACT_PLATE.replace(/\s+/g, '').toLowerCase();
}

/**
 * Есть ли в заявке машина «По факту», кроме редактируемой сейчас.
 *
 * Считаем по ВСЕМ вложениям заявки, а не по одному: правило бэкенда про заявку
 * целиком, и во втором бланке вторая такая машина была бы принята формой, а
 * отклонена сервером.
 *
 * @param {Array|Object<string, Array>} source список машин либо машины по ключу вложения
 * @param {object|null} [editing] машина, которую сейчас правят - она не в счёт
 * @returns {boolean}
 */
export function hasByFactVehicle(source, editing = null) {
  const lists = Array.isArray(source) ? [source] : Object.values(source || {});
  return lists.some((list) => (list || []).some(
    (v) => isByFactVehicle(v) && (!editing || v !== editing),
  ));
}

/**
 * Период вложения укладывается в крайний срок?
 *
 * Форма отдаёт период в виде API-объекта (`currentEntryPeriod`), даты там уже в
 * формате ГГГГ-ММ-ДД, поэтому сравнение строкой. Пустой период считаем недопустимым:
 * бэкенд отклоняет заявку «По факту» без дат так же, как многодневную.
 *
 * @param {{date_from?: string, date_to?: string}|null} period
 * @returns {boolean}
 */
export function isOneDayPeriod(period, now = serverNow()) {
  const from = String(period?.date_from || '').trim();
  const to = String(period?.date_to || '').trim();
  return Boolean(from) && Boolean(to) && isWithinByFactDeadline(to, now);
}

/**
 * Предупреждение для общей панели формы: правило срока и крайняя дата.
 *
 * Форма показывает его там же, где предупреждения по расписанию мест, - панель
 * одна, и отдельное сообщение под полями дат только дробило бы внимание.
 *
 * @param {Date} [now] момент отсчёта
 * @returns {{name: string, free: string, windows: string[]}} группа для SchedulePlaceWarningPanel
 */
export function byFactWarningGroup(now = serverNow()) {
  return { name: 'Машина «По факту»', free: BY_FACT_PERIOD_RULE, windows: [byFactDeadlineHint(now)] };
}

/**
 * Дата окончания укладывается в крайний срок?
 *
 * @param {string} isoDate дата окончания в формате ГГГГ-ММ-ДД (как её отдаёт форма)
 * @param {Date} [now] момент отсчёта
 * @returns {boolean}
 */
export function isWithinByFactDeadline(isoDate, now = serverNow()) {
  const дата = String(isoDate || '').trim();
  if (!дата) return false;
  const предел = new Date(now.getTime() + 24 * 60 * 60 * 1000);
  const p = moscowParts(предел);
  const pad = (n) => String(n).padStart(2, '0');
  return дата <= `${p.year}-${pad(p.month)}-${pad(p.day)}`;
}

/**
 * Срок вложения нарушает правило «По факту»?
 *
 * Истинно, когда в заявке есть такая машина (или тумблер включён прямо сейчас), а
 * дата окончания уходит дальше крайней. Собрано в одном месте: форма по этому
 * признаку и красит поля дат, и поднимает предупреждение в панели.
 *
 * @param {{date_from?: string, date_to?: string}|null} period срок вложения
 * @param {Array} vehicles машины вложения
 * @param {boolean} pending включён ли тумблер «по факту» в форме
 * @param {Date} [now] момент отсчёта - без него проверка зависела бы от дня прогона
 * @returns {boolean}
 */
export function byFactPeriodBroken(period, vehicles, pending, now = serverNow()) {
  if (!pending && !hasByFactVehicle(vehicles)) return false;
  return !isOneDayPeriod(period, now);
}
