import { moscowParts, serverNow } from '@/utils/serverTime';

const WEEKDAY_NAMES_RU = [
  'Воскресенье', 'Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота',
];

/**
 * Форматирует ISO-момент в "дд.ММ.гггг ЧЧ:мм" ПО МОСКВЕ. Пустое/невалидное
 * возвращает как есть.
 *
 * Зона московская, а не машины (#2298): система живёт в одном поясе, её бэкенд
 * считает в нём сроки и границу отчётных суток, и работник поста в другом поясе
 * должен видеть то же время, что все остальные. Раньше здесь стоял getHours(),
 * и время отметки на экране расходилось с временем, по которому её записал сервер.
 *
 * @param {string|Date|null|undefined} value
 * @returns {string}
 */
export function formatDateTime(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  const m = moscowParts(d);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(m.day)}.${p(m.month)}.${m.year} ${p(m.hour)}:${p(m.minute)}`;
}

/**
 * Дата момента по Москве в виде дд.ММ.гггг - без времени.
 *
 * Отдельно от formatDateRu: тот разбирает строку 'YYYY-MM-DD' (день без времени и
 * без зоны), а здесь на входе момент, и день у него зависит от зоны показа. Ночная
 * загрузка файла на машине восточнее Москвы иначе датируется следующим днём.
 *
 * @param {string|Date|null|undefined} value
 * @returns {string}
 */
export function formatMomentDate(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  const m = moscowParts(d);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(m.day)}.${p(m.month)}.${m.year}`;
}

/**
 * Название дня недели по московской дате момента. Индексация от воскресенья -
 * как у Date.getDay(), а не как в расписаниях, где неделя начинается с понедельника.
 *
 * День берётся из московских частей, а не у Date напрямую: заявка, отправленная в
 * понедельник в 00:30 МСК, на машине западнее показала бы воскресенье.
 *
 * @param {string|Date|null|undefined} value
 * @returns {string} 'Среда' или '' для пустого/невалидного значения
 */
export function weekdayName(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  const m = moscowParts(d);
  return WEEKDAY_NAMES_RU[new Date(Date.UTC(m.year, m.month - 1, m.day)).getUTCDay()];
}

/**
 * ISO-момент -> значение для `<input type="datetime-local">` (локальное время
 * без зоны). Через части даты, а не toISOString(): срез ISO-строки увёл бы
 * время на UTC и показал бы админу не тот час, который он выбрал.
 * @param {string|null|undefined} iso
 * @returns {string} 'YYYY-MM-DDTHH:mm' или ''
 */
