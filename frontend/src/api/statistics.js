import { apiRequest } from './client';

/**
 * Получить сводку метрик за период.
 * @param {string} from - дата начала в формате YYYY-MM-DD
 * @param {string} to   - дата конца в формате YYYY-MM-DD
 * @returns {Promise<{
 *   total_applications: number,
 *   by_attachment_type: Array<{name: string, count: number}>,
 *   by_status: Array<{status: string, count: number}>,
 *   processed: number,
 *   in_work: number,
 *   cars_entered: number,
 *   avg_cars_per_day: number,
 *   people_entered: number,
 *   items_sum: number,
 *   cars_on_territory: number,
 *   people_on_territory: number,
 *   users_online: number,
 *   active_users: number,
 *   banned_users: number,
 *   open_feedback: number,
 *   active_unload_places: number,
 *   blacklist_cars: number,
 *   blacklist_people: number,
 *   unique_cars: number,
 *   unique_people: number,
 * }>}
 */
export async function getSummary(from, to) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await apiRequest(`/statistics/summary?${params}`);
  return res.json();
}

/**
 * Получить временной ряд метрики.
 * @param {{from: string, to: string, metric: string, granularity: string}} opts
 *   metric: 'applications' | 'people_entries' | 'car_entries'
 *   granularity: 'day' | 'week' | 'month'
 * @returns {Promise<Array<{date: string, count: number}>>}
 */
export async function getTimeline({ from, to, metric, granularity }) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (metric) params.set('metric', metric);
  if (granularity) params.set('granularity', granularity);
  const res = await apiRequest(`/statistics/timeline?${params}`);
  return res.json();
}

/**
 * Получить последние проходы/проезды.
 * @param {number} [limit=15] - максимальное количество записей на тип
 * @returns {Promise<{
 *   people: Array<{subject: string, organization: string, place: string, created_at: string, action_type: 'entry'|'exit'}>,
 *   cars:   Array<{subject: string, mark: string, organization: string, place: string, created_at: string, action_type: 'entry'|'exit'}>,
 * }>}
 */
export async function getRecentPassages(limit = 15) {
  const res = await apiRequest(`/statistics/recent-passages?limit=${limit}`);
  return res.json();
}

/**
 * Получить готовые инсайты за период: пик нагрузки по часам, сравнение с
 * предыдущим периодом равной длины, топ мест и организаций, тренды по дням.
 * @param {string} from - дата начала в формате YYYY-MM-DD
 * @param {string} to   - дата конца в формате YYYY-MM-DD
 * @returns {Promise<{
 *   peak_hours: Array<{metric: string, label: string, unit?: string, peak_hour: number, peak_value: number, hourly: Array<{hour: number, value: number}>}>,
 *   comparisons: Array<{metric: string, label: string, unit?: string, current: number, previous: number, delta_pct: number, direction: 'up'|'down'|'flat'}>,
 *   top_places: Array<{metric: string, label: string, value: number}>,
 *   top_orgs: Array<{metric: string, label: string, value: number}>,
 *   trends: Array<{metric: string, label: string, direction: 'up'|'down'|'flat', series: number[]}>,
 * }>}
 */
export async function getInsights(from, to) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await apiRequest(`/statistics/insights?${params}`);
  if (!res.ok) {
    // На 5xx baseRequest уже редиректит на /500 и не отдаёт JSON-тело —
    // парсим защищённо, чтобы не словить SyntaxError на HTML-странице ошибки.
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось загрузить инсайты');
  }
  return res.json();
}

/**
 * Получить каталог конструктора отчётов: whitelist метрик, разрезов, фильтров
 * и list-сущностей с подставленными значениями динамических справочников.
 * @returns {Promise<{
 *   metrics: Array<{key: string, label: string, unit?: string, dimensions: string[]}>,
 *   dimensions: Array<{key: string, label: string}>,
 *   filters: Array<{key: string, label: string, type: 'date'|'enum'|'dict', options?: Array<{value: string, label: string}>}>,
 *   list_entities: Array<{key: string, label: string, columns: Array<{key: string, label: string}>, filters: string[]}>,
 *   granularities: Array<{value: string, label: string}>,
 * }>}
 */
export async function getReportCatalog() {
  const res = await apiRequest('/statistics/metrics');
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось загрузить каталог отчётов');
  return data;
}

/**
 * Исполнить отчёт конструктора.
 * @param {{
 *   mode: 'aggregate'|'list', metric?: string, dimension?: string, granularity?: string,
 *   entity?: string, filters?: Array<{key: string, values?: string[], from?: string, to?: string}>,
 *   sort?: string, limit?: number
 * }} request
 * @returns {Promise<object>} ReportResponse (aggregate) или ReportListResponse (list)
 */
export async function runReport(request) {
  const res = await apiRequest('/statistics/report', {
    method: 'POST',
    body: JSON.stringify(request),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось построить отчёт');
  return data;
}

/**
 * @typedef {object} ReportTemplate
 * @property {number} id
 * @property {string} name
 * @property {string} [description]
 * @property {object} config   снимок состояния конструктора
 * @property {boolean} is_system
 * @property {boolean} is_shared
 * @property {number} [owner_user_id]
 */

/**
 * Список доступных шаблонов отчётов: системные пресеты + личные + расшаренные.
 * @returns {Promise<ReportTemplate[]>}
 */
export async function getReportTemplates() {
  const res = await apiRequest('/statistics/templates');
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось загрузить шаблоны');
  return data;
}

/**
 * Сохранить личный шаблон отчёта.
 * @param {{ name: string, description?: string, config: object, is_shared?: boolean }} payload
 * @returns {Promise<ReportTemplate>}
 */
export async function saveReportTemplate(payload) {
  const res = await apiRequest('/statistics/templates', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось сохранить шаблон');
  return data;
}

/**
 * Удалить личный шаблон отчёта.
 * @param {number} id
 * @returns {Promise<void>}
 */
export async function deleteReportTemplate(id) {
  const res = await apiRequest(`/statistics/templates/${id}`, { method: 'DELETE' });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось удалить шаблон');
  }
}
