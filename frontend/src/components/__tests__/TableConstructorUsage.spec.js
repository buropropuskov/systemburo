import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TableConstructor from '@/components/TableConstructor.vue'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

const apiRequestMock = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', () => ({ apiRequest: apiRequestMock, apiRequestRaw: vi.fn() }))

const systemTablesApi = vi.hoisted(() => ({
  bulkArchiveSystemTables: vi.fn(),
  bulkRestoreSystemTables: vi.fn(),
  getSystemTableUsage: vi.fn(),
  detachAllSystemTable: vi.fn(),
  detachOrganizationFromSystemTable: vi.fn(),
  detachCompanyFromSystemTable: vi.fn(),
}))
vi.mock('@/api/system-tables', () => systemTablesApi)

function okJson(data) {
  return Promise.resolve({ ok: true, json: () => Promise.resolve(data) })
}

function seedTable(overrides = {}) {
  return {
    table: {
      id: 7,
      name: 'post72',
      display_name: 'ПОСТ №72',
      table_type: 'cars',
      is_active: true,
      show_fact_table: false,
      ...overrides,
    },
  }
}

function mountConstructor(list = [seedTable()]) {
  apiRequestMock.mockImplementation((url) => {
    if (typeof url === 'string' && url.startsWith('/system-tables')) return okJson(list)
    return okJson({})
  })
  return mount(TableConstructor, {
    global: {
      stubs: {
        Teleport: true,
        AdminPageShell: { template: '<div><slot /></div>' },
        WorkScheduleTab: true,
        WarningWindowsEditor: true,
        SystemTableColumnsTab: true,
        SystemTableAppearanceTab: true,
        TableConstructorCreateModal: true,
        TableConstructorPhotoSection: true,
        SystemTableHistoryModal: true,
        TextConstructor: true,
      },
    },
  })
}

// Блок привязок теперь на вкладке «Основное» — usage грузится при выборе таблицы
// (watch по selectedTable.table.id), отдельной вкладки «Привязки» больше нет.
async function selectTableWithUsage(wrapper, table = seedTable()) {
  await wrapper.vm.selectTable(table)
  await flushPromises()
}

const detachBtn = w => w.find('.detach-all-btn')

