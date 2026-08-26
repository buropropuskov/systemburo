import { apiRequest } from './client';

/**
 * API типов пользователей - блокеры удаления (#1379 delete-blockers).
 * Список блокеров = ВСЕ пользователи типа (включая архивных, is_active=false),
 * т.к. гейт удаления типа считает всех. Действие переноса освобождает тип для
 * удаления. unwrap бросает на !res.ok с сообщением бэка (эталон api/approvers.js),
 * чтобы 4xx (системный источник / target не найден / == источнику) не прошёл
 * молчаливым успехом, а показался в notify.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Пользователи типа id, чьё существование блокирует удаление типа. Возвращает
 * ВСЕХ (активных и архивных), активные первыми (ORDER is_active DESC на бэке).
 * @param {number} id тип пользователя
 * @returns {Promise<Array<{id:number, username:string, last_name:string,
 *   first_name:string, middle_name:string, position:string, is_active:boolean}>>}
 */
export async function getUserTypeBlockingUsers(id) {
  const res = await apiRequest(`/user-types-management/${id}/blocking-users`);
  return unwrap(res, 'Не удалось загрузить пользователей типа');
}

/**
 * Перенести всех пользователей типа id в целевой тип targetTypeId, освобождая
 * исходный для удаления. Идемпотентно (0 пользователей -> reassigned:0).
 * Системный тип нельзя переносить (BE отдаёт 400).
 * @param {number} id исходный тип
 * @param {number} targetTypeId целевой тип
 * @returns {Promise<{reassigned: number}>}
 */
export async function reassignUserTypeUsers(id, targetTypeId) {
  const res = await apiRequest(`/user-types-management/${id}/reassign-users`, {
    method: 'POST',
    body: JSON.stringify({ target_type_id: targetTypeId }),
  });
  return unwrap(res, 'Не удалось перенести пользователей');
}
