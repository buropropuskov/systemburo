import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import {
  getArchiveSettings,
  updateArchiveSettings,
  getArchiveTokens,
  previewArchivePath,
  reexportApplication,
} from '../fileArchive'

// apiRequest разворачивает envelope: на успехе json() = data, на ошибке = { message }.
function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/fileArchive', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('getArchiveSettings', () => {
    it('GET /file-archive/settings', async () => {
      apiRequest.mockResolvedValue(okJson({ enabled: false, dir_template: '{год}/{месяц_2}' }))
      const data = await getArchiveSettings()
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/settings')
      expect(data).toEqual({ enabled: false, dir_template: '{год}/{месяц_2}' })
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(getArchiveSettings()).rejects.toThrow('Сервер недоступен')
    })
  })

  describe('updateArchiveSettings', () => {
    it('шлёт только присланные поля в snake_case - остальные не трогает', async () => {
      apiRequest.mockResolvedValue(okJson({ enabled: true }))
      await updateArchiveSettings({ enabled: true, warnPercent: 80 })
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/settings', {
        method: 'PUT',
        body: JSON.stringify({ enabled: true, warn_percent: 80 }),
      })
    })

    it('без аргументов шлёт пустое тело (не сбрасывает настройки)', async () => {
      apiRequest.mockResolvedValue(okJson({}))
      await updateArchiveSettings()
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/settings', {
        method: 'PUT',
        body: JSON.stringify({}),
      })
    })

    it('бросает при ошибке сохранения', async () => {
      apiRequest.mockResolvedValue(errJson('Нет прав', 403))
      await expect(updateArchiveSettings({ enabled: true })).rejects.toThrow('Нет прав')
    })
  })

  describe('getArchiveTokens', () => {
    it('GET /file-archive/tokens', async () => {
      apiRequest.mockResolvedValue(okJson([{ key: 'год', label: 'Год' }]))
      const data = await getArchiveTokens()
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/tokens')
      expect(data).toEqual([{ key: 'год', label: 'Год' }])
    })
  })

  describe('previewArchivePath', () => {
    it('POST /file-archive/preview с дефолтом application_id: 0', async () => {
      apiRequest.mockResolvedValue(okJson({ rel_path: '2026/07' }))
      await previewArchivePath({ dirTemplate: '{год}', fileTemplate: '{имя}.xlsx' })
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/preview', {
        method: 'POST',
        body: JSON.stringify({ dir_template: '{год}', file_template: '{имя}.xlsx', application_id: 0 }),
      })
    })

    it('передаёт application_id при указании', async () => {
      apiRequest.mockResolvedValue(okJson({ rel_path: '2026/07' }))
      await previewArchivePath({ applicationId: 42 })
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/preview', {
        method: 'POST',
        body: JSON.stringify({ dir_template: '', file_template: '', application_id: 42 }),
      })
    })
  })

  describe('reexportApplication', () => {
    it('POST /file-archive/applications/:id/reexport', async () => {
      apiRequest.mockResolvedValue(okJson({ application_id: 5, items: [] }))
      const data = await reexportApplication(5)
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/applications/5/reexport', { method: 'POST' })
      expect(data).toEqual({ application_id: 5, items: [] })
    })

    it('бросает при ошибке (например, выключен рубильник)', async () => {
      apiRequest.mockResolvedValue(errJson('Выгрузка бланков выключена в настройках файлового архива', 409))
      await expect(reexportApplication(5)).rejects.toThrow('Выгрузка бланков выключена')
    })
  })
})
