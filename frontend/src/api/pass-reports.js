import { apiRequest } from './client';

/**
 * Живой отчёт по проходам таблицы за текущее незакрытое окно [последние 21:30, сейчас).
 * Возвращает { period_start, period_end, rows, totals }.
 */
export async function getPassReportLive(tableId) {
  const res = await apiRequest(`/system-tables/${tableId}/pass-report/live`);
  return res.json();
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
  return res.json();
}
