import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}))
import { apiRequest, apiRequestRaw } from '@/api/client'
import {
  findSubjectCandidates,
  fetchSubjectReport,
  fetchSubjectDisclosures,
  exportSubjectReport,
} from '../pdSubject'

// apiRequest отдаёт Response с подменённым json(): конверт {success,data} он
// разворачивает уже внутри него. Вернув сам Response, слой отдал бы экрану объект
// вместо массива, и раздел показал бы «ничего не найдено» на живых данных - именно
// это и случилось на стенде при первой ручной проверке.
function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/pdSubject', () => {
  beforeEach(() => vi.clearAllMocks())

  it('findSubjectCandidates отдаёт массив записей, а не Response', async () => {
    apiRequest.mockResolvedValue(okJson([{ id: 4, full_name: 'Мякотных Сергей', has_document: true }]))

    const found = await findSubjectCandidates('Мякотных Сергей')

    expect(Array.isArray(found)).toBe(true)
    expect(found).toHaveLength(1)
    expect(apiRequest).toHaveBeenCalledWith(
      `/pd-subject/candidates?fio=${encodeURIComponent('Мякотных Сергей')}`,
    )
  })

  it('пустой ответ превращается в пустой массив, а не в null', async () => {
    apiRequest.mockResolvedValue(okJson(null))
    await expect(findSubjectCandidates('Иванов Иван')).resolves.toEqual([])
  })

  it('fetchSubjectReport отдаёт разделы справки', async () => {
    apiRequest.mockResolvedValue(okJson({ total: 3, sections: [{ title: 'Сведения', rows: [] }] }))

    const report = await fetchSubjectReport({ registryId: 4 })

    expect(report.total).toBe(3)
    expect(report.sections[0].title).toBe('Сведения')
    expect(apiRequest).toHaveBeenCalledWith('/pd-subject/report?registry_id=4')
  })

  it('ошибка сервера доходит до экрана текстом, а не пустым списком', async () => {
    apiRequest.mockResolvedValue(errJson('у записи нет документа'))
    await expect(fetchSubjectReport({ registryId: 9 })).rejects.toThrow('у записи нет документа')
  })

  it('цель без записи реестра идёт строкой заявки: такой человек тоже обязан находиться', async () => {
    apiRequest.mockResolvedValue(okJson({ total: 1, sections: [] }))
    await fetchSubjectReport({ registryId: 0, employeeId: 42 })
    expect(apiRequest).toHaveBeenCalledWith('/pd-subject/report?employee_id=42')
  })

  it('fetchSubjectDisclosures без записи запрашивает весь журнал', async () => {
    apiRequest.mockResolvedValue(okJson([]))
    await fetchSubjectDisclosures()
    expect(apiRequest).toHaveBeenCalledWith('/pd-subject/disclosures')
  })

  it('exportSubjectReport берёт имя файла из заголовка ответа', async () => {
    apiRequestRaw.mockResolvedValue({
      ok: true,
      headers: { get: () => 'attachment; filename="Сведения_о_субъекте_20260908.xlsx"' },
      blob: vi.fn().mockResolvedValue(new Blob(['x'])),
    })

    const { filename } = await exportSubjectReport({ registry_id: 4, recipient: 'УМВД', request_ref: 'исх. 1' })

    expect(filename).toBe('Сведения_о_субъекте_20260908.xlsx')
  })
})
