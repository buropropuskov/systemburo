import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import CitizenshipManagement from '@/components/CitizenshipManagement.vue'
import { useDeletionsStore } from '@/stores/deletions'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}))

const citizenshipsApi = vi.hoisted(() => ({
  listCitizenships: vi.fn(),
  createCitizenship: vi.fn(),
  updateCitizenship: vi.fn(),
  archiveCitizenship: vi.fn(),
  restoreCitizenship: vi.fn(),
  getCitizenshipHistory: vi.fn(),
  bulkArchiveCitizenships: vi.fn(),
  bulkRestoreCitizenships: vi.fn(),
}))
vi.mock('@/api/citizenships', () => citizenshipsApi)

function seedCitizenships() {
  return [
    { id: 1, name: 'Alpha', is_active: true, is_default: false, patent_required: false },
    { id: 2, name: 'Beta', is_active: true, is_default: false, patent_required: false },
    { id: 3, name: 'Gamma', is_active: true, is_default: false, patent_required: false },
  ]
}

function mountCitizenships(list = seedCitizenships()) {
  citizenshipsApi.listCitizenships.mockResolvedValue(list)
  return mount(CitizenshipManagement, {
    global: {
      stubs: { Teleport: true, CitizenshipHistoryModal: true },
      mocks: { $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() } },
    },
  })
}

const rowChecks = w => w.findAll('[data-testid="citizenship-row-check"]')
const bulkBar = w => w.find('[data-testid="citizenship-bulk-bar"]')

describe('CitizenshipManagement — групповой выбор и bulk архив/восстановление', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('панель скрыта без выбора, появляется со счётчиком при выборе строки', async () => {
    wrapper = mountCitizenships()
    await flushPromises()
    expect(bulkBar(wrapper).exists()).toBe(false)

    await rowChecks(wrapper)[0].trigger('click')
    expect(bulkBar(wrapper).exists()).toBe(true)
    expect(bulkBar(wrapper).find('.bulk-count').text()).toBe('Выбрано: 1')
    expect(wrapper.vm.selectedIds).toEqual([1])
  })

  it('shift-клик выделяет диапазон', async () => {
    wrapper = mountCitizenships()
    await flushPromises()
    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[2].trigger('click', { shiftKey: true })
    expect([...wrapper.vm.selectedIds].sort()).toEqual([1, 2, 3])
  })

  it('select-all выбирает всех, повторный клик снимает', async () => {
    wrapper = mountCitizenships()
    await flushPromises()
    await wrapper.find('[data-testid="citizenship-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(3)
    await wrapper.find('[data-testid="citizenship-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(0)
  })

  it('bulk-архив: подтверждение -> API с ids, полный успех -> сброс выбора', async () => {
    citizenshipsApi.bulkArchiveCitizenships.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    wrapper = mountCitizenships()
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[1].trigger('click')
    await wrapper.find('[data-testid="citizenship-bulk-archive"]').trigger('click')
    expect(wrapper.vm.bulkConfirmVisible).toBe(true)

    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(citizenshipsApi.bulkArchiveCitizenships).toHaveBeenCalledWith([1, 2])
    expect(wrapper.vm.selectedIds).toEqual([])
    expect(wrapper.vm.bulkConfirmVisible).toBe(false)
  })

  it('частичный успех -> ui.warning с непрошедшими, выбор сброшен', async () => {
    citizenshipsApi.bulkArchiveCitizenships.mockResolvedValue({ success_count: 1, error_count: 1, errors: [{ id: 2, name: 'Beta', error: 'не найдена' }] })
    wrapper = mountCitizenships()
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
    citizenshipsApi.bulkArchiveCitizenships.mockResolvedValue({ message: 'Не выбраны гражданства' })
    wrapper = mountCitizenships()
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
    citizenshipsApi.bulkRestoreCitizenships.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] })
    wrapper = mountCitizenships([{ id: 5, name: 'Arch', is_active: false, is_default: false, patent_required: false }])
    await flushPromises()
    wrapper.vm.onArchiveModeChange('archive')
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    expect(wrapper.find('[data-testid="citizenship-bulk-restore"]').exists()).toBe(true)
    wrapper.vm.startBulkOperation('restore')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(citizenshipsApi.bulkRestoreCitizenships).toHaveBeenCalledWith([5])
  })
})
