import { apiRequest } from '@/api/client';

/**
 * ФИО текущего пользователя для подписи выгрузок («Отчёт сформировал»).
 *
 * Одинаковый запрос стоял в каждом экране управления справочниками. Имя здесь -
 * необязательная деталь: не ответил сервер или нет такого поля - подпись просто
 * останется дефолтной, поэтому ошибка гасится, а не поднимается наверх.
 *
 * @returns {Promise<string>} ФИО, логин или пустая строка
 */
export async function fetchCurrentUserName() {
  try {
    const res = await apiRequest('/users/me');
    if (!res.ok) return '';
    const user = await res.json();
    return [user.last_name, user.first_name, user.middle_name].filter(Boolean).join(' ') || user.username || '';
  } catch {
    return '';
  }
}
