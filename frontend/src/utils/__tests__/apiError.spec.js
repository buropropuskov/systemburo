import { describe, it, expect } from 'vitest';
import { capitalize, describeHttpStatus, readApiError } from '../apiError';

/**
 * Отказ объясняется словами, а не конвертом (#2320) и не страницей прокси (#2525).
 *
 * Бэк отвечает `{success:false, error:"..."}`, тело читается строкой, и раньше весь
 * конверт уходил в тост целиком: пользователь видел JSON и понимал его как «не
 * удалось отправить», хотя причина в ответе была - занятое место «По факту», срок,
 * дубль, чёрный список.
 *
 * Второй случай нашёл владелец: при перезапуске сервера прокси отвечает страницей
 * `<html>…502 Bad Gateway…nginx/1.25.5…`, и она печаталась в форме входа целиком -
 * человек не понимал, что делать, а наружу уходила версия прокси.
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

  it('страницу прокси человеку не показывает', () => {
    const page = '<html> <head><title>502 Bad Gateway</title></head> <body> <center><h1>502 Bad Gateway</h1></center> <hr><center>nginx/1.25.5</center> </body> </html>';
    expect(readApiError(page, undefined, 502)).toBe('сервер перезапускается или недоступен, повторите через минуту');
    // Версия прокси наружу не уходит ни при каком коде состояния.
    expect(readApiError(page)).not.toContain('nginx');
    expect(readApiError(page)).not.toContain('<');
  });

  it('короткое человеческое объяснение оставляет как есть', () => {
    // Такое присылают ранние гейты простым текстом - оно ближе к причине, чем общая фраза.
    expect(readApiError('Срок действия пропуска истёк')).toBe('Срок действия пропуска истёк');
  });

  it('длинное тело без разметки тоже не показывает', () => {
    // Стек, дамп, портянка логов - человеку это ничего не объясняет.
    expect(readApiError('ошибка '.repeat(40), undefined, 500)).toBe('сбой на сервере (ошибка 500)');
  });

  it('пустой ответ подменяет сообщением по коду, а без кода - запасным текстом', () => {
    expect(readApiError('')).toBe('неизвестная ошибка');
    expect(readApiError(null, 'сервер не ответил')).toBe('сервер не ответил');
    expect(readApiError('   ', undefined, 503)).toBe('сервер перезапускается или недоступен, повторите через минуту');
  });

  it('конверт без текста ошибки не показывает сам JSON', () => {
    expect(readApiError('{"success":false}', undefined, 500)).toBe('сбой на сервере (ошибка 500)');
    expect(readApiError('{"success":false}')).toBe('неизвестная ошибка');
  });
});

describe('describeHttpStatus', () => {
  it('коды шлюза объясняет как временную неполадку', () => {
    for (const status of [502, 503, 504]) {
      expect(describeHttpStatus(status)).toBe('сервер перезапускается или недоступен, повторите через минуту');
    }
  });

  it('называет понятные причины для частых кодов', () => {
    expect(describeHttpStatus(413)).toBe('файл или запрос слишком большой');
    expect(describeHttpStatus(429)).toBe('слишком много запросов, повторите позже');
    expect(describeHttpStatus(500)).toBe('сбой на сервере (ошибка 500)');
    expect(describeHttpStatus(418)).toBe('ошибка 418');
  });

  it('без кода отдаёт запасной текст', () => {
    expect(describeHttpStatus(undefined)).toBe('неизвестная ошибка');
    expect(describeHttpStatus(undefined, 'сервер не ответил')).toBe('сервер не ответил');
  });
});

describe('capitalize', () => {
  it('поднимает первую букву и не трогает остальное', () => {
    expect(capitalize('сервер перезапускается')).toBe('Сервер перезапускается');
    expect(capitalize('Неверный логин или пароль')).toBe('Неверный логин или пароль');
    expect(capitalize('')).toBe('');
  });
});
