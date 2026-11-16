import { apiRequest } from './client'

export async function getAccountOwnerAccess() {
  try {
    const response = await apiRequest('/account-owner/access')
    return response.ok ? await response.json() : {}
  } catch {
    return {}
  }
}

export async function accountOwnerRequest(path, body, signal) {
  const response = await apiRequest(path, {
    method: body ? 'POST' : 'GET',
    ...(body ? { body: JSON.stringify(body) } : {}),
    signal,
  })
  if (!response.ok) {
    const messages = {
      400: 'Проверьте сведения и основание запроса.',
      403: 'Действие недоступно.',
      404: 'Запись не найдена.',
      409: 'Подтверждение изменилось. Откройте форму заново.',
    }
    throw new Error(messages[response.status] || 'Не удалось завершить операцию. Повторите попытку.')
  }
  return response.json()
}
