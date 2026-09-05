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

/** Почему период заблокирован на одном дне. */
export const BY_FACT_ONE_DAY_HINT = 'Заявка на машину «По факту» действует один день';

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
 * Период вложения укладывается в один день?
 *
 * Форма отдаёт период в виде API-объекта (`currentEntryPeriod`), даты там уже в
 * формате ГГГГ-ММ-ДД, поэтому сравнение строкой. Пустой период считаем недопустимым:
 * бэкенд отклоняет заявку «По факту» без дат так же, как многодневную.
 *
 * @param {{date_from?: string, date_to?: string}|null} period
 * @returns {boolean}
 */
export function isOneDayPeriod(period) {
  const from = String(period?.date_from || '').trim();
  const to = String(period?.date_to || '').trim();
  return Boolean(from) && from === to;
}
