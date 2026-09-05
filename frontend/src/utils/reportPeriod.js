/** Период отчёта: пресеты, преобразование дат мастера и границы данных с бэкенда. */
import { getReportDataPeriod } from '@/api/statistics';

// Период хранится в форме как ISO YYYY-MM-DD (формат бэка), а DateFilter работает с
// Date-объектами в локальной зоне. Разбираем и собираем по календарным частям, не
// через toISOString, чтобы не словить сдвиг даты на границе суток.

/**
 * @param {Date} dt
 * @returns {string} дата в ISO YYYY-MM-DD
 */
export function dateToIso(dt) {
  return `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`;
}

/**
 * @param {string} iso
 * @returns {Date|null} null - пустая или неполная дата
 */
export function isoToDate(iso) {
  if (!iso) return null;
  const [y, m, d] = String(iso).split('-').map(Number);
  if (!y || !m || !d) return null;
  return new Date(y, m - 1, d);
}

/**
 * Диапазон дат для пресета периода. «all» -> пустой диапазон: границы данных знает
 * только бэкенд, их подставляет fetchReportPeriodBounds.
 *
 * @param {'week'|'month'|'year'|'all'} kind
 * @param {Date} [now] точка отсчёта (в тестах - фиксированная)
 * @returns {{from: string, to: string}}
 */
export function computePeriodRange(kind, now = new Date()) {
  if (kind === 'all') return { from: '', to: '' };
  const to = dateToIso(now);
  if (kind === 'week') {
    const monday = new Date(now);
    monday.setDate(now.getDate() - ((now.getDay() + 6) % 7)); // Пн = начало недели
    return { from: dateToIso(monday), to };
  }
  if (kind === 'month') return { from: dateToIso(new Date(now.getFullYear(), now.getMonth(), 1)), to };
  if (kind === 'year') return { from: dateToIso(new Date(now.getFullYear(), 0, 1)), to };
  return { from: '', to: '' };
}

/**
 * Границы дат отчёта для пресета «Весь период». Неполный ответ и сбой запроса дают
 * null: тогда остаётся прежнее поведение «без ограничения по датам» - отчёт строится
 * по всем записям, просто календарь пуст. Ронять из-за подписи построение незачем.
 *
 * @param {{mode?: string, metric?: string, entity?: string}} params
 * @returns {Promise<{from: string, to: string}|null>}
 */
export async function fetchReportPeriodBounds(params) {
  try {
    const range = await getReportDataPeriod(params);
    return range?.from && range?.to ? { from: range.from, to: range.to } : null;
  } catch {
    return null;
  }
}
