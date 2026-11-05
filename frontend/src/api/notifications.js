import { apiRequest, apiRequestRaw } from './client';

/** Refresh the current public projection on EVERY detail open, including read rows. */
export async function readNotificationDetails(id) {
  if (!Number.isSafeInteger(id) || id <= 0) throw new Error('Некорректное уведомление');
  const response = await apiRequest(`/notifications/${id}/read`, {
    method: 'PUT', body: JSON.stringify({ is_read: true }),
    // Only the still-current opening may notify; do not toast/retry in the client.
    silent403: true, silent: true, localServerErrors: true,
  });
  // apiRequest.json() already unwraps envelope.data.
  const notification = await response.json();
  if (!response.ok || !notification || notification.id !== id || notification.is_read !== true
    || typeof notification.title !== 'string' || typeof notification.message !== 'string'
    || !(notification.type === null || typeof notification.type === 'string')
    || !(notification.data === null || typeof notification.data === 'string')) {
    throw new Error('Не удалось загрузить подробности уведомления');
  }
  if (notification.data !== null) {
    const data = JSON.parse(notification.data);
    if (!data || typeof data !== 'object' || Array.isArray(data)) {
      throw new Error('Не удалось загрузить подробности уведомления');
    }
  }
  return notification;
}

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
export async function getNotificationsPaginated({ limit, offset, filter } = {}, requestOptions) {
  const query = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
    ...(filter ? { filter } : {}),
  }).toString();
  const path = `/notifications?${query}`;
  const res = await (requestOptions ? apiRequestRaw(path, requestOptions) : apiRequestRaw(path));
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
