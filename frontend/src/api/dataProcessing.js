import { apiRequest, apiRequestRaw } from './client';

/**
 * API-клиент документа согласия на обработку персональных данных.
 * Метаданные хранятся в системных настройках, файл -- на диске бэкенда.
 * @typedef {{stored_name: string, file_name: string, mime_type: string, ext: string, uploaded_at: string}} DataProcessingDocMeta
 */

const BASE = '/settings/data-processing/document';

/**
 * Метаданные текущего документа согласия или null, если он не загружен.
 * @returns {Promise<DataProcessingDocMeta|null>}
 */
export async function getDataProcessingMeta() {
  const res = await apiRequest(`${BASE}/meta`);
  if (!res.ok) throw new Error(`Не удалось загрузить данные документа: ${res.status}`);
  return res.json();
}

/**
 * Загружает (заменяет) документ согласия. Только для администратора.
 * @param {File} file
 * @returns {Promise<DataProcessingDocMeta>}
 */
export async function uploadDataProcessingDoc(file) {
  const form = new FormData();
  form.append('file', file);
  const res = await apiRequest(BASE, { method: 'POST', body: form });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body?.message || `Ошибка загрузки документа: ${res.status}`);
  }
  return res.json();
}

/**
 * Удаляет документ согласия. Только для администратора.
 * @returns {Promise<void>}
 */
export async function deleteDataProcessingDoc() {
  const res = await apiRequest(BASE, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Ошибка удаления документа: ${res.status}`);
}

/**
 * Загружает файл документа как Blob (для inline-просмотра или скачивания).
 * Запрос идёт с Authorization-заголовком, поэтому файл нельзя встроить
 * напрямую через <embed src>, а только через object URL над полученным Blob.
 * @param {{download?: boolean}} [opts]
 * @returns {Promise<Blob>}
 */
export async function fetchDataProcessingBlob({ download = false } = {}) {
  const res = await apiRequestRaw(download ? `${BASE}?download=1` : BASE);
  if (!res.ok) throw new Error(`Не удалось загрузить документ: ${res.status}`);
  return res.blob();
}

/**
 * Скачивает документ согласия в браузере пользователя.
 * @param {string} fileName
 * @returns {Promise<void>}
 */
export async function downloadDataProcessingDoc(fileName) {
  const blob = await fetchDataProcessingBlob({ download: true });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName || 'soglasie-na-obrabotku-dannyh';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
