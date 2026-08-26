import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import { listTrash, restoreItems, purgeItem, clearTrash } from '../trash'

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}

// URL ассертим целиком и в том виде, в каком его строит URLSearchParams: запятая
// уезжает как %2C, и «ожидаемая» строка 3,7 покраснела бы на верном коде.
describe('api/trash', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('listTrash', () => {
    it('без фильтров зовёт эндпоинт без query', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1 }]))
      const data = await listTrash(5)
      expect(apiRequest).toHaveBeenCalledWith('/system-tables/5/trash')
      expect(data).toEqual([{ id: 1 }])
    })

    it('мультивыбор организаций уходит comma-списком', async () => {
      apiRequest.mockResolvedValue(okJson([]))
      await listTrash(5, { organizationIds: [3, 7] })
      expect(apiRequest).toHaveBeenCalledWith('/system-tables/5/trash?organization_ids=3%2C7')
    })

    it('одна организация - тот же параметр без разделителя', async () => {
      apiRequest.mockResolvedValue(okJson([]))
      await listTrash(5, { organizationIds: [3] })
      expect(apiRequest).toHaveBeenCalledWith('/system-tables/5/trash?organization_ids=3')
    })

    it('пустой выбор не добавляет параметр', async () => {
      apiRequest.mockResolvedValue(okJson([]))
      await listTrash(5, { organizationIds: [], search: 'кам' })
      expect(apiRequest).toHaveBeenCalledWith('/system-tables/5/trash?search=%D0%BA%D0%B0%D0%BC')
    })

    it('организации соседствуют с поиском и датами', async () => {
      apiRequest.mockResolvedValue(okJson([]))
      await listTrash(9, {
        search: 'A1', organizationIds: [2, 4], dateFrom: '2026-07-01', dateTo: '2026-07-31',
      })
      expect(apiRequest).toHaveBeenCalledWith(
        '/system-tables/9/trash?search=A1&organization_ids=2%2C4&date_from=2026-07-01&date_to=2026-07-31',
      )
    })
  })

  describe('остальные операции корзины', () => {
    it('restoreItems шлёт ids телом', async () => {
      apiRequest.mockResolvedValue(okJson({ restored: 2 }))
      await restoreItems(5, [11, 12])
      expect(apiRequest).toHaveBeenCalledWith('/system-tables/5/trash/restore', {
        method: 'POST',
        body: JSON.stringify({ ids: [11, 12] }),
      })
    })

    it('purgeItem удаляет один элемент', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await purgeItem(5, 11)
      expect(apiRequest).toHaveBeenCalledWith('/system-tables/5/trash/11', { method: 'DELETE' })
    })

    it('clearTrash чистит корзину таблицы', async () => {
      apiRequest.mockResolvedValue(okJson({ purged: 3 }))
      await clearTrash(5)
      expect(apiRequest).toHaveBeenCalledWith('/system-tables/5/trash', { method: 'DELETE' })
    })
  })
})
