import { apiRequest } from './client';

/**
 * API темы оформления текущего пользователя (#1415). Тема хранится в профиле,
 * чтобы ехать за человеком между устройствами; localStorage - лишь немедленный
 * применитель до ответа сервера.
 *
 * unwrap бросает на !res.ok с сообщением бэка (envelope кладёт текст в message),
 * чтобы 4xx не прошёл молчаливым успехом.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * @returns {Promise<{ theme: string | null }>} null - юзер тему не выбирал
 */
export async function getTheme() {
  const res = await apiRequest('/users/me/theme');
  return unwrap(res, 'Не удалось загрузить тему оформления');
}

/**
 * @param {string} theme id темы из реестра utils/theme.js
 * @returns {Promise<{ message: string }>}
 */
export async function saveTheme(theme) {
  const res = await apiRequest('/users/me/theme', {
    method: 'PUT',
    body: JSON.stringify({ theme }),
  });
  return unwrap(res, 'Не удалось сохранить тему оформления');
}
