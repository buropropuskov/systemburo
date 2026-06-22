import { apiRequest } from './client'

/**
 * @returns {Promise<Array<{id: number, key: string, value: string, type: string}>>}
 */
export async function getSettings() {
  const response = await apiRequest('/settings')

  if (!response.ok) {
    throw new Error(`Ошибка загрузки настроек: ${response.status}`)
  }

  return response.json()
}

/**
 * @param {string} key
 * @param {string} value
 * @returns {Promise<Object>}
 */
export async function updateSetting(key, value) {
  const response = await apiRequest(`/settings/${key}`, {
    method: 'PUT',
    body: JSON.stringify({ value: String(value) }),
  })

  if (!response.ok) {
    throw new Error(`Ошибка сохранения настройки: ${response.status}`)
  }

  return response.json()
}

/**
 * @returns {Promise<{max_file_size: number, allowed_image_types: string, allowed_doc_types: string}>}
 */
export async function getUploadSettings() {
  const response = await apiRequest('/settings/upload')

  if (!response.ok) {
    throw new Error(`Ошибка загрузки настроек загрузки: ${response.status}`)
  }

  return response.json()
}

/**
 * Публичные контакты Бюро пропусков (телефон, почта). Без авторизации -- нужны
 * на странице логина и в плашке блокировки.
 * @returns {Promise<{phone: string, email: string}>}
 */
export async function getPublicContacts() {
  const response = await apiRequest('/settings/contacts')

  if (!response.ok) {
    throw new Error(`Ошибка загрузки контактов: ${response.status}`)
  }

  const json = await response.json()
  return json.data ?? json
}

/**
 * @returns {Promise<import('@/utils/passwordPolicy').PasswordPolicy>}
 */
export async function getPasswordPolicy() {
  const response = await apiRequest('/settings/password-policy')

  if (!response.ok) {
    throw new Error(`Ошибка загрузки политики паролей: ${response.status}`)
  }

  const json = await response.json()
  return json.data ?? json
}
