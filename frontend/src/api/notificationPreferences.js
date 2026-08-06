import { apiRequest } from './client';

/**
 * API клиент тонкой настройки уведомлений (#1748, S8).
 * unwrap бросает на !res.ok с сообщением бэка (fallback - если тела нет),
 * чтобы 4xx/5xx не прошли как молчаливый успех.
 */
async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Каталог типов уведомлений с эффективным состоянием пользователя (плоский
 * список - категория лежит в поле каждого элемента, группировка на экране).
 */
export async function getNotificationPreferences() {
  const res = await apiRequest('/notifications/preferences');
  return unwrap(res, 'Не удалось загрузить настройки уведомлений');
}

/**
 * Батч изменений: items = [{type_code, enabled}]. Обязательные типы бэк
 * отклонит 400 - на экран они не попадают вовсе (фильтруются до вызова).
 */
export async function updateNotificationPreferences(items) {
  const res = await apiRequest('/notifications/preferences', {
    method: 'PUT',
    body: JSON.stringify({ items }),
  });
  return unwrap(res, 'Не удалось сохранить настройки уведомлений');
}
