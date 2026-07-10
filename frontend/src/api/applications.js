import { apiRequest, apiRequestRaw } from './client';

export async function getApplications(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequest(`/applications${query ? '?' + query : ''}`);
  return res.json();
}

export async function getUserApplications(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequest(`/applications/user${query ? '?' + query : ''}`);
  return res.json();
}

/**
 * Активные согласованные заявки, доступные для привязки ручного вложения (#1049 режим-2).
 * Только super/admin (BE-гейт page.admin). В отличие от getApplications НЕ скоупит по
 * автор/ответственный/наблюдатель - админ видит все заявки для привязки.
 * @param {{search_query?: string}} params
 * @returns {Promise<object[]>}
 */
export async function getAttachableApplications(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequest(`/applications/attachable${query ? '?' + query : ''}`);
  return res.json();
}

export async function getApplicationById(id) {
  const res = await apiRequest(`/applications/${id}`);
  return res.json();
}

export async function createApplication(data) {
  const res = await apiRequest('/applications', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function submitCompleteApplication(data) {
  const res = await apiRequest('/applications/submit-complete-application', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateApplication(id, data) {
  const res = await apiRequest(`/applications/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function forwardApplication(id, data) {
  const res = await apiRequest(`/applications/${id}/forward`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function approveApplication(id, data) {
  const res = await apiRequest(`/applications/${id}/approve`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function takeToWork(id) {
  const res = await apiRequest(`/applications/${id}/take-to-work`, {
    method: 'POST',
  });
  return res.json();
}

export async function revokeFromWork(id) {
  const res = await apiRequest(`/applications/${id}/revoke-from-work`, {
    method: 'POST',
  });
  return res.json();
}

export async function markAsRead(id) {
  return apiRequest(`/applications/${id}/read`, { method: 'POST' });
}

export async function getUnreadCount() {
  const res = await apiRequest('/applications/unread-count');
  return res.json();
}

export async function getApplicationHistory(id) {
  const res = await apiRequest(`/applications/${id}/history`);
  return res.json();
}

export async function getForwardMessages(id) {
  const res = await apiRequest(`/applications/${id}/forward-messages`);
  return res.json();
}

export async function getApplicationDetails(id) {
  const res = await apiRequest(`/applications/${id}/details`);
  return res.json();
}

export async function getApplicationAttachments(id) {
  const res = await apiRequest(`/applications/${id}/attachments`);
  return res.json();
}

/**
 * Список вложений, доступных охраннику/админу во вкладке "Доступные мне" (#706).
 * Пагинация лежит в envelope.meta рядом с data, а apiRequest снимает только data
 * и meta теряется - поэтому читаем сырой ответ через apiRequestRaw.
 * @param {{page?: number, per_page?: number}} params
 * @returns {Promise<{items: object[], meta: {total: number, page: number, per_page: number}}>}
 */
export async function getAccessibleAttachments(params = {}) {
  const query = new URLSearchParams(params).toString();
  const res = await apiRequestRaw(`/applications/available-attachments${query ? '?' + query : ''}`);
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось загрузить доступные вложения');
  }
  return {
    items: body.data || [],
    meta: body.meta || { total: 0, page: 1, per_page: 20 },
  };
}

/**
 * Деталь доступного вложения (#706): заголовок с инфо заявки + типизированное
 * содержимое (cars/employees/items). Читаем через apiRequestRaw, чтобы 403 на
 * чужое вложение пробросился ошибкой, а не отдал полу-распакованный объект.
 * @param {number} id ID вложения
 * @returns {Promise<{attachment: object, cars?: object[], employees?: object[], items?: object[]}>}
 */
export async function getAccessibleAttachmentDetail(id) {
  const res = await apiRequestRaw(`/applications/available-attachments/${id}`);
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось загрузить вложение');
  }
  return body.data;
}

/**
 * Вопросы к заявке (Q&A #973). Envelope снят в apiRequest -> массив вопросов с вложенными ответами.
 * @param {number} id ID заявки
 * @returns {Promise<Array>}
 */
export async function getQuestions(id) {
  const res = await apiRequest(`/applications/${id}/questions`);
  return res.json();
}

/**
 * Создать вопрос к заявке (#973).
 * @param {number} id ID заявки
 * @param {{subject: string, text: string, attachment_ids?: number[]}} data
 * @returns {Promise<object>} созданный вопрос
 */
export async function createQuestion(id, data) {
  const res = await apiRequestRaw(`/applications/${id}/questions`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось отправить вопрос');
  }
  return body.data;
}

/**
 * Добавить ответ в тред вопроса (#973).
 * @param {number} applicationId ID заявки
 * @param {number} questionId ID вопроса
 * @param {{text: string}} data
 * @returns {Promise<object>} созданный ответ
 */
export async function createAnswer(applicationId, questionId, data) {
  const res = await apiRequestRaw(`/applications/${applicationId}/questions/${questionId}/answers`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось отправить ответ');
  }
  return body.data;
}

/**
 * Отметить вопросы заявки просмотренными (#973). Fire-and-forget, как markAsRead.
 * @param {number} id ID заявки
 */
export async function markQuestionsSeen(id) {
  return apiRequest(`/applications/${id}/questions/seen`, { method: 'POST' });
}

/**
 * Пометить конкретный вопрос-топик прочитанным (#973). Гасит его новизну для пользователя
 * (недочитанные топики остаются новыми). Fire-and-forget.
 * @param {number} applicationId ID заявки
 * @param {number} questionId ID вопроса
 */
export async function markQuestionRead(applicationId, questionId) {
  return apiRequest(`/applications/${applicationId}/questions/${questionId}/read`, { method: 'POST' });
}
