import { describe, it, expect } from 'vitest';
import { readApiError } from '../apiError';

/**
 * Отказ подачи объясняется словами, а не конвертом (#2320).
 *
 * Бэк отвечает `{success:false, error:"..."}`, тело читается строкой, и раньше весь
 * конверт уходил в тост целиком: пользователь видел JSON и понимал его как «не
 * удалось отправить», хотя причина в ответе была - занятое место «По факту», срок,
 * дубль, чёрный список.
 */
describe('readApiError', () => {
  it('достаёт объяснение из конверта проекта', () => {
    const body = JSON.stringify({
      success: false,
      error: 'У организации уже есть действующая заявка на машину «По факту» № 20260905/001.',
    });
    expect(readApiError(body)).toBe('У организации уже есть действующая заявка на машину «По факту» № 20260905/001.');
  });

  it('понимает и формат echo с message', () => {
    // Ранние гейты (разбор тела, лимит размера) отвечают до конверта проекта.
    expect(readApiError('{"message":"Request body too large"}')).toBe('Request body too large');
  });

  it('не-JSON показывает как есть - это ближе к причине, чем общая фраза', () => {
    expect(readApiError('<html>502 Bad Gateway</html>')).toBe('<html>502 Bad Gateway</html>');
  });

  it('пустой ответ подменяет запасным текстом', () => {
    expect(readApiError('')).toBe('неизвестная ошибка');
    expect(readApiError(null, 'сервер не ответил')).toBe('сервер не ответил');
    expect(readApiError('   ')).toBe('неизвестная ошибка');
  });

  it('конверт без текста ошибки не превращается в пустой тост', () => {
    expect(readApiError('{"success":false}')).toBe('{"success":false}');
  });
});
