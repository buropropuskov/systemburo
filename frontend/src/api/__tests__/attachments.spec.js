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
})
