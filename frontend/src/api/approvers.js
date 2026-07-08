import { apiRequest } from './client';

/**
 * API клиент справочника принимающих заявки (#417).
 * unwrap бросает на !res.ok с сообщением бэка (fallback - если тела нет),
 * чтобы 4xx не прошёл как молчаливый успех.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

export async function getApprovers() {
  const res = await apiRequest('/application-approvers');
  return unwrap(res, 'Не удалось загрузить принимающих');
}

export async function getAllUsers() {
  const res = await apiRequest('/users/all');
  return unwrap(res, 'Не удалось загрузить пользователей');
}

export async function addApprover(userId) {
  const res = await apiRequest('/application-approvers', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId }),
  });
  return unwrap(res, 'Не удалось добавить принимающего');
}

/**
 * Задать/снять маску отображаемого имени принимающего. displayName === null или пустая
 * строка снимают маску (заявитель снова видит реальное ФИО).
 */
export async function updateApprover(id, displayName) {
  const res = await apiRequest(`/application-approvers/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ display_name: displayName }),
  });
  return unwrap(res, 'Не удалось сохранить отображаемое имя');
}

export async function deleteApprover(id) {
  const res = await apiRequest(`/application-approvers/${id}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось удалить принимающего');
}

/**
 * Глобальный аудит-лог принимающих (без :id - принимающие hard-delete,
 * история хранит снимок имени). Записи отсортированы новыми сверху.
 */
export async function getApproverHistory() {
  const res = await apiRequest('/application-approvers/history');
  return unwrap(res, 'Не удалось загрузить историю принимающих');
}
