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
