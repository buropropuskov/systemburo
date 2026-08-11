import { apiRequest } from './client';

/**
 * Войти в систему от имени пользователя (#1912). Возвращает маркер доступа со
 * своим сроком жизни и данные того, от чьего имени открыт сеанс.
 *
 * @param {number} userId
 * @returns {Promise<{ token: string, expires_at: string, target: { id: number, username: string, full_name: string } }>}
 */
export async function startImpersonation(userId) {
  const res = await apiRequest(`/users/${userId}/impersonate`, { method: 'POST' });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.message || 'Не удалось войти от имени пользователя');
  }
  return res.json();
}

/**
 * Закрыть сеанс работы от чужого имени записью в журнал. Вызывается маркером
 * режима: свою учётную запись клиент возвращает отдельно, обновлением маркера.
 * Ошибка не должна мешать возврату - её глотает вызывающий стор.
 *
 * @returns {Promise<Response>}
 */
export async function stopImpersonation() {
  return apiRequest('/impersonation/stop', { method: 'POST', silent403: true });
}
