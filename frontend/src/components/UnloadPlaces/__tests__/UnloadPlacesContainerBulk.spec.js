import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UnloadPlacesContainer from '@/components/UnloadPlaces/UnloadPlacesContainer.vue'
import { useDeletionsStore } from '@/stores/deletions'

const apiRequestMock = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', () => ({ apiRequest: apiRequestMock }))

const unloadPlacesApi = vi.hoisted(() => ({
  bulkArchiveUnloadPlaces: vi.fn(),
  bulkRestoreUnloadPlaces: vi.fn(),
}))
vi.mock('@/api/unload-places', () => unloadPlacesApi)

function okJson(data) {
  return Promise.resolve({ ok: true, json: () => Promise.resolve(data) })
}

function seedPlaces() {
  return [
    { id: 1, name: 'Alpha', is_active: true, status: 'active', current_status: 'open' },
    { id: 2, name: 'Beta', is_active: true, status: 'active', current_status: 'closed' },
    { id: 3, name: 'Gamma', is_active: true, status: 'active', current_status: 'open' },
  ]
}

function mountContainer(list = seedPlaces()) {
  apiRequestMock.mockImplementation((url) => {
    if (typeof url === 'string' && url.startsWith('/unload-places?include_archived=true')) return okJson(list)
    if (url === '/users/me') return okJson({ username: 'tester' })
    return okJson({})
  })
  return mount(UnloadPlacesContainer, {
    global: {
      stubs: { Teleport: true, WorkScheduleTab: true, UnloadPlaceHistoryModal: true },
    },
  })
}

const rowChecks = w => w.findAll('[data-testid="unloadplaces-row-check"]')
const bulkBar = w => w.find('[data-testid="unloadplaces-bulk-bar"]')

describe('UnloadPlacesContainer — групповой выбор и bulk архив/восстановление', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('панель скрыта без выбора, появляется со счётчиком при выборе строки', async () => {
    wrapper = mountContainer()
    await flushPromises()
    expect(bulkBar(wrapper).exists()).toBe(false)

    await rowChecks(wrapper)[0].trigger('click')
    expect(bulkBar(wrapper).exists()).toBe(true)
    expect(bulkBar(wrapper).find('.bulk-count').text()).toBe('Выбрано: 1')
    expect(wrapper.vm.selectedIds).toEqual([1])
  })

  it('shift-клик выделяет диапазон', async () => {
    wrapper = mountContainer()
    await flushPromises()
    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[2].trigger('click', { shiftKey: true })
    expect([...wrapper.vm.selectedIds].sort()).toEqual([1, 2, 3])
  })

  it('select-all выбирает всех, повторный клик снимает', async () => {
    wrapper = mountContainer()
    await flushPromises()
    await wrapper.find('[data-testid="unloadplaces-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(3)
    await wrapper.find('[data-testid="unloadplaces-select-all"]').trigger('change')
    expect(wrapper.vm.selectedIds).toHaveLength(0)
  })

  it('bulk-архив: подтверждение -> API с ids, полный успех -> сброс выбора', async () => {
    unloadPlacesApi.bulkArchiveUnloadPlaces.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    wrapper = mountContainer()
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[1].trigger('click')
    await wrapper.find('[data-testid="unloadplaces-bulk-archive"]').trigger('click')
    expect(wrapper.vm.bulkConfirmVisible).toBe(true)

    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(unloadPlacesApi.bulkArchiveUnloadPlaces).toHaveBeenCalledWith([1, 2])
    expect(wrapper.vm.selectedIds).toEqual([])
    expect(wrapper.vm.bulkConfirmVisible).toBe(false)
  })

  it('частичный успех -> ui.warning с непрошедшими, выбор сброшен', async () => {
    unloadPlacesApi.bulkArchiveUnloadPlaces.mockResolvedValue({ success_count: 1, error_count: 1, errors: [{ id: 2, name: 'Beta', error: 'не найдено' }] })
    wrapper = mountContainer()
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
    unloadPlacesApi.bulkArchiveUnloadPlaces.mockResolvedValue({ message: 'Не выбраны места разгрузки' })
    wrapper = mountContainer()
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
    unloadPlacesApi.bulkRestoreUnloadPlaces.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] })
    wrapper = mountContainer([{ id: 5, name: 'Arch', is_active: false, status: 'active', current_status: 'closed' }])
    await flushPromises()
    wrapper.vm.onArchiveModeChange('archive')
    await flushPromises()

    await rowChecks(wrapper)[0].trigger('click')
    expect(wrapper.find('[data-testid="unloadplaces-bulk-restore"]').exists()).toBe(true)
    wrapper.vm.startBulkOperation('restore')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(unloadPlacesApi.bulkRestoreUnloadPlaces).toHaveBeenCalledWith([5])
  })
})
