import { apiRequest } from './client';
import { useAuthStore } from '@/stores/auth';

/**
 * API-клиент модалки «Руководство» (B2).
 * Бэкенд (GET /guide/sections) уже отдаёт только разделы, на которые у
 * пользователя есть право guide.<role>, — фронт рисует ровно пришедшее.
 */

/**
 * Разделы руководства, доступные текущему пользователю.
 * @returns {Promise<Array<{role: string, title: string, lead: string, items: string[], file: object|null}>>}
 */
export async function listGuideSections() {
  const res = await apiRequest('/guide/sections');
  return res.json();
}

/**
 * Скачать PDF раздела через Blob. Эндпоинт под JWT, поэтому Bearer-токен из Pinia
 * передаём явным заголовком (для blob-URL браузер его сам не добавит).
 * downloadUrl приходит из ответа уже с префиксом /api (GuideFileInfo.download_url),
 * поэтому базу берём из VITE_API_BASE_URL без повторного /api.
 * @param {string} downloadUrl - section.file.download_url
 * @param {string} fileName - section.file.name для имени сохраняемого файла
 */
export async function downloadGuideFile(downloadUrl, fileName) {
  const base = import.meta.env.VITE_API_BASE_URL || '';
  const authStore = useAuthStore();
  const res = await fetch(`${base}${downloadUrl}`, {
    credentials: 'include',
    headers: {
      ...(authStore.token ? { Authorization: `Bearer ${authStore.token}` } : {}),
    },
  });
  if (!res.ok) throw new Error(`download failed: ${res.status}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName || 'guide.pdf';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

/* ===================== Админ-управление (B1c, гейт page.admin) ===================== */

/**
 * apiRequest разворачивает envelope: на успехе res.json() = data, на ошибке = { message }.
 * res.ok проверяем явно и бросаем Error с сообщением бэка, чтобы экран управления
 * показал реальную ошибку, а не принял конверт ошибки за обновлённый раздел.
 */
async function unwrapGuide(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Все разделы руководства без фильтра по правам (для админ-экрана управления).
 * @returns {Promise<Array<{role: string, title: string, lead: string, items: string[], file: object|null}>>}
 */
export async function listAllGuideSections() {
  const res = await apiRequest('/guide/admin/sections');
  return unwrapGuide(res, 'Не удалось загрузить разделы руководства');
}

/**
 * Сохранить текст раздела руководства (лид + пункты).
 * @param {string} role - user|guard|admin
 * @param {{lead: string, items: string[]}} content
 * @returns {Promise<object>} обновлённый раздел
 */
export async function updateGuideSection(role, { lead, items }) {
  const res = await apiRequest(`/guide/admin/sections/${role}`, {
    method: 'PUT',
    body: JSON.stringify({ lead, items }),
  });
  return unwrapGuide(res, 'Не удалось сохранить раздел');
}

/**
 * Загрузить/заменить PDF раздела (multipart). Content-Type не ставим — apiRequest
 * для FormData отдаёт его браузеру (нужна boundary).
 * @param {string} role - user|guard|admin
 * @param {File} file - PDF-файл
 * @returns {Promise<object>} обновлённый раздел (с file)
 */
export async function uploadGuideFile(role, file) {
  const form = new FormData();
  form.append('file', file);
  const res = await apiRequest(`/guide/admin/sections/${role}/file`, {
    method: 'PUT',
    body: form,
  });
  return unwrapGuide(res, 'Не удалось загрузить файл');
}

/**
 * Удалить PDF раздела.
 * @param {string} role - user|guard|admin
 * @returns {Promise<object>} обновлённый раздел (file: null)
 */
export async function deleteGuideFile(role) {
  const res = await apiRequest(`/guide/admin/sections/${role}/file`, {
    method: 'DELETE',
  });
  return unwrapGuide(res, 'Не удалось удалить файл');
}
