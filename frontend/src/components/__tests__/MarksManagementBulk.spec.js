import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import MarksManagement from '@/components/MarksManagement.vue'
import { useDeletionsStore } from '@/stores/deletions'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}))

const marksApi = vi.hoisted(() => ({
  listMarks: vi.fn(),
  createMark: vi.fn(),
  renameMark: vi.fn(),
  archiveMark: vi.fn(),
  restoreMark: vi.fn(),
  getMarkHistory: vi.fn(),
  bulkArchiveMarks: vi.fn(),
  bulkRestoreMarks: vi.fn(),
}))
vi.mock('@/api/marks', () => marksApi)

function seedMarks() {
  return [
    { id: 1, name: 'Alpha', is_active: true },
    { id: 2, name: 'Beta', is_active: true },
    { id: 3, name: 'Gamma', is_active: true },
  ]
}

function mountMarks(list = seedMarks()) {
  marksApi.listMarks.mockResolvedValue(list)
  return mount(MarksManagement, {
    global: {
      stubs: { Teleport: true, MarkHistoryModal: true },
      mocks: { $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() } },
    },
  })
}

const rowChecks = w => w.findAll('[data-testid="marks-row-check"]')
const bulkBar = w => w.find('[data-testid="marks-bulk-bar"]')

describe('MarksManagement — групповой выбор и bulk архив/восстановление', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('панель скрыта без выбора, появляется со счётчиком при выборе строки', async () => {
    wrapper = mountMarks()
    await flushPromises()
    expect(bulkBar(wrapper).exists()).toBe(false)

    await rowChecks(wrapper)[0].trigger('click')
    expect(bulkBar(wrapper).exists()).toBe(true)
    expect(bulkBar(wrapper).find('.bulk-count').text()).toBe('Выбрано: 1')
    expect(wrapper.vm.selectedIds).toEqual([1])
  })

  it('shift-клик выделяет диапазон', async () => {
    wrapper = mountMarks()
    await flushPromises()
    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[2].trigger('click', { shiftKey: true })
    expect([...wrapper.vm.selectedIds].sort()).toEqual([1, 2, 3])
  })

  it('select-all выбирает всех, повторный клик снимает', async () => {
    wrapper = mountMarks()
    await flushPromises()
    await wrapper.find('[data-testid="marks-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(3)
    await wrapper.find('[data-testid="marks-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(0)
  })

  it('bulk-архив: подтверждение -> API с ids, полный успех -> сброс выбора', async () => {
    marksApi.bulkArchiveMarks.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    wrapper = mountMarks()
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[1].trigger('click')
    await wrapper.find('[data-testid="marks-bulk-archive"]').trigger('click')
    expect(wrapper.vm.bulkConfirmVisible).toBe(true)

    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(marksApi.bulkArchiveMarks).toHaveBeenCalledWith([1, 2])
    expect(wrapper.vm.selectedIds).toEqual([])
    expect(wrapper.vm.bulkConfirmVisible).toBe(false)
  })

  it('частичный успех -> ui.warning с непрошедшими, выбор сброшен', async () => {
    marksApi.bulkArchiveMarks.mockResolvedValue({ success_count: 1, error_count: 1, errors: [{ id: 2, name: 'Beta', error: 'не найдена' }] })
    wrapper = mountMarks()
    await flushPromises()
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[1].trigger('click')
    wrapper.vm.startBulkOperation('archive')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning', suffix: expect.stringContaining('Beta') }))
    expect(wrapper.vm.selectedIds).toEqual([])
  })

  it('ошибка-envelope ({message}) -> error-notify, выбор НЕ сброшен, модалка держится', async () => {
    marksApi.bulkArchiveMarks.mockResolvedValue({ message: 'Не выбраны марки' })
    wrapper = mountMarks()
    await flushPromises()
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    await rowChecks(wrapper)[0].trigger('click')
    wrapper.vm.startBulkOperation('archive')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
    expect(wrapper.vm.selectedIds).toEqual([1])
    expect(wrapper.vm.bulkConfirmVisible).toBe(true)
  })

  it('в архивном режиме кнопка Восстановить зовёт restore-API', async () => {
    marksApi.bulkRestoreMarks.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] })
    wrapper = mountMarks([{ id: 5, name: 'Arch', is_active: false }])
    await flushPromises()
    wrapper.vm.onArchiveModeChange('archive')
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    expect(wrapper.find('[data-testid="marks-bulk-restore"]').exists()).toBe(true)
    wrapper.vm.startBulkOperation('restore')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(marksApi.bulkRestoreMarks).toHaveBeenCalledWith([5])
  })
})
