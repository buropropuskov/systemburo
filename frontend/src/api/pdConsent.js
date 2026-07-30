import { apiRequest } from './client';

/**
 * API-клиент согласия на обработку персональных данных, которое запрашивается у
 * пользователя при первом входе (#1567): администраторские настройки (текст,
 * требуемая редакция, выключатель запроса) и состояние гейта для самого
 * пользователя.
 * @typedef {{text: string, version: number, required: boolean}} PDConsentSettings
 * @typedef {{stored_name: string, file_name: string, mime_type: string, ext: string, uploaded_at: string}} PDConsentDocument
 * @typedef {{required: boolean, version: number, text: string, document: PDConsentDocument|null}} PDConsentGateState
 */

const BASE = '/settings/pd-consent';
const CONSENTS = '/consents';

/**
 * Разворачивает ответ, доставая сообщение об ошибке из envelope. Ключ ошибки в
 * envelope - `error`, но `wrapJsonUnwrap` в client.js уже переложил его в
 * `message`, поэтому читаем именно `message` и не трогаем `res.text()`.
 * @param {Response} res
 * @param {string} fallback
 * @returns {Promise<any>}
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

/**
 * Состояние согласия для текущего пользователя: спрашивать ли его, какой
 * редакции и какой текст показать. Исключения (супер-администратор, пустой
 * текст, выключенный запрос) сервер учитывает сам в поле `required` - фронт эти
 * правила у себя не повторяет, иначе они разъедутся.
 * @returns {Promise<PDConsentGateState>}
 */
export async function getConsentGate() {
  const res = await apiRequest(`${CONSENTS}/gate`);
  return unwrap(res, 'Не удалось проверить согласие на обработку данных');
}

/**
 * Записывает согласие текущего пользователя на текущую редакцию текста.
 * Редакцию и хэш штампует сервер, поэтому осмысленного тела у запроса нет.
 * @returns {Promise<PDConsentGateState>}
 */
export async function acceptConsent() {
  const res = await apiRequest(`${CONSENTS}/accept`, {
    method: 'POST',
    body: '{}',
  });
  return unwrap(res, 'Не удалось подтвердить согласие');
}
