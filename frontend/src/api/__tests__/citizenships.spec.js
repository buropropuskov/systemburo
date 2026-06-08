import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import {
  listCitizenships,
  createCitizenship,
  updateCitizenship,
  archiveCitizenship,
  restoreCitizenship,
  getCitizenshipHistory,
} from '../citizenships'

// apiRequest разворачивает envelope: на успехе json() = data, на ошибке = { message }.
function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/citizenships', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('listCitizenships', () => {
    it('GET /citizenships без архивных по умолчанию', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, name: 'Россия' }]))
      const data = await listCitizenships()
      expect(apiRequest).toHaveBeenCalledWith('/citizenships')
      expect(data).toEqual([{ id: 1, name: 'Россия' }])
    })

    it('добавляет include_archived=true', async () => {
      apiRequest.mockResolvedValue(okJson([]))
      await listCitizenships({ includeArchived: true })
      expect(apiRequest).toHaveBeenCalledWith('/citizenships?include_archived=true')
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(listCitizenships()).rejects.toThrow('Сервер недоступен')
    })
  })

  describe('createCitizenship', () => {
    it('POST с флагами в snake_case', async () => {
      apiRequest.mockResolvedValue(okJson({ id: 7 }))
      await createCitizenship({ name: 'Беларусь', isDefault: true, patentRequired: false })
      expect(apiRequest).toHaveBeenCalledWith('/citizenships', {
        method: 'POST',
        body: JSON.stringify({ name: 'Беларусь', is_default: true, patent_required: false }),
      })
    })
  })

  describe('updateCitizenship', () => {
    it('full-replace: шлёт все поля включая icon, чтобы не затереть', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await updateCitizenship(3, { name: 'Казахстан', icon: 'kz', isDefault: false, patentRequired: true })
      expect(apiRequest).toHaveBeenCalledWith('/citizenships/3', {
        method: 'PUT',
        body: JSON.stringify({ name: 'Казахстан', icon: 'kz', is_default: false, patent_required: true }),
      })
    })

    it('icon по умолчанию null когда не передан', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await updateCitizenship(3, { name: 'Казахстан' })
      const [, opts] = apiRequest.mock.calls[0]
      expect(JSON.parse(opts.body)).toMatchObject({ icon: null })
    })
  })

  describe('archiveCitizenship', () => {
    it('DELETE /citizenships/{id}', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await archiveCitizenship(5)
      expect(apiRequest).toHaveBeenCalledWith('/citizenships/5', { method: 'DELETE' })
    })

    it('бросает 409 с сообщением бэка (архив дефолта), а не считает успехом', async () => {
      apiRequest.mockResolvedValue(errJson('Нельзя архивировать гражданство по умолчанию', 409))
      await expect(archiveCitizenship(1)).rejects.toThrow('Нельзя архивировать гражданство по умолчанию')
    })
  })

  describe('restoreCitizenship', () => {
    it('POST /citizenships/{id}/restore', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await restoreCitizenship(5)
      expect(apiRequest).toHaveBeenCalledWith('/citizenships/5/restore', { method: 'POST' })
    })
  })

  describe('getCitizenshipHistory', () => {
    it('GET /citizenships/{id}/history возвращает массив записей', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, action_type: 'created' }]))
      const data = await getCitizenshipHistory(5)
      expect(apiRequest).toHaveBeenCalledWith('/citizenships/5/history')
      expect(data).toEqual([{ id: 1, action_type: 'created' }])
    })

    it('бросает при ошибке загрузки, а не считает успехом', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(getCitizenshipHistory(5)).rejects.toThrow('Сервер недоступен')
    })
  })
})
