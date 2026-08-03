import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}))
import { apiRequest, apiRequestRaw } from '@/api/client'
import {
  getArchiveSettings,
  updateArchiveSettings,
  getArchiveTokens,
  previewArchivePath,
  reexportApplication,
  getArchiveStats,
  estimateArchiveDownload,
  issueArchiveDownloadTicket,
  runArchiveBackfill,
  listArchiveItems,
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

  describe('getArchiveStats', () => {
    it('GET /file-archive/stats', async () => {
      apiRequest.mockResolvedValue(okJson({ used_bytes: 100, file_count: 1 }))
      const data = await getArchiveStats()
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/stats')
      expect(data).toEqual({ used_bytes: 100, file_count: 1 })
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Файловый архив недоступен: каталог не настроен', 503))
      await expect(getArchiveStats()).rejects.toThrow('каталог не настроен')
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

  describe('estimateArchiveDownload', () => {
    it('POST /file-archive/estimate с границами периода', async () => {
      apiRequest.mockResolvedValue(okJson({ file_count: 3, bytes: 1024, exceeds_limit: false }))
      const data = await estimateArchiveDownload({ dateFrom: '2026-07-01', dateTo: '2026-07-31' })
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/estimate', {
        method: 'POST',
        body: JSON.stringify({ date_from: '2026-07-01', date_to: '2026-07-31' }),
      })
      expect(data).toEqual({ file_count: 3, bytes: 1024, exceeds_limit: false })
    })

    it('бросает при некорректном периоде', async () => {
      apiRequest.mockResolvedValue(errJson('Некорректный период', 400))
      await expect(estimateArchiveDownload({ dateFrom: 'x', dateTo: 'y' })).rejects.toThrow('Некорректный период')
    })
  })

  describe('issueArchiveDownloadTicket', () => {
    it('POST /file-archive/download-ticket', async () => {
      apiRequest.mockResolvedValue(okJson({ ticket: 'abc' }))
      const data = await issueArchiveDownloadTicket({ dateFrom: '2026-07-01', dateTo: '2026-07-31' })
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/download-ticket', {
        method: 'POST',
        body: JSON.stringify({ date_from: '2026-07-01', date_to: '2026-07-31' }),
      })
      expect(data).toEqual({ ticket: 'abc' })
    })

    it('бросает 413 при превышении предела выгрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Выгрузка за этот период больше допустимого предела', 413))
      await expect(issueArchiveDownloadTicket({ dateFrom: '2026-01-01', dateTo: '2026-12-31' }))
        .rejects.toThrow('больше допустимого предела')
    })
  })

  describe('runArchiveBackfill', () => {
    it('POST /file-archive/backfill без типа вложения', async () => {
      apiRequest.mockResolvedValue(okJson({ queued: 12 }))
      const data = await runArchiveBackfill({ dateFrom: '2026-07-01', dateTo: '2026-07-31' })
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/backfill', {
        method: 'POST',
        body: JSON.stringify({ date_from: '2026-07-01', date_to: '2026-07-31' }),
      })
      expect(data).toEqual({ queued: 12 })
    })

    it('добавляет unique_attachment_id, если тип указан', async () => {
      apiRequest.mockResolvedValue(okJson({ queued: 3 }))
      await runArchiveBackfill({ dateFrom: '2026-07-01', dateTo: '2026-07-31', uniqueAttachmentId: 7 })
      expect(apiRequest).toHaveBeenCalledWith('/file-archive/backfill', {
        method: 'POST',
        body: JSON.stringify({ date_from: '2026-07-01', date_to: '2026-07-31', unique_attachment_id: 7 }),
      })
    })

    it('бросает, если архив выключен', async () => {
      apiRequest.mockResolvedValue(errJson('Выгрузка бланков выключена в настройках файлового архива', 409))
      await expect(runArchiveBackfill({ dateFrom: '2026-07-01', dateTo: '2026-07-31' }))
        .rejects.toThrow('выключена')
    })
  })

  describe('listArchiveItems', () => {
    function rawOk(body) {
      return { ok: true, status: 200, json: vi.fn().mockResolvedValue(body) }
    }
    function rawErr(body, status = 400) {
      return { ok: false, status, json: vi.fn().mockResolvedValue(body) }
    }

    it('GET /file-archive/items с фильтром статуса и пагинацией', async () => {
      apiRequestRaw.mockResolvedValue(rawOk({
        success: true,
        data: [{ id: 1, status: 'failed' }],
        meta: { total: 1, page: 1, per_page: 20 },
      }))
      const { items, meta } = await listArchiveItems({ status: 'failed', page: 1, perPage: 20 })
      expect(apiRequestRaw).toHaveBeenCalledWith('/file-archive/items?page=1&per_page=20&status=failed')
      expect(items).toEqual([{ id: 1, status: 'failed' }])
      expect(meta).toEqual({ total: 1, page: 1, per_page: 20 })
    })

    it('без статуса не шлёт параметр status', async () => {
      apiRequestRaw.mockResolvedValue(rawOk({ success: true, data: [], meta: { total: 0, page: 1, per_page: 20 } }))
      await listArchiveItems({ page: 2, perPage: 20 })
      expect(apiRequestRaw).toHaveBeenCalledWith('/file-archive/items?page=2&per_page=20')
    })

    it('бросает при ошибке сервера', async () => {
      apiRequestRaw.mockResolvedValue(rawErr({ success: false, error: 'Некорректный статус' }))
      await expect(listArchiveItems({ status: 'bogus' })).rejects.toThrow('Некорректный статус')
    })
  })
})
