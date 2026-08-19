import { useAuthStore } from '@/stores/auth';

/**
 * Скачивание журнала обращений файлом .xlsx (#2125).
 *
 * Идёт мимо apiRequest: тот разворачивает JSON-конверт, а здесь ответ - поток
 * байтов книги. Имя файла берётся из Content-Disposition, который ставит сервер.
 *
 * Числа охвата приходят заголовками X-Export-*: сервер отдаёт не больше
 * десяти тысяч строк, и молчать об отсечённом остатке нельзя - по такому файлу
 * человек считает итоги за период.
 *
 * @param {Record<string, string|number>} params query-параметры отбора и порядка
 * @returns {Promise<{rows: number, total: number, truncated: boolean}>} охват выгрузки
 */
export async function downloadRequestLogs(params = {}) {
  const authStore = useAuthStore();
  const query = new URLSearchParams(params).toString();
  const res = await fetch(
    `${(import.meta.env.VITE_API_BASE_URL || '') + '/api'}/request-logs/export${query ? '?' + query : ''}`,
    {
      credentials: 'include',
      headers: { ...(authStore.token ? { Authorization: `Bearer ${authStore.token}` } : {}) },
    },
  );
  if (!res.ok) {
    // Код ответа нужен экрану: 403 и 500 читаются человеком по-разному, и
    // «пустая таблица без объяснений» - это то, что чинит срез.
    const err = new Error(`Не удалось выгрузить журнал: ${res.status}`);
    err.status = res.status;
    throw err;
  }

  const disposition = res.headers.get('Content-Disposition') || '';
  const fromHeader = /filename="?([^";]+)"?/.exec(disposition);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fromHeader ? fromHeader[1] : 'request-logs.xlsx';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);

  return {
    rows: Number(res.headers.get('X-Export-Rows') || 0),
    total: Number(res.headers.get('X-Export-Total') || 0),
    truncated: res.headers.get('X-Export-Truncated') === 'true',
  };
}
