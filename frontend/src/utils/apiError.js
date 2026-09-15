import { useDeletionsStore } from '@/stores/deletions';

/**
 * Сообщение по коду состояния - когда объяснения в теле нет или оно непригодно.
 *
 * Отдельно названы коды шлюза: при перезапуске сервера человеку важно понять, что
 * ошибка временная и повторить попытку, а не искать причину в своих действиях.
 *
 * @param {number} [status]
 * @param {string} [fallback]
 * @returns {string}
 */
export function describeHttpStatus(status, fallback = 'неизвестная ошибка') {
  if (status === 502 || status === 503 || status === 504) {
    return 'сервер перезапускается или недоступен, повторите через минуту';
  }
  if (status === 413) return 'файл или запрос слишком большой';
  if (status === 429) return 'слишком много запросов, повторите позже';
  if (Number.isFinite(status) && status >= 500) return `сбой на сервере (ошибка ${status})`;
  if (Number.isFinite(status) && status > 0) return `ошибка ${status}`;
  return fallback;
}

/**
 * Годится ли сырое тело ответа для показа человеку.
 *
 * Разметку не показываем: при 502 прокси отвечает страницей `<html>…nginx/1.25.5…`,
 * и она попадала в форму входа и в тосты целиком - человек не понимал, что делать, а
 * наружу заодно уходила версия прокси (#2525). Простой короткий текст оставляем: его
 * присылают ранние гейты, и он ближе к причине, чем общая фраза.
 */
function looksHuman(raw) {
  if (raw.includes('<') || raw.includes('{') || raw.includes('\n')) return false;
  return raw.length <= 200;
}

/**
 * Текст отказа из ответа API - для показа человеку.
 *
 * Бэк объясняет отказ словами в конверте `{success:false, error:"..."}`, но тело
 * ответа читается строкой, и в тост попадал весь конверт: пользователь видел
 * `{"success":false,"error":"..."}` вместо самой причины и понимал это как
 * «не удалось отправить» (#2320).
 *
 * @param {string} body сырое тело ответа
 * @param {string} [fallback] что показать, если объяснения в ответе нет
 * @param {number} [status] код состояния - по нему собирается запасное сообщение
 * @returns {string}
 */
export function readApiError(body, fallback = 'неизвестная ошибка', status = undefined) {
  const spare = status === undefined ? fallback : describeHttpStatus(status, fallback);
  const raw = String(body ?? '').trim();
  if (!raw) return spare;

  try {
    const parsed = JSON.parse(raw);
    // error - конверт проекта, message - дефолтный формат echo на ранних гейтах.
    const text = parsed?.error || parsed?.message;
    if (typeof text === 'string' && text.trim()) return text.trim();
    if (text && typeof text === 'object') return JSON.stringify(text);
    // Конверт без объяснения: сам JSON человеку ничего не говорит.
    return spare;
  } catch {
    // Не JSON: либо короткое человеческое объяснение, либо страница прокси.
  }
  return looksHuman(raw) ? raw : spare;
}

/**
 * Показывает отказ API тостом - с объяснением из ответа, а не с сырым конвертом.
 *
 * @param {string} prefix что случилось («Ошибка отправки заявки: »)
 * @param {string} body сырое тело ответа
 * @param {string} [fallback]
 * @param {number} [status] код состояния - для запасного сообщения
 */
export function notifyApiError(prefix, body, fallback, status = undefined) {
  useDeletionsStore().notify({ prefix, bold: readApiError(body, fallback, status), type: 'error' });
}

/**
 * Первая буква заглавная - объяснения из ответа приходят строчными («сервер
 * перезапускается…»), а в форме входа сообщение стоит отдельной строкой.
 *
 * @param {string} text
 * @returns {string}
 */
export function capitalize(text) {
  const raw = String(text ?? '').trim();
  return raw ? raw[0].toUpperCase() + raw.slice(1) : raw;
}
