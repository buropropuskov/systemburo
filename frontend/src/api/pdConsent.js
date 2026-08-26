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
 * Сохраняет текст согласия. Редакцию двигает только по requireAgain, и тем же
 * запросом: отдельный вызов мог бы не дойти, оставив новый текст со старой
 * редакцией - то есть согласие, данное не тому тексту.
 * @param {string} text HTML согласия; пустая строка означает очистку
 * @param {boolean} [requireAgain] поднять редакцию, чтобы согласие подтвердили заново
 * @returns {Promise<PDConsentSettings>}
 */
export async function savePDConsentText(text, requireAgain = false) {
  const res = await apiRequest(`${BASE}/text`, {
    method: 'PUT',
    body: JSON.stringify({ text, require_again: requireAgain }),
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
 * Отзывает согласие работника по его обращению к администратору. Своей кнопки
 * отзыва у работника нет, поэтому исполнить просьбу может только администратор.
 * После отзыва система снова закрывает человеку доступ до нового подтверждения.
 * @param {string} username логин работника
 * @returns {Promise<{message?: string}>}
 */
export async function revokeUserConsent(username) {
  const res = await apiRequest(`/users/${encodeURIComponent(username)}/consent`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось отозвать согласие');
}

/**
 * Согласия текущего работника: по ним личный кабинет показывает, что и когда он
 * подтвердил, и даёт отозвать.
 * @typedef {{id: number, consent_type: string, granted: boolean, granted_at: string,
 *   revoked_at: ?string, document_version: number}} PDConsentRecord
 * @returns {Promise<PDConsentRecord[]>}
 */
export async function listMyConsents() {
  const res = await apiRequest(CONSENTS);
  return unwrap(res, 'Не удалось загрузить сведения о согласии');
}

/**
 * Отзывает собственное согласие работника. После отзыва система снова закроет ему
 * доступ и покажет окно согласия.
 * @param {string} [type] вид согласия
 * @returns {Promise<{message?: string}>}
 */
export async function revokeMyConsent(type = 'pd_processing') {
  const res = await apiRequest(`${CONSENTS}/${type}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось отозвать согласие');
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

/**
 * Сводка по сбору согласий текущей редакции и список тех, кто ещё не подтвердил.
 * Считается той же меркой, что и гейт, поэтому число согласившихся совпадает с
 * числом тех, кого система пускает.
 * @typedef {{id: number, username: string, full_name: string, organization: string}} PDConsentPendingUser
 * @typedef {{active: boolean, version: number, total: number, accepted: number, pending: number, pending_users: PDConsentPendingUser[], truncated: boolean}} PDConsentCollection
 * @param {{full?: boolean}} [opts] full - вернуть список не подтвердивших целиком
 *   (нужно выгрузке: урезанный список означал бы потерю людей в файле)
 * @returns {Promise<PDConsentCollection>}
 */
export async function getPDConsentCollection({ full = false } = {}) {
  const res = await apiRequest(`${BASE}/collection${full ? '?full=1' : ''}`);
  return unwrap(res, 'Не удалось загрузить сводку по сбору согласий');
}
