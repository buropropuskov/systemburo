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
