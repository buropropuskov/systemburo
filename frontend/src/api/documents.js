import { apiRequest } from './client';
import { useAuthStore } from '@/stores/auth';

/**
 * API-клиент управления документами (#39).
 * Все методы возвращают unwrapped data (см. apiRequest/wrapJsonUnwrap в client.js).
 */

// ========== Группы документов ==========

export async function listDocumentGroups() {
  const res = await apiRequest('/document-groups');
  return res.json();
}

export async function createDocumentGroup({ name }) {
  const res = await apiRequest('/document-groups', {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function renameDocumentGroup(id, { name }) {
  const res = await apiRequest(`/document-groups/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function deleteDocumentGroup(id) {
  const res = await apiRequest(`/document-groups/${id}`, { method: 'DELETE' });
  return res.json();
}

export async function reorderDocumentGroups(ids) {
  const res = await apiRequest('/document-groups/reorder', {
    method: 'PUT',
    body: JSON.stringify({ ids }),
  });
  return res.json();
}

// ========== Документы (админка) ==========

/**
 * Список документов для админки.
 * @param {object} opts
 * @param {number|null} opts.groupId - фильтр по группе (null = все)
 * @param {boolean} opts.includeHidden - включить скрытые (=true для админки)
 */
export async function listDocuments({ groupId = null, includeHidden = false } = {}) {
  const params = new URLSearchParams();
  if (groupId != null) params.set('group_id', groupId);
  if (includeHidden) params.set('include_hidden', '1');
  const qs = params.toString() ? `?${params}` : '';
  const res = await apiRequest(`/documents${qs}`);
  return res.json();
}

/**
 * Загрузить один файл с метаданными. Модалка шлёт очередь последовательно.
 * @param {File} file - объект File
 * @param {object} meta - { title, description, group_id, published_at, sort_order }
 */
export async function uploadDocument(file, meta = {}) {
  const form = new FormData();
  form.append('file', file);
  if (meta.title != null) form.append('title', meta.title);
  if (meta.description != null) form.append('description', meta.description);
  if (meta.group_id != null) form.append('group_id', String(meta.group_id));
  if (meta.published_at != null) form.append('published_at', meta.published_at);
  if (meta.sort_order != null) form.append('sort_order', String(meta.sort_order));
  const res = await apiRequest('/documents', { method: 'POST', body: form });
  return res.json();
}

/**
 * Обновить метаданные документа (без замены файла).
 * @param {number} id
 * @param {object} meta - { title, description, group_id, published_at, is_visible }
 */
export async function updateDocument(id, meta) {
  const res = await apiRequest(`/documents/${id}`, {
    method: 'PUT',
    body: JSON.stringify(meta),
  });
  return res.json();
}

/**
 * Заменить файл документа (multipart).
 * @param {number} id
 * @param {File} file
 */
export async function replaceDocumentFile(id, file) {
  const form = new FormData();
  form.append('file', file);
  const res = await apiRequest(`/documents/${id}/file`, { method: 'PUT', body: form });
  return res.json();
}

export async function deleteDocument(id) {
  const res = await apiRequest(`/documents/${id}`, { method: 'DELETE' });
  return res.json();
}

/**
 * Изменить порядок документов внутри группы.
 * @param {number|null} groupId - null для «Прочее»
 * @param {number[]} ids - упорядоченный массив id
 */
export async function reorderDocuments(groupId, ids) {
  const res = await apiRequest('/documents/reorder', {
    method: 'PUT',
    body: JSON.stringify({ group_id: groupId, ids }),
  });
  return res.json();
}

// ========== Публичный список (блок /news) ==========

/**
 * Видимые документы, сгруппированные и упорядоченные.
 * Для блока DocumentsBlock.vue.
 */
export async function listPublicDocuments() {
  const res = await apiRequest('/public/documents');
  return res.json();
}

// ========== Скачивание ==========

/**
 * Скачать документ через Blob. Использует Bearer-токен из Pinia (не через cookie),
 * так как скачивание требует явного auth-заголовка (не передаётся браузером для blob-URL).
 * Триггерит нативный download через временный <a>.
 * @param {number} id - id документа
 * @param {string} fileName - имя файла для Content-Disposition (подставляем из document.file_name)
 */
export async function downloadDocument(id, fileName) {
  // TODO: бэкенд отдаёт Content-Disposition: attachment — fileName здесь резервный
  const authStore = useAuthStore();
  const res = await fetch(
    `${(import.meta.env.VITE_API_BASE_URL || '') + '/api'}/documents/${id}/download`,
    {
      credentials: 'include',
      headers: {
        ...(authStore.token ? { Authorization: `Bearer ${authStore.token}` } : {}),
      },
    }
  );
  if (!res.ok) throw new Error(`download failed: ${res.status}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName || 'document';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
