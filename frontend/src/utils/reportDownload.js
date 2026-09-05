/** Имя файла выгрузки и само скачивание - общее для xlsx, pdf и png. */
import { formatMoscowDateTime } from '@/utils/serverTime';

function exportStamp() {
  return formatMoscowDateTime().replace(/[.: ]/g, '-');
}

export function downloadFileName(opts, ext) {
  // Имя уезжает в почту, архиваторы и на чужие файловые системы, поэтому оставляем
  // только буквы, цифры, дефис и подчёркивание: кавычки-ёлочки из названия разреза
  // переживают не всякую из них (#2324).
  const safe = (opts.title || 'аналитика')
    .replace(/\s+/g, '_')
    .replace(/[^\p{L}\p{N}_-]+/gu, '')
    .replace(/_{2,}/g, '_')
    .replace(/^_|_$/g, '');
  return `Отчёт_${safe || 'аналитика'}_${exportStamp()}.${ext}`;
}

export function downloadBlob(blob, filename) {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.download = filename;
  a.href = url;
  // Без attach в DOM Firefox/Safari не запускают скачивание по click().
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.URL.revokeObjectURL(url);
}
