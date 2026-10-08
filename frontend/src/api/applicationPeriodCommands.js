import { apiRequest } from './client';

async function command(applicationId, request, preview) {
  if (!Number.isSafeInteger(applicationId) || applicationId <= 0) throw new Error('Некорректная заявка');
  const response = await apiRequest(`/applications/${applicationId}/attachment-period${preview ? '/preview' : ''}`, {
    method: preview ? 'POST' : 'PUT', body: JSON.stringify(request),
  });
  const result = await response.json(); // client.js unwraps the API envelope.
  if (!response.ok) {
    const error = new Error(response.status >= 500 ? 'Не удалось изменить срок вложений' : (result?.message || 'Не удалось изменить срок вложений'));
    error.status = response.status;
    throw error;
  }
  const expected = [...request.attachment_ids].sort((a, b) => a - b);
  const actual = Array.isArray(result?.attachment_ids) ? [...result.attachment_ids].sort((a, b) => a - b) : [];
  if (result?.application_id !== applicationId || !/^[a-f0-9]{64}$/i.test(result?.period_revision || '') ||
      JSON.stringify(actual) !== JSON.stringify(expected) || result.individual_policy !== request.individual_policy ||
      !Array.isArray(result.attachments) || result.attachments.length !== expected.length ||
      JSON.stringify(result.attachments.map(item => item?.attachment_id).sort((a, b) => a - b)) !== JSON.stringify(expected) ||
      result.attachments.some(item => !expected.includes(item.attachment_id) || !item.old_period) ||
      !['attachment_count', 'employee_count', 'car_count', 'individual_count'].every(key => Number.isSafeInteger(result[key]) && result[key] >= 0) ||
      result.attachment_count !== expected.length || !result.new_period || result.approvals_reset !== false) {
    throw new Error('Не удалось прочитать подтверждение срока вложений');
  }
  return result;
}

export const previewApplicationPeriods = (id, request) => command(id, request, true);
export const changeApplicationPeriods = (id, request) => command(id, request, false);
