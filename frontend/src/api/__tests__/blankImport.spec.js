import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequestRaw: vi.fn(),
}))
import { apiRequestRaw } from '@/api/client'
import { downloadBlankTemplate } from '../blankImport'

function blobRes(ok, { disposition = '', blob = new Blob(['x']) } = {}) {
  return {
    ok,
    status: ok ? 200 : 404,
    blob: () => Promise.resolve(blob),
    headers: { get: (h) => (h === 'Content-Disposition' ? disposition : null) },
  }
}

function errRes(status, body) {
  return { ok: false, status, json: () => Promise.resolve(body) }
}

describe('api/blankImport', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('downloadBlankTemplate', () => {
    it('GET /attachments/{id}/blank-template отдаёт blob и кириллическое имя из filename*', async () => {
      const disposition = "attachment; filename=\"blank.xlsx\"; filename*=UTF-8''%D0%91%D0%BB%D0%B0%D0%BD%D0%BA.xlsx"
      apiRequestRaw.mockResolvedValue(blobRes(true, { disposition }))

      const { blob, filename } = await downloadBlankTemplate(42)

      expect(apiRequestRaw).toHaveBeenCalledWith('/attachments/42/blank-template')
      expect(blob).toBeInstanceOf(Blob)
      expect(filename).toBe('Бланк.xlsx')
    })

    it('без Content-Disposition отдаёт имя-фолбэк с id вложения', async () => {
      apiRequestRaw.mockResolvedValue(blobRes(true, { disposition: '' }))
      const { filename } = await downloadBlankTemplate(7)
      expect(filename).toBe('blank_template_7.xlsx')
    })

    it('404 без размеченного списка бросает текст бэка как есть', async () => {
      apiRequestRaw.mockResolvedValue(errRes(404, {
        success: false,
        error: 'В бланке не размечен список участников',
      }))
      await expect(downloadBlankTemplate(5)).rejects.toThrow('В бланке не размечен список участников')
    })

    it('404 без активного шаблона бросает свой текст бэка', async () => {
      apiRequestRaw.mockResolvedValue(errRes(404, {
        success: false,
        error: 'Шаблон бланка не настроен',
      }))
      await expect(downloadBlankTemplate(5)).rejects.toThrow('Шаблон бланка не настроен')
    })

    it('403 бросает с текстом ошибки, а не отдаёт пустой blob как успех', async () => {
      apiRequestRaw.mockResolvedValue(errRes(403, {
        success: false,
        error: 'Недостаточно прав для этого действия.',
      }))
      await expect(downloadBlankTemplate(5)).rejects.toThrow('Недостаточно прав')
    })

    it('без тела ошибки не падает на разборе json - фолбэк-сообщение', async () => {
      apiRequestRaw.mockResolvedValue({ ok: false, status: 500, json: () => Promise.reject(new Error('no body')) })
      await expect(downloadBlankTemplate(5)).rejects.toThrow('Не удалось скачать бланк для заполнения')
    })
  })
})
