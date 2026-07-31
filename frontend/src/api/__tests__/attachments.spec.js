import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import {
  listAttachments,
  listAllAttachments,
  createAttachment,
  updateAttachment,
  archiveAttachment,
  restoreAttachment,
  getAttachmentHistory,
} from '../attachments'

// apiRequest разворачивает envelope: на успехе json() = data, на ошибке = { message }.
function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/attachments', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('listAttachments / listAllAttachments', () => {
    it('GET /attachments отдаёт только активные', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, display_name: 'Автозаявка' }]))
      const data = await listAttachments()
      expect(apiRequest).toHaveBeenCalledWith('/attachments')
      expect(data).toEqual([{ id: 1, display_name: 'Автозаявка' }])
    })

    it('GET /attachments/all отдаёт активные и архивные (отдельный эндпоинт, не ?include_archived)', async () => {
      apiRequest.mockResolvedValue(okJson([]))
      await listAllAttachments()
      expect(apiRequest).toHaveBeenCalledWith('/attachments/all')
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(listAllAttachments()).rejects.toThrow('Сервер недоступен')
    })
  })

  describe('createAttachment', () => {
    it('POST с полями в snake_case', async () => {
      apiRequest.mockResolvedValue(okJson({ id: 7 }))
      await createAttachment({
        attachmentType: 'people',
        name: 'lyudi',
        displayName: 'Заявка на людей',
        title: 'ЛЮДИ',
        instruction: 'текст',
      })
      expect(apiRequest).toHaveBeenCalledWith('/attachments', {
        method: 'POST',
        body: JSON.stringify({
          attachment_type: 'people',
          name: 'lyudi',
          display_name: 'Заявка на людей',
          title: 'ЛЮДИ',
          instruction: 'текст',
        }),
      })
    })

    it('instruction по умолчанию null когда не передан', async () => {
      apiRequest.mockResolvedValue(okJson({ id: 7 }))
      await createAttachment({ attachmentType: 'cars', name: 'avto', displayName: 'Авто', title: 'АВТО' })
      const [, opts] = apiRequest.mock.calls[0]
      expect(JSON.parse(opts.body)).toMatchObject({ instruction: null })
    })

    it('бросает с сообщением бэка при дубле имени, а не считает успехом', async () => {
      apiRequest.mockResolvedValue(errJson('Attachment with this name already exists', 400))
      await expect(
        createAttachment({ attachmentType: 'cars', name: 'avto', displayName: 'Авто', title: 'АВТО' }),
      ).rejects.toThrow('Attachment with this name already exists')
    })

    it('auto_export не попадает в тело, когда не передан (undefined выпадает из JSON.stringify)', async () => {
      apiRequest.mockResolvedValue(okJson({ id: 7 }))
      await createAttachment({ attachmentType: 'cars', name: 'avto', displayName: 'Авто', title: 'АВТО' })
      const [, opts] = apiRequest.mock.calls[0]
      expect(JSON.parse(opts.body)).not.toHaveProperty('auto_export')
    })

    it('шлёт auto_export, когда вызывающая форма его передала (#1615)', async () => {
      apiRequest.mockResolvedValue(okJson({ id: 7 }))
      await createAttachment({
        attachmentType: 'cars', name: 'avto', displayName: 'Авто', title: 'АВТО', autoExport: false,
      })
      const [, opts] = apiRequest.mock.calls[0]
      expect(JSON.parse(opts.body)).toMatchObject({ auto_export: false })
    })
  })

  describe('updateAttachment', () => {
    it('PUT full-replace всех полей', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await updateAttachment(3, {
        attachmentType: 'items',
        name: 'tmc',
        displayName: 'ТМЦ заявка',
        title: 'ТМЦ',
        instruction: null,
      })
      expect(apiRequest).toHaveBeenCalledWith('/attachments/3', {
        method: 'PUT',
        body: JSON.stringify({
          attachment_type: 'items',
          name: 'tmc',
          display_name: 'ТМЦ заявка',
          title: 'ТМЦ',
          instruction: null,
        }),
      })
    })

    // КРИТИЧНО (#1615): PUT - полная замена полей, поэтому тумблер архива обязан
    // уходить в теле запроса каждый раз, когда форма его показывает - иначе он
    // молча выпадает из сохранения и правка теряется на первом же изменении
    // другого поля.
    it('шлёт auto_export в теле PUT, когда передан', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await updateAttachment(3, {
        attachmentType: 'items', name: 'tmc', displayName: 'ТМЦ заявка', title: 'ТМЦ', autoExport: true,
      })
      const [, opts] = apiRequest.mock.calls[0]
      expect(JSON.parse(opts.body)).toMatchObject({ auto_export: true })
    })

    it('auto_export не попадает в тело PUT, когда не передан', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await updateAttachment(3, {
        attachmentType: 'items', name: 'tmc', displayName: 'ТМЦ заявка', title: 'ТМЦ',
      })
      const [, opts] = apiRequest.mock.calls[0]
      expect(JSON.parse(opts.body)).not.toHaveProperty('auto_export')
    })
  })

  describe('archiveAttachment', () => {
    it('DELETE /attachments/{id} (soft)', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await archiveAttachment(5)
      expect(apiRequest).toHaveBeenCalledWith('/attachments/5', { method: 'DELETE' })
    })
  })

  describe('restoreAttachment', () => {
    it('PUT /attachments/{id}/restore (именно PUT, не POST - контракт вложений)', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await restoreAttachment(5)
      expect(apiRequest).toHaveBeenCalledWith('/attachments/5/restore', { method: 'PUT' })
    })

    it('бросает при ошибке, а не считает успехом', async () => {
      apiRequest.mockResolvedValue(errJson('Вложение не найдено', 404))
      await expect(restoreAttachment(5)).rejects.toThrow('Вложение не найдено')
    })
  })

  describe('getAttachmentHistory', () => {
    it('GET /attachments/{id}/history отдаёт массив записей', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, action_type: 'created' }]))
      const data = await getAttachmentHistory(7)
      expect(apiRequest).toHaveBeenCalledWith('/attachments/7/history')
      expect(data).toEqual([{ id: 1, action_type: 'created' }])
    })

    it('бросает при ошибке загрузки истории', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(getAttachmentHistory(7)).rejects.toThrow('Сервер недоступен')
    })
  })
})
