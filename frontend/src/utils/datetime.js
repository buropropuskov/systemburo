/**
 * Форматирует ISO-дату в "дд.ММ.гггг ЧЧ:мм". Пустое/невалидное возвращает как есть.
 * @param {string|Date|null|undefined} value
 * @returns {string}
 */
export function formatDateTime(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(d.getDate())}.${p(d.getMonth() + 1)}.${d.getFullYear()} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

/**
 * Относительное «N назад» от now до момента value. Для last_seen онлайна и лент.
 * Будущие/нулевые значения -> 'только что'. Считается в абсолютных интервалах
 * (UTC-инстант), таймзона не влияет на «сколько прошло».
 * @param {string|Date|null|undefined} value
 * @returns {string}
 */
export function formatTimeAgo(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  const diffMs = Date.now() - d.getTime();
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

/**
 * Форматирует ячейку отчёта-выгрузки (mode=list) ПО ТИПУ колонки (column.type с
 * бэка): 'date'/'datetime' -> ISO-даты 'ГГГГ-ММ-ДД' в 'дд.мм.гггг'; 'time'/'datetime'
 * -> убрать секунды 'ЧЧ:ММ:СС' -> 'ЧЧ:ММ' (секунды в отчётах не нужны). Бэк отдаёт
 * склеенные строки-диапазоны ('2026-06-20 - 2026-06-21', '00:01:00 - 23:59:00'),
 * поэтому правим все подстроки. Колонки без типа (номер, ФИО, организация,
 * произвольный текст) возвращаем как есть — иначе дата внутри текста была бы испорчена.
 * @param {string|number|null|undefined} value
 * @param {string} [type] — тип колонки: 'date' | 'time' | 'datetime' | прочее
 * @returns {string}
 */
export function formatReportCell(value, type) {
  if (value == null || value === '') return '';
  let s = String(value);
  if (type === 'date' || type === 'datetime') {
    s = s.replace(/(\d{4})-(\d{2})-(\d{2})/g, '$3.$2.$1');
  }
  if (type === 'time' || type === 'datetime') {
    s = s.replace(/(\d{1,2}:\d{2}):\d{2}/g, '$1');
  }
  return s;
}
