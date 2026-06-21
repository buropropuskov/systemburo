import { apiRequest } from './client';

/**
 * Создать обращение обратной связи.
 * @param {{ message: string }} data
 * @returns {Promise<object>} данные созданного обращения
 */
export async function createFeedback(data) {
  const res = await apiRequest('/feedback', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error('Не удалось отправить обращение');
  return res.json();
}

/**
 * Получить все обращения (для администраторов).
 * @returns {Promise<object[]>}
 */
export async function getAllFeedback() {
  const res = await apiRequest('/feedback/all');
  if (!res.ok) throw new Error('Не удалось загрузить обращения');
  return res.json();
}

/**
 * Получить обращения текущего пользователя.
 * @returns {Promise<object[]>}
 */
export async function getMyFeedback() {
  const res = await apiRequest('/feedback/my');
  if (!res.ok) throw new Error('Не удалось загрузить обращения');
  return res.json();
}

/**
 * Получить статистику по обращениям.
 * @returns {Promise<object>}
 */
export async function getFeedbackStats() {
  const res = await apiRequest('/feedback/stats');
  if (!res.ok) throw new Error('Не удалось загрузить статистику обращений');
  return res.json();
}

/**
 * Обновить статус обращения. При переводе в "Решено" можно передать ответ заявителю.
 * @param {number} id
 * @param {{ status: string, comment?: string }} data
 * @returns {Promise<object>}
 */
export async function updateFeedbackStatus(id, data) {
  const res = await apiRequest(`/feedback/${id}/status`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error('Не удалось обновить статус обращения');
  return res.json();
}

/**
 * Отметить обращение прочитанным/непрочитанным.
 * @param {number} id
 * @param {boolean} [isRead=true]
 * @returns {Promise<object>}
 */
export async function markFeedbackAsRead(id, isRead = true) {
  const res = await apiRequest(`/feedback/${id}/read`, {
    method: 'PUT',
    body: JSON.stringify({ is_read: isRead }),
  });
  if (!res.ok) throw new Error('Не удалось обновить статус прочтения');
  return res.json();
}
