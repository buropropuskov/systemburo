import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import {
  listAllGuideSections,
  updateGuideSection,
  uploadGuideFile,
  deleteGuideFile,
} from '../guide'

// apiRequest разворачивает envelope: на успехе json() = data, на ошибке = { message }.
function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/guide — админ-методы', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('listAllGuideSections', () => {
    it('GET /guide/admin/sections и возвращает массив разделов', async () => {
      apiRequest.mockResolvedValue(okJson([{ role: 'user', title: 'Пользователь' }]))
      const data = await listAllGuideSections()
      expect(apiRequest).toHaveBeenCalledWith('/guide/admin/sections')
      expect(data).toEqual([{ role: 'user', title: 'Пользователь' }])
    })

    it('пробрасывает ошибку загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(listAllGuideSections()).rejects.toThrow('Сервер недоступен')
    })
  })

  describe('updateGuideSection', () => {
    it('PUT /guide/admin/sections/:role с lead+items', async () => {
      apiRequest.mockResolvedValue(okJson({ role: 'admin', lead: 'L', items: ['a'] }))
      const data = await updateGuideSection('admin', { lead: 'L', items: ['a'] })
      expect(apiRequest).toHaveBeenCalledWith('/guide/admin/sections/admin', {
        method: 'PUT',
        body: JSON.stringify({ lead: 'L', items: ['a'] }),
      })
      expect(data).toEqual({ role: 'admin', lead: 'L', items: ['a'] })
    })
  })

  describe('uploadGuideFile', () => {
    it('PUT multipart с полем file', async () => {
      apiRequest.mockResolvedValue(okJson({ role: 'user', file: { name: 'g.pdf' } }))
      const file = new File(['%PDF-1.4'], 'g.pdf', { type: 'application/pdf' })
      const data = await uploadGuideFile('user', file)

      expect(apiRequest).toHaveBeenCalledTimes(1)
      const [url, opts] = apiRequest.mock.calls[0]
      expect(url).toBe('/guide/admin/sections/user/file')
      expect(opts.method).toBe('PUT')
      expect(opts.body).toBeInstanceOf(FormData)
      expect(opts.body.get('file')).toBe(file)
      expect(data).toEqual({ role: 'user', file: { name: 'g.pdf' } })
    })
  })

  describe('deleteGuideFile', () => {
    it('DELETE /guide/admin/sections/:role/file', async () => {
      apiRequest.mockResolvedValue(okJson({ role: 'user', file: null }))
      const data = await deleteGuideFile('user')
      expect(apiRequest).toHaveBeenCalledWith('/guide/admin/sections/user/file', {
        method: 'DELETE',
      })
      expect(data).toEqual({ role: 'user', file: null })
    })
  })
})
