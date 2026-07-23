import { apiRequest } from './client';

/**
 * API суточных отчётов охранника по проходам. unwrap бросает на !res.ok с
 * сообщением бэка (паттерн approvers.js): wrapJsonUnwrap при ошибке РЕЗОЛВИТ
 * {message}, и голый res.json() отдал бы truthy-объект без totals/rows как успех.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Живой отчёт по проходам таблицы за текущее незакрытое окно [последние 21:30, сейчас).
 * Возвращает { period_start, period_end, rows, totals }.
 */
export async function getPassReportLive(tableId) {
  const res = await apiRequest(`/system-tables/${tableId}/pass-report/live`);
  return unwrap(res, 'Не удалось загрузить отчёт');
}

/**
 * Сохранённые суточные отчёты таблицы. from/to - строки YYYY-MM-DD по report_date,
 * без фильтра бэк отдаёт последний месяц. Возвращает { days: [...] }.
 */
export async function listPassReports(tableId, { from, to } = {}) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const qs = params.toString();
  const res = await apiRequest(`/system-tables/${tableId}/pass-reports${qs ? `?${qs}` : ''}`);
  return unwrap(res, 'Не удалось загрузить историю отчётов');
}
