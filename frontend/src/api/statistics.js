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
