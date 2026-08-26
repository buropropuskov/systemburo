import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  estimateArchiveDownload: vi.fn(),
  runArchiveBackfill: vi.fn(),
  listArchiveItems: vi.fn(),
}))
vi.mock('@/api/fileArchive', () => api)

const attachmentsApi = vi.hoisted(() => ({ listAttachments: vi.fn() }))
vi.mock('@/api/attachments', () => attachmentsApi)

import ArchiveBackfillPanel from '../ArchiveBackfillPanel.vue'
import DateFilter from '@/components/DateFilter.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import ConfirmationModal from '@/components/ConfirmationModal.vue'
import { useDeletionsStore } from '@/stores/deletions'

const ATTACHMENTS = [
  { id: 1, display_name: 'Заявка (машины)' },
  { id: 2, display_name: 'Заявка (люди)' },
]

function mountPanel() {
  setActivePinia(createPinia())
  vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {})
  return mount(ArchiveBackfillPanel, { global: { stubs: { DateFilter: true, Teleport: true } } })
}

async function setRange(w, from, to) {
  const filter = w.findComponent(DateFilter)
  await filter.vm.$emit('update:dateRangeStart', from)
  await filter.vm.$emit('update:dateRangeEnd', to)
}

describe('ArchiveBackfillPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    attachmentsApi.listAttachments.mockResolvedValue(ATTACHMENTS)
    api.estimateArchiveDownload.mockResolvedValue({ file_count: 40, bytes: 4096, exceeds_limit: false })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('без периода кнопка постановки в очередь выключена', async () => {
    const w = mountPanel()
    await flushPromises()
    expect(w.find('[data-testid="abp-submit"]').attributes('disabled')).toBeDefined()
  })

  it('с периодом и без сужения типом ставит бэкфилл в очередь после подтверждения', async () => {
    api.runArchiveBackfill.mockResolvedValue({ queued: 7 })
    const w = mountPanel()
    await flushPromises()
    await setRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))

    await w.find('[data-testid="abp-submit"]').trigger('click')
    await flushPromises()

    expect(api.estimateArchiveDownload).toHaveBeenCalledWith({ dateFrom: '2026-07-01', dateTo: '2026-07-31' })
    expect(w.findComponent(ConfirmationModal).props('show')).toBe(true)

    await w.findComponent(ConfirmationModal).vm.$emit('confirm')
    await flushPromises()

    expect(api.runArchiveBackfill).toHaveBeenCalledWith({
      dateFrom: '2026-07-01', dateTo: '2026-07-31', uniqueAttachmentId: null,
    })
    expect(w.find('[data-testid="abp-queued"]').text()).toContain('7')
  })

  it('тумблер сужения требует выбранный тип и передаёт unique_attachment_id', async () => {
    api.runArchiveBackfill.mockResolvedValue({ queued: 2 })
    const w = mountPanel()
    await flushPromises()
    await setRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))

    await w.findComponent(ToggleSwitch).vm.$emit('update:modelValue', true)
    // Без выбранного типа при включённом сужении кнопка остаётся выключенной.
    expect(w.find('[data-testid="abp-submit"]').attributes('disabled')).toBeDefined()

    await w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 1)
    await w.find('[data-testid="abp-submit"]').trigger('click')
    await flushPromises()
    await w.findComponent(ConfirmationModal).vm.$emit('confirm')
    await flushPromises()

    expect(api.runArchiveBackfill).toHaveBeenCalledWith({
      dateFrom: '2026-07-01', dateTo: '2026-07-31', uniqueAttachmentId: 1,
    })
  })

  it('ошибку постановки в очередь показывает и уведомляет', async () => {
    api.runArchiveBackfill.mockRejectedValue(new Error('Выгрузка бланков выключена в настройках файлового архива'))
    const w = mountPanel()
    await flushPromises()
    await setRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))

    await w.find('[data-testid="abp-submit"]').trigger('click')
    await flushPromises()
    await w.findComponent(ConfirmationModal).vm.$emit('confirm')
    await flushPromises()

    expect(w.find('[data-testid="abp-submit-error"]').text()).toContain('выключена')
    expect(useDeletionsStore().notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
  })

  it('после постановки в очередь опрашивает остаток pending и останавливается на нуле', async () => {
    vi.useFakeTimers()
    api.runArchiveBackfill.mockResolvedValue({ queued: 5 })
    api.listArchiveItems
      .mockResolvedValueOnce({ items: [], meta: { total: 3, page: 1, per_page: 1 } })
      .mockResolvedValueOnce({ items: [], meta: { total: 0, page: 1, per_page: 1 } })

    const w = mountPanel()
    await flushPromises()
    await setRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))
    await w.find('[data-testid="abp-submit"]').trigger('click')
    await flushPromises()
    await w.findComponent(ConfirmationModal).vm.$emit('confirm')
    await flushPromises()

    expect(api.listArchiveItems).toHaveBeenCalledWith({ status: 'pending', page: 1, perPage: 1 })
    expect(w.find('[data-testid="abp-progress"]').exists()).toBe(true)
    expect(w.find('[data-testid="abp-progress"]').text()).toContain('3')

    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()
    expect(w.find('[data-testid="abp-progress"]').exists()).toBe(false) // total=0 остановил опрос
  })

  it('размонтирование останавливает опрос (beforeUnmount)', async () => {
    vi.useFakeTimers()
    api.runArchiveBackfill.mockResolvedValue({ queued: 5 })
    api.listArchiveItems.mockResolvedValue({ items: [], meta: { total: 3, page: 1, per_page: 1 } })

    const w = mountPanel()
    await flushPromises()
    await setRange(w, new Date(2026, 6, 1), new Date(2026, 6, 31))
    await w.find('[data-testid="abp-submit"]').trigger('click')
    await flushPromises()
    await w.findComponent(ConfirmationModal).vm.$emit('confirm')
    await flushPromises()

    const callsBeforeUnmount = api.listArchiveItems.mock.calls.length
    w.unmount()

    await vi.advanceTimersByTimeAsync(20000)
    expect(api.listArchiveItems.mock.calls.length).toBe(callsBeforeUnmount)
  })
})