export function isoToLocalInput(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

/**
 * Значение `<input type="datetime-local">` -> ISO-момент для API. Браузер
 * трактует строку без зоны как локальное время, поэтому смещение учтено.
 * @param {string|null|undefined} value
 * @returns {string} ISO-строка или ''
 */
export function localInputToIso(value) {
  if (!value) return '';
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? '' : d.toISOString();
}

/**
 * Относительное «N назад» от now до момента value. Для last_seen онлайна и лент.
 * Будущие/нулевые значения -> 'только что'. Считается в абсолютных интервалах
 * (UTC-инстант), таймзона не влияет на «сколько прошло».
 *
 * Точка отсчёта - серверные часы, а не машины (#2298): у пользователя со
 * спешащими часами свежая отметка получала бы возраст в часы, и «кто сейчас
 * онлайн» показывало бы давно ушедших.
 * @param {string|Date|null|undefined} value
 * @returns {string}
 */
export function formatTimeAgo(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  const diffMs = serverNow().getTime() - d.getTime();
  if (diffMs < 60000) return 'только что';
  const mins = Math.floor(diffMs / 60000);
  if (mins < 60) return `${mins} мин назад`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} ч назад`;
  const days = Math.floor(hours / 24);
  if (days === 1) return 'вчера';
  return `${days} дн назад`;
}

/**
 * Форматирует date-only строку 'YYYY-MM-DD' (период отчёта: день/неделя/месяц
 * приходят как to_char 'YYYY-MM-DD') в 'дд.мм.гггг'. Разбираем вручную, без
 * new Date(), чтобы date-only не съехала на UTC-полночь (-3ч в МСК). Значение,
 * не похожее на дату (название статуса, организации), возвращаем как есть.
 * @param {string|null|undefined} value
 * @returns {string}
 */
export function formatDateRu(value) {
  if (!value) return '';
  const s = String(value).slice(0, 10);
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  return m ? `${m[3]}.${m[2]}.${m[1]}` : String(value);
}

const MONTH_NAMES_RU = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

/**
 * Форматирует период 'YYYY-MM' (разбивка файлового архива по месяцам) в
 * 'Месяц ГГГГ'. Разбор вручную, как в formatDateRu — new Date('YYYY-MM')
 * трактуется как UTC-полночь и в МСК съезжает на предыдущий месяц.
 * @param {string|null|undefined} value
 * @returns {string}
 */
export function formatMonthRu(value) {
  const m = /^(\d{4})-(\d{2})$/.exec(String(value || ''));
  if (!m) return String(value || '');
  const name = MONTH_NAMES_RU[Number(m[2]) - 1];
  return name ? `${name} ${m[1]}` : String(value);
}

/**
 * Форматирует ячейку отчёта-выгрузки (mode=list) ПО ТИПУ колонки (column.type с
 * бэка): 'date'/'datetime' -> ISO-даты 'ГГГГ-ММ-ДД' в 'дд.мм.гггг'; 'time'/'datetime'
 * -> убрать секунды 'ЧЧ:ММ:СС' -> 'ЧЧ:ММ' (секунды в отчётах не нужны). Бэк отдаёт
 * склеенные строки-диапазоны ('2026-06-20 - 2026-06-21', '00:01:00 - 23:59:00'),
 * поэтому правим все подстроки. Колонки без типа (номер, ФИО, организация,
 * произвольный текст) возвращаем как есть — иначе дата внутри текста была бы испорчена.
 * Тип 'duration' — длительность в СЕКУНДАХ (метрики обработки заявок, #1240):
 * 8100 -> '2 ч 15 мин'. Отсутствие значения у duration-колонки означает «этап не
 * пройден», а не ноль (см. formatDuration).
 * @param {string|number|null|undefined} value
 * @param {string} [type] — тип колонки: 'date' | 'time' | 'datetime' | 'duration' | прочее
 * @returns {string}
 */
export function formatReportCell(value, type) {
  if (value == null || value === '') return '';
  if (type === 'duration') return formatDuration(value);
  let s = String(value);
  if (type === 'date' || type === 'datetime') {
    s = s.replace(/(\d{4})-(\d{2})-(\d{2})/g, '$3.$2.$1');
  }
  if (type === 'time' || type === 'datetime') {
    s = s.replace(/(\d{1,2}:\d{2}):\d{2}/g, '$1');
  }
  return s;
}

/**
 * Длительность в секундах -> человекочитаемое: '<1 мин', '45 мин', '2 ч 15 мин',
 * '3 сут 4 ч'. Единицы округляются вниз, чтобы остаток не переполнял старшую
 * ('1 ч 60 мин' невозможно). Ноль осмыслен и показывается как '0 мин': движок
 * COALESCE'ит пустое окно в 0 при разрезе 'none', а «нет данных» приходит
 * отсутствием ключа в values — его отличает вызывающий и рисует «—» (контракт
 * metricOmitsFakeZero, #1240 B2). Нечисловое возвращаем как есть.
 * @param {number|string|null|undefined} seconds
 * @returns {string}
 */
export function formatDuration(seconds) {
  if (seconds == null || seconds === '') return '';
  const total = Number(seconds);
  if (!Number.isFinite(total)) return String(seconds);

  const sign = total < 0 ? '-' : '';
  const abs = Math.floor(Math.abs(total));
  if (abs === 0) return '0 с';
  // Секунды на коротких интервалах: без них реальные «6 секунд» показывались как
  // «0 мин»/«<1 мин», и метрика читалась как сломанная (#1251 polish, п.3 - на
  // проходной согласование и принятие часто делаются одним действием).
  if (abs < 60) return `${sign}${abs} с`;

  const days = Math.floor(abs / 86400);
  const hours = Math.floor((abs % 86400) / 3600);
  const minutes = Math.floor((abs % 3600) / 60);
  const secs = abs % 60;

  if (days > 0) return hours > 0 ? `${sign}${days} сут ${hours} ч` : `${sign}${days} сут`;
  if (hours > 0) return minutes > 0 ? `${sign}${hours} ч ${minutes} мин` : `${sign}${hours} ч`;
  // До часа секунды ещё информативны, на часах и сутках — уже шум.
  return secs > 0 ? `${sign}${minutes} мин ${secs} с` : `${sign}${minutes} мин`;
}
