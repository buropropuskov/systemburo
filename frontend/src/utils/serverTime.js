/**
 * Единое время интерфейса: московское и сверенное с сервером (#2298).
 *
 * Часы в интерфейсе брались с машины пользователя (`new Date()`). На посту по ним
 * сверяют срок действия пропуска и разрешённые часы въезда, поэтому сбитые часы на
 * рабочем месте дают неверное решение о пропуске - а заметить это некому: время на
 * экране выглядит правдоподобно.
 *
 * Две беды разом, и лечатся они по-разному:
 *
 *  1. Часы машины отстают или спешат. Лечится смещением: берём `Date` из ответа
 *     сервера и запоминаем разницу с локальными часами. Отдельного метода для этого
 *     не заводим - заголовок приходит с ЛЮБЫМ ответом, включая 401, так что синк
 *     идёт бесплатно на обычном трафике и не добавляет публичного роута.
 *  2. Машина в другом часовом поясе. Смещение тут не поможет: оно про момент
 *     времени, а не про то, как его показать. Лечится форматированием в
 *     Europe/Moscow - зона у системы одна, и бэкенд уже считает в ней (analytics_tz,
 *     moscowWorkModeLoc, граница отчётных суток).
 *
 * Точность заголовка - секунда, чего для часов и сроков достаточно. До первого
 * ответа сервера смещение нулевое: показываем местное время, а не пустоту.
 */

const MSK = 'Europe/Moscow';

/** Разница «сервер минус эта машина», миллисекунды. */
let offsetMs = 0;
let synced = false;

/**
 * Запоминает смещение по заголовку Date очередного ответа.
 *
 * Заголовок обязателен по HTTP и проставляется сервером в момент отправки, поэтому
 * сетевая задержка добавляет к нашей оценке не больше времени ответа. Для часов и
 * сроков пропуска этого хватает; синхронизация точнее потребовала бы обмена с
 * измерением задержки, а это отдельный метод и его обслуживание.
 *
 * @param {Response} response ответ fetch
 */
export function syncServerTime(response) {
  const header = response?.headers?.get?.('date');
  if (!header) return;

  const serverMs = Date.parse(header);
  if (Number.isNaN(serverMs)) return;

  offsetMs = serverMs - Date.now();
  synced = true;
}

/** Сверялись ли хоть раз с сервером (для тестов и диагностики). */
export function isServerTimeSynced() {
  return synced;
}

/** Текущий момент по часам сервера. */
export function serverNow() {
  return new Date(Date.now() + offsetMs);
}

/**
 * Части московской даты и времени числами: показывать их в шаблоне надёжнее, чем
 * разбирать строку локали, а `getHours()` у Date вернул бы часовой пояс машины.
 *
 * @param {Date} [date] момент, по умолчанию текущий серверный
 * @returns {{year: number, month: number, day: number, hour: number, minute: number, second: number}}
 */
export function moscowParts(date = serverNow()) {
  const parts = new Intl.DateTimeFormat('ru-RU', {
    timeZone: MSK,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).formatToParts(date);

  const get = (type) => Number(parts.find((p) => p.type === type)?.value);
  return {
    year: get('year'),
    month: get('month'),
    day: get('day'),
    // В полночь ru-RU отдаёт час как «24», а не «00»: приводим к суточному кругу.
    hour: get('hour') % 24,
    minute: get('minute'),
    second: get('second'),
  };
}

/** Московская дата и время в виде ДД.ММ.ГГГГ ЧЧ:ММ:СС. */
export function formatMoscowDateTime(date = serverNow()) {
  const p = moscowParts(date);
  const pad = (n) => String(n).padStart(2, '0');
  return `${pad(p.day)}.${pad(p.month)}.${p.year} ${pad(p.hour)}:${pad(p.minute)}:${pad(p.second)}`;
}

/**
 * Произвольный формат даты по Москве - для мест, где нужны не части, а строка
 * локали (день словами в группировке истории, месяц с годом в шапке периода).
 *
 * Смысл обёртки в том, что `timeZone` нельзя забыть: без неё `toLocaleString`
 * молча берёт зону машины, и на компьютере в Москве ошибка незаметна.
 *
 * @param {Date} date момент
 * @param {Intl.DateTimeFormatOptions} options части даты и их вид
 * @returns {string}
 */
export function formatMoscow(date, options) {
  return new Intl.DateTimeFormat('ru-RU', { timeZone: MSK, ...options }).format(date);
}

/** Московский час текущего момента - для приветствия и суточных границ. */
export function moscowHour(date = serverNow()) {
  return moscowParts(date).hour;
}

/**
 * Подключает сверку часов к сетевому слою.
 *
 * Обёртка над `fetch`, а не строка в клиенте API: `client.js` стоит ровно на пороге
 * размера, и гейт не пускает туда даже двух строк - это его способ сказать, что файл
 * пора разгружать, а не дописывать. Заодно сверка ловит ответы всех запросов, включая
 * те, что идут мимо клиента.
 *
 * Читается только заголовок, ответ отдаётся вызывающему коду нетронутым.
 */
export function installClockSync() {
  const original = globalThis.fetch;
  if (typeof original !== 'function') return;

  globalThis.fetch = async (...args) => {
    const response = await original(...args);
    syncServerTime(response);
    return response;
  };
}
