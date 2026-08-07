import { apiRequest } from '@/api/client';
import { useAuthStore } from '@/stores/auth';

/**
 * Файлы, приложенные к заявке (#1721).
 *
 * Прикрепить файл можно только при подаче, поэтому загрузка идёт до создания
 * заявки: сервер держит файл черновиком, а подача привязывает его к заявке по
 * `file_ids`. Черновик, о котором подача не сказала, сервер уберёт сам.
 */

/**
 * Загрузить файлы к ещё не поданной заявке.
 * @param {File[]} files
 * @returns {Promise<Array<{id: number, file_name: string, mime_type: string, file_size: number}>>}
 */
export async function uploadApplicationFiles(files) {
  const formData = new FormData();
  files.forEach((file) => formData.append('files', file));

  // headers пустые намеренно: FormData сам проставит boundary в Content-Type.
  const response = await apiRequest('/applications/files', {
    method: 'POST',
    body: formData,
    headers: {},
  });
  // client.js разворачивает конверт {success, data}: json() отдаёт уже сами
  // данные, а при отказе - {message}. Читать payload.data поверх этого нельзя.
  const payload = await response.json();
  if (!response.ok) {
    throw new Error(payload?.message || 'Не удалось загрузить файлы');
  }
  return payload ?? [];
}

/**
 * Убрать файл, ещё не привязанный к заявке.
 * @param {number} id
 */
export async function deleteApplicationDraftFile(id) {
  const response = await apiRequest(`/applications/files/${id}`, { method: 'DELETE' });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.message || 'Не удалось убрать файл');
  }
}

/**
 * Список файлов заявки.
 * @param {number} applicationId
 * @returns {Promise<Array<{id: number, file_name: string, mime_type: string, file_size: number}>>}
 */
export async function fetchApplicationFiles(applicationId) {
  const response = await apiRequest(`/applications/${applicationId}/files`);
  const payload = await response.json();
  if (!response.ok) {
    throw new Error(payload?.message || 'Не удалось получить файлы заявки');
  }
  return payload ?? [];
}

/**
 * Убрать файл, приложенный к заявке.
 * @param {number} applicationId
 * @param {number} fileId
 */
export async function deleteApplicationFile(applicationId, fileId) {
  const response = await apiRequest(`/applications/${applicationId}/files/${fileId}`, {
    method: 'DELETE',
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.message || 'Не удалось убрать файл');
  }
}

/**
 * Скачать файл заявки. Через Blob с явным Bearer: обычная ссылка не несёт
 * заголовок авторизации, а файлы заявки раздаются только под доступом к ней.
 * @param {number} applicationId
 * @param {number} fileId
 * @param {string} fileName
 */
export async function downloadApplicationFile(applicationId, fileId, fileName) {
  const authStore = useAuthStore();
  const base = (import.meta.env.VITE_API_BASE_URL || '') + '/api';
  const response = await fetch(`${base}/applications/${applicationId}/files/${fileId}`, {
    credentials: 'include',
    headers: {
      ...(authStore.token ? { Authorization: `Bearer ${authStore.token}` } : {}),
    },
  });
  if (!response.ok) {
    throw new Error('Не удалось скачать файл');
  }

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName || 'file';
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
