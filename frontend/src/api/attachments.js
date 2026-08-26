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
 * - title бэкенд приводит к верхнему регистру;
 * - auto_export (#1615) - исключение из "полной замены": бэк трактует его как
 *   указатель, отсутствующий ключ не трогает тумблер архива. createAttachment/
 *   updateAttachment передают его, только когда вызывающая форма явно знает про
 *   архив (иначе поле остаётся undefined и выпадает из JSON.stringify).
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

/**
 * autoExport - тумблер файлового архива (#1615): писать ли бланки этого типа в
 * файловый архив. Указатель по смыслу - оставляем его undefined, если вызывающий
 * не передал значение явно: JSON.stringify выбрасывает undefined-поля из тела, и
 * бэк (тоже *bool) трактует отсутствующий ключ как "не трогать" при обновлении.
 * Явно передаём его ТОЛЬКО когда вызывающая форма реально показывает тумблер -
 * иначе сохранение других полей молча гасило/поднимало бы архивную настройку.
 */
export async function createAttachment({ attachmentType, name, displayName, title, instruction = null, autoExport }) {
  const res = await apiRequest('/attachments', {
    method: 'POST',
    body: JSON.stringify({
      attachment_type: attachmentType,
      name,
      display_name: displayName,
      title,
      instruction,
      auto_export: autoExport,
    }),
  });
  return unwrap(res, 'Не удалось создать вложение');
}

export async function updateAttachment(id, {
  attachmentType, name, displayName, title, instruction = null, autoExport,
}) {
  const res = await apiRequest(`/attachments/${id}`, {
    method: 'PUT',
    body: JSON.stringify({
      attachment_type: attachmentType,
      name,
      display_name: displayName,
      title,
      instruction,
      auto_export: autoExport,
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
 * Привязка ручного вложения-сироты к заявке (#1049 режим-2, backend #1060).
 * Только super/admin (BE-гейт page.admin). Ровно одно из applicationId /
 * targetAttachmentId (XOR): applicationId усыновляет сироту в заявку (создаёт
 * в ней новое вложение), targetAttachmentId перевешивает сущности сироты на
 * существующее вложение заявки (сирота удаляется).
 * POST /attachments/{id}/attach-to-application -> {application_id, attachment_id}.
 * @param {number} attachmentId экземпляр вложения-сироты (resp.attachment_id из manual-create)
 * @param {{applicationId?: number, targetAttachmentId?: number}} target
 */
export async function attachToApplication(attachmentId, { applicationId = null, targetAttachmentId = null } = {}) {
  const body = {};
  if (applicationId != null) body.application_id = applicationId;
  if (targetAttachmentId != null) body.target_attachment_id = targetAttachmentId;
  const res = await apiRequest(`/attachments/${attachmentId}/attach-to-application`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  return unwrap(res, 'Не удалось привязать вложение к заявке');
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
