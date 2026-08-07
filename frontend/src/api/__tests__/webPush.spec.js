import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import { getWebPushStatus, subscribeWebPush, unsubscribeWebPush, getPushSummary } from '../webPush'

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/webPush', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('getWebPushStatus', () => {
    it('GET /notifications/push/status возвращает статус', async () => {
      const payload = { enabled: true, public_key: 'key', devices: [] }
      apiRequest.mockResolvedValue(okJson(payload))
      const data = await getWebPushStatus()
      expect(apiRequest).toHaveBeenCalledWith('/notifications/push/status')
      expect(data).toEqual(payload)
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(getWebPushStatus()).rejects.toThrow('Сервер недоступен')
    })
  })

  describe('subscribeWebPush', () => {
    it('POST /notifications/push/subscribe с endpoint и вложенными keys (форма PushSubscription.toJSON())', async () => {
      apiRequest.mockResolvedValue(okJson('Подписка сохранена'))
      await subscribeWebPush({ endpoint: 'https://push.example/abc', p256dh: 'p', auth: 'a' })
      expect(apiRequest).toHaveBeenCalledWith('/notifications/push/subscribe', {
        method: 'POST',
        body: JSON.stringify({ endpoint: 'https://push.example/abc', keys: { p256dh: 'p', auth: 'a' } }),
      })
    })

    it('бросает при отказе сервера', async () => {
      apiRequest.mockResolvedValue(errJson('Невалидная подписка', 400))
      await expect(subscribeWebPush({ endpoint: 'e', p256dh: 'p', auth: 'a' }))
        .rejects.toThrow('Невалидная подписка')
    })
  })

  describe('unsubscribeWebPush', () => {
    it('DELETE /notifications/push/subscribe с endpoint в query', async () => {
      apiRequest.mockResolvedValue(okJson({}))
      await unsubscribeWebPush('https://push.example/abc?x=1')
      expect(apiRequest).toHaveBeenCalledWith(
        '/notifications/push/subscribe?endpoint=' + encodeURIComponent('https://push.example/abc?x=1'),
        { method: 'DELETE' },
      )
    })

    it('бросает при ошибке отписки', async () => {
      apiRequest.mockResolvedValue(errJson('Подписка не найдена', 404))
      await expect(unsubscribeWebPush('e')).rejects.toThrow('Подписка не найдена')
    })
  })

  describe('getPushSummary', () => {
    it('GET /notifications/push/summary возвращает сводку', async () => {
      const payload = {
        active_users_total: 40,
        users_with_push: 12,
        users_without_push: 28,
        subscriptions_by_platform: { ios: 8, android: 4, desktop: 0, unknown: 0 },
        users_by_last_login_platform: { ios: 25, android: 10, desktop: 5, unknown: 0 },
      }
      apiRequest.mockResolvedValue(okJson(payload))
      const data = await getPushSummary()
      expect(apiRequest).toHaveBeenCalledWith('/notifications/push/summary')
      expect(data).toEqual(payload)
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(getPushSummary()).rejects.toThrow('Сервер недоступен')
    })
  })
})
