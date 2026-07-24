import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UnloadPlacesContainer from '@/components/UnloadPlaces/UnloadPlacesContainer.vue'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

const apiRequestMock = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', () => ({ apiRequest: apiRequestMock }))

const unloadPlacesApi = vi.hoisted(() => ({
  bulkArchiveUnloadPlaces: vi.fn(),
  bulkRestoreUnloadPlaces: vi.fn(),
  getUnloadPlaceUsage: vi.fn(),
  detachAllUnloadPlace: vi.fn(),
}))
vi.mock('@/api/unload-places', () => unloadPlacesApi)

function okJson(data) {
  return Promise.resolve({ ok: true, json: () => Promise.resolve(data) })
}

function seedPlaces() {
  return [{ id: 7, name: 'Alpha', is_active: true, status: 'active', current_status: 'open' }]
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

// Выбрать место и открыть вкладку «Привязки» (watch грузит usage лениво).
async function openUsageTab(wrapper) {
  wrapper.vm.selectPlace({ id: 7, name: 'Alpha', is_active: true })
  await flushPromises()
  wrapper.vm.activeTab = 'usage'
  await flushPromises()
}

const detachBtn = w => w.find('.detach-all-btn')

describe('UnloadPlacesContainer — вкладка «Привязки» (usage/detach-all)', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('вкладка грузит и рендерит организации/компании, архивные помечены (архив)', async () => {
    unloadPlacesApi.getUnloadPlaceUsage.mockResolvedValue({
      organizations: [
        { id: 1, name: 'ООО Ромашка', is_active: true },
        { id: 2, name: 'ООО Архивная', is_active: false },
      ],
      companies: [{ id: 5, name: 'Компания-5', is_active: true }],
    })
    wrapper = mountContainer()
    await flushPromises()
    await openUsageTab(wrapper)

    expect(unloadPlacesApi.getUnloadPlaceUsage).toHaveBeenCalledWith(7)
    const items = wrapper.findAll('.usage-item')
    expect(items).toHaveLength(3)
    expect(wrapper.text()).toContain('ООО Ромашка')
    expect(wrapper.text()).toContain('Компания-5')
    // Ровно одна архивная метка (у неактивной организации), активные — без неё.
    const archived = wrapper.findAll('.usage-item__archived')
    expect(archived).toHaveLength(1)
    expect(archived[0].text()).toBe('(архив)')
    expect(wrapper.text()).toContain('Организации: 2')
    expect(wrapper.text()).toContain('Компании: 1')
  })

  it('кнопка «Отвязать всё» скрыта без права page.admin, видна с ним', async () => {
    unloadPlacesApi.getUnloadPlaceUsage.mockResolvedValue({
      organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
      companies: [],
    })
    wrapper = mountContainer()
    await flushPromises()
    // mode по умолчанию 'normal' -> hasPermission('page.admin') === false
    await openUsageTab(wrapper)
    expect(detachBtn(wrapper).exists()).toBe(false)

    usePermissionsStore().mode = 'super'
    await flushPromises()
    expect(detachBtn(wrapper).exists()).toBe(true)
  })

  it('кнопка «Отвязать всё» скрыта, когда привязок нет (даже с правом)', async () => {
    unloadPlacesApi.getUnloadPlaceUsage.mockResolvedValue({ organizations: [], companies: [] })
    wrapper = mountContainer()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await openUsageTab(wrapper)
    expect(detachBtn(wrapper).exists()).toBe(false)
    expect(wrapper.text()).toContain('Нет привязанных организаций')
    expect(wrapper.text()).toContain('Нет привязанных компаний')
  })

  it('detach-all: вызывает API, перезагружает usage, notify с числами', async () => {
    unloadPlacesApi.getUnloadPlaceUsage
      .mockResolvedValueOnce({
        organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
        companies: [{ id: 5, name: 'Компания-5', is_active: true }],
      })
      .mockResolvedValueOnce({ organizations: [], companies: [] })
    unloadPlacesApi.detachAllUnloadPlace.mockResolvedValue({ organizations_detached: 1, companies_detached: 1 })
    wrapper = mountContainer()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await openUsageTab(wrapper)
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    await wrapper.vm.performDetachAll()
    await flushPromises()

    expect(unloadPlacesApi.detachAllUnloadPlace).toHaveBeenCalledWith(7)
    expect(unloadPlacesApi.getUnloadPlaceUsage).toHaveBeenCalledTimes(2) // первичная + после detach
    expect(wrapper.vm.usageHasBindings).toBe(false)
    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ suffix: expect.stringContaining('организаций (1)') }),
    )
    expect(wrapper.findAll('.usage-item')).toHaveLength(0)
  })

  it('detach-all: ошибка бэка -> error-notify, привязки на месте', async () => {
    unloadPlacesApi.getUnloadPlaceUsage.mockResolvedValue({
      organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
      companies: [],
    })
    unloadPlacesApi.detachAllUnloadPlace.mockRejectedValue(new Error('Недостаточно прав'))
    wrapper = mountContainer()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await openUsageTab(wrapper)
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    await wrapper.vm.performDetachAll()
    await flushPromises()

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', bold: 'Недостаточно прав' }))
    expect(wrapper.vm.detaching).toBe(false)
    // Ошибка detach не зовёт loadUsage — привязки остаются на экране.
    expect(wrapper.findAll('.usage-item')).toHaveLength(1)
  })

  it('переключение места, пока грузятся привязки, не показывает кнопку с чужими цифрами', async () => {
    let resolveB
    unloadPlacesApi.getUnloadPlaceUsage
      .mockResolvedValueOnce({
        organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
        companies: [{ id: 5, name: 'Компания-5', is_active: true }],
      })
      .mockImplementationOnce(() => new Promise((res) => { resolveB = res }))
    wrapper = mountContainer()
    await flushPromises()
    usePermissionsStore().mode = 'super'

    // Место A с привязками -> кнопка «Отвязать всё» видна.
    wrapper.vm.selectPlace({ id: 7, name: 'Alpha', is_active: true })
    await flushPromises()
    wrapper.vm.activeTab = 'usage'
    await flushPromises()
    expect(detachBtn(wrapper).exists()).toBe(true)

    // Переключились на место B, его привязки ещё не приехали (запрос висит).
    wrapper.vm.selectPlace({ id: 8, name: 'Beta', is_active: true })
    await flushPromises()
    wrapper.vm.activeTab = 'usage'
    await flushPromises()
    expect(wrapper.vm.usageLoading).toBe(true)
    expect(wrapper.vm.usage.organizations).toHaveLength(0) // не осталось цифр места A
    expect(detachBtn(wrapper).exists()).toBe(false) // кнопка скрыта, пока грузится

    // B оказалось без привязок -> кнопка так и не появилась.
    resolveB({ organizations: [], companies: [] })
    await flushPromises()
    expect(detachBtn(wrapper).exists()).toBe(false)
  })

  it('ошибка загрузки usage -> показывает текст ошибки, а не молчит', async () => {
    unloadPlacesApi.getUnloadPlaceUsage.mockRejectedValue(new Error('Сервер недоступен'))
    wrapper = mountContainer()
    await flushPromises()
    await openUsageTab(wrapper)

    expect(wrapper.find('.usage-state--error').exists()).toBe(true)
    expect(wrapper.find('.usage-state--error').text()).toBe('Сервер недоступен')
    expect(wrapper.findAll('.usage-item')).toHaveLength(0)
  })
})
