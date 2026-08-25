import { apiRequest } from './client';

/**
 * Сквозной поиск по разделам системы.
 *
 * Раздел, на который у пользователя нет права, в ответ не приходит вовсе, поэтому
 * набор групп у разных людей разный -- интерфейс не должен рассчитывать на
 * фиксированный список.
 *
 * @param {string} query поисковый запрос, от 3 символов
 * @param {object} [options]
 * @param {AbortSignal} [options.signal] отмена устаревшего запроса
 * @param {number} [options.limit] результатов на раздел
 * @returns {Promise<{query: string, groups: Array, total: number, took_ms: number, degraded?: string[]}>}
 */
export async function globalSearch(query, options = {}) {
  const params = new URLSearchParams({ q: query });
  if (options.limit) params.set('limit', String(options.limit));

  // silent403 не нужен: путь добавлен в SILENT_403_PREFIXES клиента -- иначе снятое
  // посреди сессии право сыпало бы тостом на каждый введённый символ.
  const res = await apiRequest(`/search?${params.toString()}`, { signal: options.signal });
  const body = await res.json();
  return body.data ?? body;
}
