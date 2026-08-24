import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import MarksManagement from '@/components/MarksManagement.vue'
import CitizenshipManagement from '@/components/CitizenshipManagement.vue'

/**
 * Справочники подключены к переходу из сквозного поиска: `?q` подставляется в фильтр,
 * `?open` раскрывает найденную запись. Без роутера (так их монтируют существующие
 * спеки и кабинетные экраны) поведение прежнее.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}))

const marksApi = vi.hoisted(() => ({
  listMarks: vi.fn(), createMark: vi.fn(), renameMark: vi.fn(), archiveMark: vi.fn(),
  restoreMark: vi.fn(), getMarkHistory: vi.fn(), bulkArchiveMarks: vi.fn(), bulkRestoreMarks: vi.fn(),
}))
vi.mock('@/api/marks', () => marksApi)

const citizenshipApi = vi.hoisted(() => ({
  listCitizenships: vi.fn(), createCitizenship: vi.fn(), renameCitizenship: vi.fn(),
  archiveCitizenship: vi.fn(), restoreCitizenship: vi.fn(), getCitizenshipHistory: vi.fn(),
  bulkArchiveCitizenships: vi.fn(), bulkRestoreCitizenships: vi.fn(),
}))
vi.mock('@/api/citizenships', () => citizenshipApi)

const MARKS = [{ id: 1, name: 'Alpha', is_active: true }, { id: 7, name: 'Volvo', is_active: true }]
const COUNTRIES = [{ id: 2, name: 'Армения', is_active: true }, { id: 9, name: 'Сербия', is_active: true }]

function mountWith(component, query, replace = vi.fn().mockResolvedValue(undefined)) {
  return mount(component, {
    global: {
      stubs: { Teleport: true, MarkHistoryModal: true, CitizenshipHistoryModal: true },
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        ...(query === null ? {} : { $route: { query }, $router: { replace } }),
      },
    },
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  marksApi.listMarks.mockResolvedValue(MARKS)
  citizenshipApi.listCitizenships.mockResolvedValue(COUNTRIES)
})

describe('MarksManagement — переход из сквозного поиска', () => {
  it('карточка найденной марки раскрывается сразу', async () => {
    const wrapper = mountWith(MarksManagement, { q: 'volvo', open: '7' })
    await flushPromises()

    expect(wrapper.vm.selectedMark?.id).toBe(7)
  })

  it('строка поиска из адреса подставляется в фильтр', async () => {
    const wrapper = mountWith(MarksManagement, { q: 'volvo', open: '7' })
    await flushPromises()

    expect(wrapper.vm.searchQuery).toBe('volvo')
  })

  it('open вычищается из адреса', async () => {
    const replace = vi.fn().mockResolvedValue(undefined)
    mountWith(MarksManagement, { q: 'volvo', open: '7' }, replace)
    await flushPromises()

    expect(replace).toHaveBeenCalledWith({ query: { q: 'volvo' } })
  })

  it('записи нет среди загруженных - ничего не открывается', async () => {
    const wrapper = mountWith(MarksManagement, { open: '999' })
    await flushPromises()

    expect(wrapper.vm.selectedMark).toBeNull()
  })

  it('без роутера экран работает как раньше', async () => {
    const wrapper = mountWith(MarksManagement, null)
    await flushPromises()

    expect(wrapper.vm.selectedMark).toBeNull()
    expect(wrapper.vm.searchQuery).toBe('')
  })
})

describe('CitizenshipManagement — переход из сквозного поиска', () => {
  it('карточка найденного гражданства раскрывается сразу', async () => {
    const wrapper = mountWith(CitizenshipManagement, { q: 'сербия', open: '9' })
    await flushPromises()

    expect(wrapper.vm.selectedCitizenship?.id).toBe(9)
  })

  it('без роутера экран работает как раньше', async () => {
    const wrapper = mountWith(CitizenshipManagement, null)
    await flushPromises()

    expect(wrapper.vm.searchQuery).toBe('')
  })
})
