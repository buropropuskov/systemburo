import { apiRequest, apiRequestRaw } from './client';
import { parseContentDispositionFilename } from '@/utils/download';

/**
 * API клиент массового ввода участников через Excel-бланк (эпик blank-import).
 */

/**
 * Скачать пустой бланк типа вложения для заполнения списком участников (B1).
 * Эндпоинт отдаёт файл, только когда у активного шаблона размечена списочная
 * часть - иначе 404 с готовым русским текстом ("Шаблон бланка не настроен" /
 * "В бланке не размечен список участников" / "Тип вложения не найден"), который
 * бросаем как есть - формулировать причину заново на фронте не нужно.
 * @param {number} uniqueAttachmentId
 * @returns {Promise<{ blob: Blob, filename: string }>}
 */
export async function downloadBlankTemplate(uniqueAttachmentId) {
  const res = await apiRequestRaw(`/attachments/${uniqueAttachmentId}/blank-template`);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body?.error || 'Не удалось скачать бланк для заполнения');
  }
  const blob = await res.blob();
  const cd = res.headers.get('Content-Disposition') || '';
  const filename = parseContentDispositionFilename(cd, `blank_template_${uniqueAttachmentId}.xlsx`);
  return { blob, filename };
}

/**
 * Загрузить заполненный бланк на разбор и валидацию (C1C2/C3). Бэк отвечает 200 при
 * чистом файле и 207 (MultiStatus) при частичном успехе - оба входят в диапазон
 * `res.ok`, поэтому unwrap отдаёт {rows, summary} в обоих случаях без разбора статуса;
 * различать чистый/частичный результат вызывающая сторона может по summary.rejected.
 * Гейт файла (чужой бланк, сдвинутая структура, пустой/слишком длинный список) отдаёт
 * 400 с готовым русским текстом - бросаем как есть.
 * @param {number} uniqueAttachmentId
 * @param {File} file
 * @returns {Promise<{ rows: Array<Object>, summary: { read: number, accepted: number, rejected: number } }>}
 */
export async function uploadImportList(uniqueAttachmentId, file) {
  const form = new FormData();
  form.append('file', file);
  // FormData -> client.js не ставит Content-Type (см. isFormData в doFetch).
  const res = await apiRequest(`/attachments/${uniqueAttachmentId}/import-list`, {
    method: 'POST',
    body: form,
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body?.message || 'Не удалось загрузить список');
  return body;
}
