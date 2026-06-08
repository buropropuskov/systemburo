import { apiRequest } from './client';

// apiRequest разворачивает envelope, поэтому res.json() даёт уже сам массив истории.
// res.ok проверяем явно: на 4xx wrapJsonUnwrap вернул бы {message}, а не массив,
// и без throw модалка молча показала бы "История пуста" вместо ошибки.

export async function getLicenseFormatHistory(id) {
  const res = await apiRequest(`/license-plate-formats/${id}/history`);
  if (!res.ok) throw new Error('Не удалось загрузить историю формата');
  return res.json();
}
