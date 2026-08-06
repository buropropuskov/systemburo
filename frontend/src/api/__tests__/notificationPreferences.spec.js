import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import { getNotificationPreferences, updateNotificationPreferences } from '../notificationPreferences'

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/notificationPreferences', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('getNotificationPreferences', () => {
    it('GET /notifications/preferences возвращает каталог', async () => {
      const payload = [{ type_code: 'application_created', category: 'application', enabled: true }]
      apiRequest.mockResolvedValue(okJson(payload))
      const data = await getNotificationPreferences()
      expect(apiRequest).toHaveBeenCalledWith('/notifications/preferences')
      expect(data).toEqual(payload)
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(getNotificationPreferences()).rejects.toThrow('Сервер недоступен')
    })
  })

  describe('updateNotificationPreferences', () => {
    it('PUT /notifications/preferences с батчем items', async () => {
      apiRequest.mockResolvedValue(okJson({}))
      const items = [{ type_code: 'news_published', enabled: false }]
      await updateNotificationPreferences(items)
      expect(apiRequest).toHaveBeenCalledWith('/notifications/preferences', {
        method: 'PUT',
        body: JSON.stringify({ items }),
      })
    })

    it('бросает при отказе (например, попытке выключить обязательный тип)', async () => {
      apiRequest.mockResolvedValue(errJson('Нельзя отключить обязательное уведомление', 400))
      await expect(updateNotificationPreferences([{ type_code: 'password_changed', enabled: false }]))
        .rejects.toThrow('Нельзя отключить обязательное уведомление')
    })
  })
})
