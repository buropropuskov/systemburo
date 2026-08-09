import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequestRaw: vi.fn(),
  apiRequest: vi.fn(),
}))
import { apiRequestRaw, apiRequest } from '@/api/client'
import { downloadBlankTemplate, uploadImportList } from '../blankImport'

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

  describe('uploadImportList', () => {
    it('POST /attachments/{id}/import-list с файлом в FormData, 200 при чистом файле', async () => {
      const summary = { read: 3, accepted: 3, rejected: 0 }
      apiRequest.mockResolvedValue(okJson({ rows: [], summary }))

      const file = new File(['x'], 'blank.xlsx')
      const result = await uploadImportList(42, file)

      expect(apiRequest).toHaveBeenCalledTimes(1)
      const [path, options] = apiRequest.mock.calls[0]
      expect(path).toBe('/attachments/42/import-list')
      expect(options.method).toBe('POST')
      expect(options.body).toBeInstanceOf(FormData)
      expect(options.body.get('file')).toBe(file)
      expect(result).toEqual({ rows: [], summary })
    })

    it('207 (частичный успех) тоже res.ok - отдаёт rows и summary как есть', async () => {
      const payload = {
        rows: [
          { row_number: 5, employee: { last_name: 'Иванов' }, errors: [], warnings: [] },
          { row_number: 6, errors: ['Поле «Фамилия» обязательно для заполнения'], warnings: [] },
        ],
        summary: { read: 2, accepted: 1, rejected: 1 },
      }
      apiRequest.mockResolvedValue({ ok: true, status: 207, json: vi.fn().mockResolvedValue(payload) })

      const result = await uploadImportList(42, new File(['x'], 'blank.xlsx'))
      expect(result).toEqual(payload)
    })

    it('400 (кривой бланк) бросает текст бэка как есть', async () => {
      apiRequest.mockResolvedValue(errJson('Бланк изменён: колонки не на своих местах. Скачайте бланк заново и заполните его'))
      await expect(uploadImportList(42, new File(['x'], 'blank.xlsx')))
        .rejects.toThrow('Бланк изменён')
    })

    it('403 без права бросает с текстом ошибки', async () => {
      apiRequest.mockResolvedValue(errJson('Недостаточно прав для этого действия.', 403))
      await expect(uploadImportList(42, new File(['x'], 'blank.xlsx')))
        .rejects.toThrow('Недостаточно прав')
    })

    it('без тела ошибки не падает на разборе json - фолбэк-сообщение', async () => {
      apiRequest.mockResolvedValue({ ok: false, status: 500, json: () => Promise.reject(new Error('no body')) })
      await expect(uploadImportList(42, new File(['x'], 'blank.xlsx')))
        .rejects.toThrow('Не удалось загрузить список')
    })
  })
})

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}
