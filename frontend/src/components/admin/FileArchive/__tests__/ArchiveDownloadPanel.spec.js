import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  estimateArchiveDownload: vi.fn(),
  issueArchiveDownloadTicket: vi.fn(),
}))
vi.mock('@/api/fileArchive', () => api)

const downloadUtils = vi.hoisted(() => ({
  startTicketDownload: vi.fn(),
}))
vi.mock('@/utils/download', async () => {
  const actual = await vi.importActual('@/utils/download')
  return { ...actual, startTicketDownload: downloadUtils.startTicketDownload }
})

import ArchiveDownloadPanel from '../ArchiveDownloadPanel.vue'
import DateFilter from '@/components/DateFilter.vue'
import { useDeletionsStore } from '@/stores/deletions'

function mountPanel() {
  setActivePinia(createPinia())
  vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {})
  return mount(ArchiveDownloadPanel, { global: { stubs: { DateFilter: true } } })
}

async function applyRange(w, from, to) {
  const filter = w.findComponent(DateFilter)
  await filter.vm.$emit('update:dateRangeStart', from)
  await filter.vm.$emit('update:dateRangeEnd', to)
  await filter.vm.$emit('apply')
  await flushPromises()
}

describe('ArchiveDownloadPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('без выбранного периода кнопка скачивания выключена и оценка не запрашивается', async () => {
    const w = mountPanel()
    expect(w.find('[data-testid="adp-download"]').attributes('disabled')).toBeDefined()
    expect(api.estimateArchiveDownload).not.toHaveBeenCalled()
  })

  it('применение периода запрашивает оценку и включает кнопку', async () => {
    api.estimateArchiveDownload.mockResolvedValue({ file_count: 5, bytes: 2048, exceeds_limit: false })
    const w = mountPanel()

    await applyRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))

    expect(api.estimateArchiveDownload).toHaveBeenCalledWith({ dateFrom: '2026-07-01', dateTo: '2026-07-31' })
    expect(w.find('[data-testid="adp-estimate"]').text()).toContain('Файлов: 5')
    expect(w.find('[data-testid="adp-download"]').attributes('disabled')).toBeUndefined()
  })

  it('превышение лимита блокирует кнопку скачивания', async () => {
    api.estimateArchiveDownload.mockResolvedValue({ file_count: 900, bytes: 3e9, exceeds_limit: true })
    const w = mountPanel()

    await applyRange(w, new Date(2026, 0, 1), new Date(2026, 11, 31))

    expect(w.find('[data-testid="adp-exceeds"]').exists()).toBe(true)
    expect(w.find('[data-testid="adp-download"]').attributes('disabled')).toBeDefined()
  })

  it('объём больше 1 ГБ показывает предупреждение, но не блокирует кнопку', async () => {
    api.estimateArchiveDownload.mockResolvedValue({
      file_count: 100, bytes: 1.5 * 1024 * 1024 * 1024, exceeds_limit: false,
    })
    const w = mountPanel()

    await applyRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))

    expect(w.find('[data-testid="adp-large-warning"]').exists()).toBe(true)
    expect(w.find('[data-testid="adp-download"]').attributes('disabled')).toBeUndefined()
  })

  it('пустой период (0 файлов) не даёт скачать', async () => {
    api.estimateArchiveDownload.mockResolvedValue({ file_count: 0, bytes: 0, exceeds_limit: false })
    const w = mountPanel()

    await applyRange(w, new Date(2026, 6, 1), new Date(2026, 6, 2))

    expect(w.find('[data-testid="adp-empty"]').exists()).toBe(true)
    expect(w.find('[data-testid="adp-download"]').attributes('disabled')).toBeDefined()
  })

  it('клик по кнопке получает билет и запускает скачивание по нему', async () => {
    api.estimateArchiveDownload.mockResolvedValue({ file_count: 3, bytes: 1024, exceeds_limit: false })
    api.issueArchiveDownloadTicket.mockResolvedValue({ ticket: 'tkt-1' })
    const w = mountPanel()

    await applyRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))
    await w.find('[data-testid="adp-download"]').trigger('click')
    await flushPromises()

    expect(api.issueArchiveDownloadTicket).toHaveBeenCalledWith({ dateFrom: '2026-07-01', dateTo: '2026-07-31' })
    expect(downloadUtils.startTicketDownload).toHaveBeenCalledWith('/file-archive/download', 'tkt-1')
  })

  it('ошибку получения билета показывает и уведомляет', async () => {
    api.estimateArchiveDownload.mockResolvedValue({ file_count: 3, bytes: 1024, exceeds_limit: false })
    api.issueArchiveDownloadTicket.mockRejectedValue(new Error('Билет недействителен'))
    const w = mountPanel()

    await applyRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))
    await w.find('[data-testid="adp-download"]').trigger('click')
    await flushPromises()

    expect(w.find('[data-testid="adp-download-error"]').text()).toContain('Билет недействителен')
    expect(useDeletionsStore().notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
  })

  it('seq-guard: только ответ на последний применённый период попадает в состояние', async () => {
    const w = mountPanel()
    let resolveFirst
    api.estimateArchiveDownload.mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve }))

    const filter = w.findComponent(DateFilter)
    await filter.vm.$emit('update:dateRangeStart', new Date(2026, 0, 1))
    await filter.vm.$emit('update:dateRangeEnd', new Date(2026, 0, 5))
    await filter.vm.$emit('apply') // первый (медленный) запрос повис

    api.estimateArchiveDownload.mockResolvedValueOnce({ file_count: 9, bytes: 999, exceeds_limit: false })
    await filter.vm.$emit('update:dateRangeStart', new Date(2026, 1, 1))
    await filter.vm.$emit('update:dateRangeEnd', new Date(2026, 1, 5))
    await filter.vm.$emit('apply') // второй запрос отвечает быстрее
    await flushPromises()

    expect(w.find('[data-testid="adp-estimate"]').text()).toContain('Файлов: 9')

    resolveFirst({ file_count: 1, bytes: 1, exceeds_limit: false }) // устаревший ответ пришёл последним
    await flushPromises()
    expect(w.find('[data-testid="adp-estimate"]').text()).toContain('Файлов: 9')
  })
})
