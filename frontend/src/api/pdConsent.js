import { apiRequest } from './client';

/**
 * API-клиент настроек согласия на обработку персональных данных, которое
 * запрашивается у пользователя при первом входе (#1567). Пока это только
 * администраторская часть: текст, требуемая версия и выключатель запроса.
 * @typedef {{text: string, version: number, required: boolean}} PDConsentSettings
 */

const BASE = '/settings/pd-consent';

/**
 * Разворачивает ответ, доставая сообщение об ошибке из envelope.
 * @param {Response} res
 * @param {string} fallback
 * @returns {Promise<PDConsentSettings>}
 */
async function unwrap(res, fallback) {
  const body = await res.json().catch(() => null);
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Текущие настройки согласия.
 * @returns {Promise<PDConsentSettings>}
 */
export async function getPDConsentSettings() {
  const res = await apiRequest(BASE);
  return unwrap(res, 'Не удалось загрузить настройки согласия');
}

/**
 * Сохраняет текст согласия. Версию не двигает: правка опечатки не должна
 * заставлять всех соглашаться заново.
 * @param {string} text HTML согласия; пустая строка означает очистку
 * @returns {Promise<PDConsentSettings>}
 */
export async function savePDConsentText(text) {
  const res = await apiRequest(`${BASE}/text`, {
    method: 'PUT',
    body: JSON.stringify({ text }),
  });
  return unwrap(res, 'Не удалось сохранить текст согласия');
}

/**
 * Включает или выключает запрос согласия при входе. Включение с пустым
 * текстом сервер отклоняет.
 * @param {boolean} required
 * @returns {Promise<PDConsentSettings>}
 */
export async function setPDConsentRequired(required) {
  const res = await apiRequest(`${BASE}/required`, {
    method: 'PUT',
    body: JSON.stringify({ required }),
  });
  return unwrap(res, 'Не удалось изменить настройку согласия');
}

/**
 * Поднимает требуемую версию согласия: система запросит его заново у всех, кто
 * соглашался с прежней редакцией текста.
 * @returns {Promise<PDConsentSettings>}
 */
export async function requirePDConsentAgain() {
  const res = await apiRequest(`${BASE}/require-again`, {
    method: 'POST',
    body: '{}',
  });
  return unwrap(res, 'Не удалось запросить согласие заново');
}
