import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import { getConsentGate, acceptConsent } from '../pdConsent'

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
// Ошибку envelope client.js уже переложил из `error` в `message` - обёртка
// обязана читать именно его, а не res.text().
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/pdConsent - гейт согласия (#1567)', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('getConsentGate', () => {
    it('GET /consents/gate отдаёт состояние гейта целиком', async () => {
      const state = {
        required: true,
        version: 3,
        text: '<p>Согласие</p>',
        document: { stored_name: 'a.pdf', file_name: 'Согласие.pdf' },
      }
      apiRequest.mockResolvedValue(okJson(state))

      const data = await getConsentGate()

      expect(apiRequest).toHaveBeenCalledWith('/consents/gate')
      expect(data).toEqual(state)
    })

    it('бросает с сообщением сервера', async () => {
      apiRequest.mockResolvedValue(errJson('Сервис недоступен', 503))
      await expect(getConsentGate()).rejects.toThrow('Сервис недоступен')
    })

    it('без разбираемого тела бросает свой текст, а не падает на json()', async () => {
      apiRequest.mockResolvedValue({
        ok: false,
        status: 502,
        json: vi.fn().mockRejectedValue(new SyntaxError('unexpected token')),
      })
      await expect(getConsentGate()).rejects.toThrow(
        'Не удалось проверить согласие на обработку данных',
      )
    })
  })

  describe('acceptConsent', () => {
    it('POST /consents/accept без осмысленного тела - редакцию штампует сервер', async () => {
      apiRequest.mockResolvedValue(okJson({ required: false, version: 3, text: '<p>x</p>' }))

      const data = await acceptConsent()

      expect(apiRequest).toHaveBeenCalledWith('/consents/accept', { method: 'POST', body: '{}' })
      expect(data.required).toBe(false)
    })

    it('бросает с сообщением сервера', async () => {
      apiRequest.mockResolvedValue(errJson('Пользователь не найден', 401))
      await expect(acceptConsent()).rejects.toThrow('Пользователь не найден')
    })
  })
})