describe('TableConstructor — блок «Привязки» на вкладке «Основное» (usage/detach-all)', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('блок грузит и рендерит организации/компании, архивные помечены (архив)', async () => {
    systemTablesApi.getSystemTableUsage.mockResolvedValue({
      organizations: [
        { id: 1, name: 'ООО Ромашка', is_active: true },
        { id: 2, name: 'ООО Архивная', is_active: false },
      ],
      companies: [{ id: 5, name: 'Компания-5', is_active: true }],
    })
    wrapper = mountConstructor()
    await flushPromises()
    await selectTableWithUsage(wrapper)

    expect(systemTablesApi.getSystemTableUsage).toHaveBeenCalledWith(7)
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

  it('правка поля той же таблицы (id не меняется) не перезагружает привязки', async () => {
    systemTablesApi.getSystemTableUsage.mockResolvedValue({ organizations: [], companies: [] })
    wrapper = mountConstructor()
    await flushPromises()
    await selectTableWithUsage(wrapper)
    expect(systemTablesApi.getSystemTableUsage).toHaveBeenCalledTimes(1)

    // Правка поля той же таблицы (id тот же) — watch по id не срабатывает, usage не перезапрашивается.
    wrapper.vm.selectedTable.table.display_name = 'ПОСТ №72 (изм.)'
    await flushPromises()
    expect(systemTablesApi.getSystemTableUsage).toHaveBeenCalledTimes(1)
  })

  it('кнопка «Отвязать всё» скрыта без права page.admin, видна с ним', async () => {
    systemTablesApi.getSystemTableUsage.mockResolvedValue({
      organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
      companies: [],
    })
    wrapper = mountConstructor()
    await flushPromises()
    // mode по умолчанию 'normal' -> hasPermission('page.admin') === false
    await selectTableWithUsage(wrapper)
    expect(detachBtn(wrapper).exists()).toBe(false)

    usePermissionsStore().mode = 'super'
    await flushPromises()
    expect(detachBtn(wrapper).exists()).toBe(true)
  })

  it('кнопка «Отвязать всё» скрыта, когда привязок нет (даже с правом)', async () => {
    systemTablesApi.getSystemTableUsage.mockResolvedValue({ organizations: [], companies: [] })
    wrapper = mountConstructor()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await selectTableWithUsage(wrapper)
    expect(detachBtn(wrapper).exists()).toBe(false)
    expect(wrapper.text()).toContain('Нет привязанных организаций')
    expect(wrapper.text()).toContain('Нет привязанных компаний')
  })

  it('detach-all: вызывает API, перезагружает usage, notify с числами', async () => {
    systemTablesApi.getSystemTableUsage
      .mockResolvedValueOnce({
        organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
        companies: [{ id: 5, name: 'Компания-5', is_active: true }],
      })
      .mockResolvedValueOnce({ organizations: [], companies: [] })
    systemTablesApi.detachAllSystemTable.mockResolvedValue({ organizations_detached: 1, companies_detached: 1 })
    wrapper = mountConstructor()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await selectTableWithUsage(wrapper)
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    await wrapper.vm.performDetachAll()
    await flushPromises()

    expect(systemTablesApi.detachAllSystemTable).toHaveBeenCalledWith(7)
    expect(systemTablesApi.getSystemTableUsage).toHaveBeenCalledTimes(2) // первичная + после detach
    expect(wrapper.vm.usageHasBindings).toBe(false)
    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ suffix: expect.stringContaining('организаций (1)') }),
    )
    expect(wrapper.findAll('.usage-item')).toHaveLength(0)
  })

  it('detach-all: ошибка бэка -> error-notify, привязки на месте', async () => {
    systemTablesApi.getSystemTableUsage.mockResolvedValue({
      organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
      companies: [],
    })
    systemTablesApi.detachAllSystemTable.mockRejectedValue(new Error('Недостаточно прав'))
    wrapper = mountConstructor()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await selectTableWithUsage(wrapper)
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    await wrapper.vm.performDetachAll()
    await flushPromises()

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', bold: 'Недостаточно прав' }))
    expect(wrapper.vm.detaching).toBe(false)
    // Ошибка detach не зовёт loadUsage — привязки остаются на экране.
    expect(wrapper.findAll('.usage-item')).toHaveLength(1)
  })

  it('переключение таблицы, пока грузятся привязки, не показывает кнопку с чужими цифрами', async () => {
    let resolveB
    systemTablesApi.getSystemTableUsage
      .mockResolvedValueOnce({
        organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
        companies: [{ id: 5, name: 'Компания-5', is_active: true }],
      })
      .mockImplementationOnce(() => new Promise((res) => { resolveB = res }))
    wrapper = mountConstructor()
    await flushPromises()
    usePermissionsStore().mode = 'super'

    // Таблица A с привязками -> кнопка «Отвязать всё» видна (usage грузится при выборе).
    await wrapper.vm.selectTable(seedTable({ id: 7 }))
    await flushPromises()
    expect(detachBtn(wrapper).exists()).toBe(true)

    // Переключились на таблицу B, её привязки ещё не приехали (запрос висит).
    await wrapper.vm.selectTable(seedTable({ id: 8, name: 'post99', display_name: 'ПОСТ №99' }))
    await flushPromises()
    expect(wrapper.vm.usageLoading).toBe(true)
    expect(wrapper.vm.usage.organizations).toHaveLength(0) // не осталось цифр таблицы A
    expect(detachBtn(wrapper).exists()).toBe(false) // кнопка скрыта, пока грузится

    // B оказалось без привязок -> кнопка так и не появилась.
    resolveB({ organizations: [], companies: [] })
    await flushPromises()
    expect(detachBtn(wrapper).exists()).toBe(false)
  })

  it('ошибка загрузки usage -> показывает текст ошибки, а не молчит', async () => {
    systemTablesApi.getSystemTableUsage.mockRejectedValue(new Error('Сервер недоступен'))
    wrapper = mountConstructor()
    await flushPromises()
    await selectTableWithUsage(wrapper)

    expect(wrapper.find('.usage-state--error').exists()).toBe(true)
    expect(wrapper.find('.usage-state--error').text()).toBe('Сервер недоступен')
    expect(wrapper.findAll('.usage-item')).toHaveLength(0)
  })

  it('крестик точечной отвязки: скрыт без права page.admin, виден с ним', async () => {
    systemTablesApi.getSystemTableUsage.mockResolvedValue({
      organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
      companies: [],
    })
    wrapper = mountConstructor()
    await flushPromises()
    await selectTableWithUsage(wrapper) // mode 'normal' -> нет page.admin
    expect(wrapper.findAll('.usage-item__detach')).toHaveLength(0)

    usePermissionsStore().mode = 'super'
    await flushPromises()
    expect(wrapper.findAll('.usage-item__detach')).toHaveLength(1)
  })

  it('точечная отвязка: клик по крестику -> подтверждение -> API + reload + notify', async () => {
    systemTablesApi.getSystemTableUsage
      .mockResolvedValueOnce({
        organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
        companies: [{ id: 5, name: 'Компания-5', is_active: true }],
      })
      .mockResolvedValueOnce({
        organizations: [],
        companies: [{ id: 5, name: 'Компания-5', is_active: true }],
      })
    systemTablesApi.detachOrganizationFromSystemTable.mockResolvedValue({ detached: true })
    wrapper = mountConstructor()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await selectTableWithUsage(wrapper)
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    const btns = wrapper.findAll('.usage-item__detach')
    expect(btns).toHaveLength(2) // org + company
    // Клик по крестику организации открывает подтверждение (target проставлен).
    await btns[0].trigger('click')
    expect(wrapper.vm.detachOneTarget).toEqual({ kind: 'organization', id: 1, name: 'ООО Ромашка' })

    await wrapper.vm.performDetachOne()
    await flushPromises()

    expect(systemTablesApi.detachOrganizationFromSystemTable).toHaveBeenCalledWith(7, 1)
    expect(systemTablesApi.getSystemTableUsage).toHaveBeenCalledTimes(2) // первичная + reload
    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'ООО Ромашка', suffix: expect.stringContaining('отвязана') }),
    )
    // Организация ушла, компания осталась.
    expect(wrapper.findAll('.usage-item')).toHaveLength(1)
    expect(wrapper.text()).toContain('Компания-5')
  })

  it('точечная отвязка: ошибка -> error-notify, привязка на месте', async () => {
    systemTablesApi.getSystemTableUsage.mockResolvedValue({
      organizations: [{ id: 1, name: 'ООО Ромашка', is_active: true }],
      companies: [],
    })
    systemTablesApi.detachOrganizationFromSystemTable.mockRejectedValue(new Error('Недостаточно прав'))
    wrapper = mountConstructor()
    await flushPromises()
    usePermissionsStore().mode = 'super'
    await selectTableWithUsage(wrapper)
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    wrapper.vm.confirmDetachOne('organization', { id: 1, name: 'ООО Ромашка' })
    await wrapper.vm.performDetachOne()
    await flushPromises()

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', bold: 'Недостаточно прав' }))
    expect(wrapper.vm.detachingOne).toBe(false)
    expect(wrapper.findAll('.usage-item')).toHaveLength(1) // ошибка не зовёт reload
  })
})
