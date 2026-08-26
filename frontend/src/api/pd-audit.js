import { apiRequest } from './client';

/**
 * Журнал доступа к персональным данным (152-ФЗ, #1472).
 * Только чтение: записи по закону не удаляются, сроком хранения занимаются партиции.
 */

/**
 * Страница журнала с фильтрами.
 * @param {{page?: number, limit?: number, username?: string, action?: string,
 *          resource?: string, only_denied?: boolean, from?: string, to?: string}} params
 * @returns {Promise<{items: Array, total: number, page: number, limit: number}>}
 */
export async function listPDAudit(params = {}) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '' && v !== false) qs.set(k, v);
  });
  const url = '/pd-audit' + (qs.size ? `?${qs}` : '');
  const res = await apiRequest(url);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body?.message || 'Не удалось загрузить журнал');
  }
  return res.json();
}
