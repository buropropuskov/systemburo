import { apiRequest } from './client';

export async function getAttachmentTemplates() {
  const res = await apiRequest('/attachments');
  return res.json();
}
