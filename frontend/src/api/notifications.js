import { apiRequest, apiRequestRaw } from './client';

/**
 * Уведомления порциями (#1748 S7): `GET /notifications` без параметров отдаёт
 * как раньше плоский массив (обратная совместимость), с limit/offset/filter -
 * пагинированный ответ. Пагинация лежит в envelope.meta рядом с data, а
 * apiRequest снимает только data и meta теряется - поэтому читаем сырой ответ
 * через apiRequestRaw (см. getApplicationsPaginated/getUserApplicationsPaginated).
 * meta.unread_count - число непрочитанных НЕЗАВИСИМО от текущей страницы/фильтра,
 * его и берёт счётчик в шапке (длина загруженного массива для этого не годится -
 * при постраничной подгрузке она не отражает весь набор).
 * @param {{limit: number, offset: number, filter?: 'all'|'unread'}} params
 * @returns {Promise<{items: object[], total: number, unreadCount: number}>}
 */
export async function getNotificationsPaginated({ limit, offset, filter } = {}) {
  const query = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
    ...(filter ? { filter } : {}),
  }).toString();
  const res = await apiRequestRaw(`/notifications?${query}`);
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось загрузить уведомления');
  }
  return {
    items: body.data || [],
    total: (body.meta && body.meta.total) || 0,
    unreadCount: (body.meta && body.meta.unread_count) || 0,
  };
}

/**
 * Отмечает прочитанными все уведомления текущего пользователя одним запросом.
 * @returns {Promise<number>} число затронутых уведомлений
 */
export async function markAllNotificationsRead() {
  const response = await apiRequest('/notifications/read-all', { method: 'PUT' });
  const data = await response.json();
  if (!response.ok) {
    throw new Error(data?.message || 'Не удалось отметить уведомления прочитанными');
  }
  return typeof data === 'number' ? data : 0;
}
