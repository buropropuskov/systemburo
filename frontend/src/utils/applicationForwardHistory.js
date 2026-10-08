// Форматирование только безопасного DTO пересылки. Старые recipients содержали
// снимки имён без ID: их нельзя использовать для интерфейса или выгрузки.
export const FORWARD_RECIPIENTS_UNAVAILABLE = 'Сведения о получателях недоступны';

export function forwardRecipients(data) {
  if (data?.recipient_details_available !== true || !Array.isArray(data.recipient_details)) return [];
  const ids = new Set();
  const valid = data.recipient_details.every(recipient => {
    if (!recipient || !Number.isInteger(recipient.user_id) || recipient.user_id <= 0 || ids.has(recipient.user_id)) return false;
    ids.add(recipient.user_id);
    return ['approval', 'view'].includes(recipient.purpose)
      && typeof recipient.required_approval === 'boolean'
      && typeof recipient.access_granted === 'boolean'
      && typeof recipient.display_name === 'string'
      && recipient.display_name.trim().length > 0
      && (recipient.purpose !== 'view' || !recipient.required_approval)
      && (recipient.purpose !== 'approval' || recipient.access_granted);
  });
  return valid ? data.recipient_details : [];
}

export function forwardRecipientPurpose(recipient) {
  if (recipient.purpose === 'approval') {
    return recipient.required_approval ? 'назначен обязательным согласующим' : 'назначен согласующим';
  }
  return recipient.access_granted ? 'предоставлен доступ к просмотру' : 'повторно направлено для просмотра';
}

export function forwardActionText() {
  return 'Переслал(-а) заявку';
}

export function forwardRecipientsText(data) {
  const recipients = forwardRecipients(data);
  return recipients.length
    ? recipients.map(recipient => `${recipient.display_name} — ${forwardRecipientPurpose(recipient)}`).join('\n')
    : FORWARD_RECIPIENTS_UNAVAILABLE;
}

export function forwardMaterialsText(data) {
  if (data?.attachment_scope === 'all' || (data?.attachment_scope !== 'selected' && data?.whole === true)) return 'заявка со всеми приложениями';
  const selected = data?.attachment_scope === 'selected' || data?.whole === false;
  if (!selected) return 'Сведения о материалах недоступны';
  // Новые события используют снимки ID/имени; DTO ветки и legacy — снимки attachments.
  const names = Array.isArray(data.attachment_details)
    ? data.attachment_details.map(attachment => attachment?.name).filter(name => typeof name === 'string' && name.length)
    : (Array.isArray(data.attachments) ? data.attachments.filter(name => typeof name === 'string' && name.length) : []);
  return names.length ? `только выбранные приложения — ${names.join(', ')}` : 'только выбранные приложения — сведения недоступны';
}
