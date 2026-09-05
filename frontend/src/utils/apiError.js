import { useDeletionsStore } from '@/stores/deletions';

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
 * @returns {string}
 */
export function readApiError(body, fallback = 'неизвестная ошибка') {
  const raw = String(body ?? '').trim();
  if (!raw) return fallback;

  try {
    const parsed = JSON.parse(raw);
    // error - конверт проекта, message - дефолтный формат echo на ранних гейтах.
    const text = parsed?.error || parsed?.message;
    if (typeof text === 'string' && text.trim()) return text.trim();
    if (text && typeof text === 'object') return JSON.stringify(text);
  } catch {
    // Не JSON (прокси отдал HTML, обрыв соединения) - показываем как есть: это
    // всё же ближе к причине, чем общая фраза.
  }
  return raw;
}

/**
 * Показывает отказ API тостом - с объяснением из ответа, а не с сырым конвертом.
 *
 * @param {string} prefix что случилось («Ошибка отправки заявки: »)
 * @param {string} body сырое тело ответа
 * @param {string} [fallback]
 */
export function notifyApiError(prefix, body, fallback) {
  useDeletionsStore().notify({ prefix, bold: readApiError(body, fallback), type: 'error' });
}
