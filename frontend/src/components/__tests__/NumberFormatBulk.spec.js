import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import NumberFormat from '@/components/NumberFormat.vue'
import { useDeletionsStore } from '@/stores/deletions'

// NumberFormat.vue дёргает apiRequest напрямую (не через api/licenseFormats.js) для
// списка/CRUD - мокаем клиент общим обработчиком по URL. Форма ответа списка -
// РЕАЛЬНАЯ: [{format:{id,name,is_active,...}, cells:[]}] (см. license_formats_test.go).
const apiClientMock = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('@/api/client', () => apiClientMock)

const licenseFormatsApi = vi.hoisted(() => ({
  getLicenseFormatHistory: vi.fn(),
  bulkArchiveLicenseFormats: vi.fn(),
  bulkRestoreLicenseFormats: vi.fn(),
}))
vi.mock('@/api/licenseFormats', () => licenseFormatsApi)

function seedFormats() {
  return [
    { format: { id: 1, name: 'Alpha', is_active: true, country_code: 'RU', is_default: false }, cells: [] },
    { format: { id: 2, name: 'Beta', is_active: true, country_code: 'RU', is_default: false }, cells: [] },
    { format: { id: 3, name: 'Gamma', is_active: true, country_code: 'RU', is_default: false }, cells: [] },
  ]
}

function mountFormats(list = seedFormats()) {
  apiClientMock.apiRequest.mockImplementation(async (url) => {
    if (url.startsWith('/license-plate-formats')) return { ok: true, json: async () => list }
    return { ok: true, json: async () => ({}) }
  })
  return mount(NumberFormat, {
    global: {
      stubs: { Teleport: true, LicensePlateFormatHistoryModal: true },
    },
  })
}

const rowChecks = w => w.findAll('[data-testid="numberformat-row-check"]')
const bulkBar = w => w.find('[data-testid="numberformat-bulk-bar"]')

describe('NumberFormat — групповой выбор и bulk архив/восстановление', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('панель скрыта без выбора, появляется со счётчиком при выборе строки', async () => {
    wrapper = mountFormats()
    await flushPromises()
    expect(bulkBar(wrapper).exists()).toBe(false)

    await rowChecks(wrapper)[0].trigger('click')
    expect(bulkBar(wrapper).exists()).toBe(true)
    expect(bulkBar(wrapper).find('.bulk-count').text()).toBe('Выбрано: 1')
    expect(wrapper.vm.selectedIds).toEqual([1])
  })

  it('shift-клик выделяет диапазон', async () => {
    wrapper = mountFormats()
    await flushPromises()
    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[2].trigger('click', { shiftKey: true })
    expect([...wrapper.vm.selectedIds].sort()).toEqual([1, 2, 3])
  })

  it('select-all выбирает всех, повторный клик снимает', async () => {
    wrapper = mountFormats()
    await flushPromises()
    await wrapper.find('[data-testid="numberformat-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(3)
    await wrapper.find('[data-testid="numberformat-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(0)
  })

  it('bulk-архив: подтверждение -> API с ids, полный успех -> сброс выбора', async () => {
    licenseFormatsApi.bulkArchiveLicenseFormats.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    wrapper = mountFormats()
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[1].trigger('click')
    await wrapper.find('[data-testid="numberformat-bulk-archive"]').trigger('click')
    expect(wrapper.vm.bulkConfirmVisible).toBe(true)

    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(licenseFormatsApi.bulkArchiveLicenseFormats).toHaveBeenCalledWith([1, 2])
    expect(wrapper.vm.selectedIds).toEqual([])
    expect(wrapper.vm.bulkConfirmVisible).toBe(false)
  })

  it('частичный успех -> ui.warning с непрошедшими, выбор сброшен', async () => {
    licenseFormatsApi.bulkArchiveLicenseFormats.mockResolvedValue({ success_count: 1, error_count: 1, errors: [{ id: 2, name: 'Beta', error: 'не найден' }] })
    wrapper = mountFormats()
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
    licenseFormatsApi.bulkArchiveLicenseFormats.mockResolvedValue({ message: 'Не выбраны форматы' })
    wrapper = mountFormats()
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
    licenseFormatsApi.bulkRestoreLicenseFormats.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] })
    wrapper = mountFormats([{ format: { id: 5, name: 'Arch', is_active: false, country_code: null, is_default: false }, cells: [] }])
    await flushPromises()
    await wrapper.vm.onArchiveModeChange('archive')
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    expect(wrapper.find('[data-testid="numberformat-bulk-restore"]').exists()).toBe(true)
    wrapper.vm.startBulkOperation('restore')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(licenseFormatsApi.bulkRestoreLicenseFormats).toHaveBeenCalledWith([5])
  })
})
