import { apiRequestRaw } from './client';
import { saveBlobAs } from './attachment-templates';
import { parseContentDispositionFilename } from '@/utils/download';

/**
 * Скачивание журнала обращений файлом .xlsx (#2125). Ответ - поток байтов книги,
 * поэтому идёт через apiRequestRaw, как выгрузка версий системных таблиц.
 *
 * Числа охвата приходят заголовками X-Export-*: сервер отдаёт не больше десяти
 * тысяч строк, и молчать об отсечённом остатке нельзя - по такому файлу человек
 * считает итоги за период.
 *
 * silent403: отказ показывает экран своим текстом рядом со списком, общий тост
 * «Недостаточно прав» здесь только дублировал бы его.
 *
 * @param {Record<string, string|number>} params query-параметры отбора и порядка
 * @returns {Promise<{rows: number, total: number, truncated: boolean}>} охват выгрузки
 */
export async function downloadRequestLogs(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequestRaw(`/request-logs/export${query ? '?' + query : ''}`, { silent403: true });
  if (!res.ok) {
    // Код ответа нужен экрану: 403 и 500 читаются человеком по-разному, и
    // «пустая таблица без объяснений» - это то, что чинит срез.
    const err = new Error(`Не удалось выгрузить журнал: ${res.status}`);
    err.status = res.status;
    throw err;
  }

  const blob = await res.blob();
  const cd = res.headers.get('Content-Disposition') || '';
  saveBlobAs(blob, parseContentDispositionFilename(cd, 'request-logs.xlsx'));

  return {
    rows: Number(res.headers.get('X-Export-Rows') || 0),
    total: Number(res.headers.get('X-Export-Total') || 0),
    truncated: res.headers.get('X-Export-Truncated') === 'true',
  };
}
