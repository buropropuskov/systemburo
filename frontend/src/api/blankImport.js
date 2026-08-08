import { apiRequestRaw } from './client';
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
