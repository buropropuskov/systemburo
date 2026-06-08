import { apiRequest } from './client';

/**
 * API клиент шаблонов вложений (бланков) (#416).
 *
 * apiRequest разворачивает envelope: на успехе res.json() отдаёт data,
 * на ошибке - { message }. res.ok проверяем явно и бросаем Error с
 * сообщением бэка, чтобы компонент показал реальную ошибку (например, 400
 * при дубле системного имени), а не считал её успехом.
 *
 * Особенности контракта вложений (отличия от других справочников):
 * - архив через ДВА эндпоинта (/attachments активные, /attachments/all все),
 *   а не через ?include_archived=true;
 * - восстановление через PUT /attachments/{id}/restore (не POST);
 * - PUT - полная замена всех полей (attachment_type/name/display_name/title/instruction),
 *   поэтому updateAttachment всегда шлёт весь набор;
 * - title бэкенд приводит к верхнему регистру.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

export async function listAttachments() {
  const res = await apiRequest('/attachments');
  return unwrap(res, 'Не удалось загрузить вложения');
}

export async function listAllAttachments() {
  const res = await apiRequest('/attachments/all');
  return unwrap(res, 'Не удалось загрузить вложения');
}

export async function createAttachment({ attachmentType, name, displayName, title, instruction = null }) {
  const res = await apiRequest('/attachments', {
    method: 'POST',
    body: JSON.stringify({
      attachment_type: attachmentType,
      name,
      display_name: displayName,
      title,
      instruction,
    }),
  });
  return unwrap(res, 'Не удалось создать вложение');
}

export async function updateAttachment(id, { attachmentType, name, displayName, title, instruction = null }) {
  const res = await apiRequest(`/attachments/${id}`, {
    method: 'PUT',
    body: JSON.stringify({
      attachment_type: attachmentType,
      name,
      display_name: displayName,
      title,
      instruction,
    }),
  });
  return unwrap(res, 'Не удалось сохранить вложение');
}

export async function archiveAttachment(id) {
  const res = await apiRequest(`/attachments/${id}`, { method: 'DELETE' });
  return unwrap(res, 'Не удалось архивировать вложение');
}

export async function restoreAttachment(id) {
  const res = await apiRequest(`/attachments/${id}/restore`, { method: 'PUT' });
  return unwrap(res, 'Не удалось восстановить вложение');
}

/**
 * История действий над шаблоном вложения (#416, backend #485).
 * GET /attachments/{id}/history -> [{id, action_type, details, actor_user_id, actor_name, created_at}].
 * details: created={display_name}; updated={field:{old,new}} по
 * attachment_type/name/display_name/title/instruction; archived/restored без details.
 */
export async function getAttachmentHistory(id) {
  const res = await apiRequest(`/attachments/${id}/history`);
  return unwrap(res, 'Не удалось загрузить историю вложения');
}
