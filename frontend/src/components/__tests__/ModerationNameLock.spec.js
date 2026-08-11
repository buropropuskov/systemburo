import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

/**
 * Наименование записи, заведённой подачей заявки, до разбора правится только
 * блоком разбора (#1876).
 *
 * Раньше путей было два с разным итогом: «Исправить наименование» гасило
 * moderation_status, а обычное поле «Наименование» слало PUT, после которого
 * статус оставался pending. Админ правил опечатку в привычном поле, видел
 * «Изменения сохранены» и продолжал видеть бейдж «На проверке».
 *
 * Организации и компании - файлы-клоны, поэтому набор гоняется по обоим.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const directoryApi = vi.hoisted(() => ({
  getOrganizationMembers: vi.fn(),
  getCompanyMembers: vi.fn(),
}))
vi.mock('@/api/organizations', () => directoryApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import OrganizationsManagement from '../OrganizationsManagement.vue'
import CompaniesManagement from '../CompaniesManagement.vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useCompaniesStore } from '@/stores/companies'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

const STUBS = {
  teleport: true,
  RefreshButton: true,
  SearchComponent: true,
  ResponsibleUsersSection: true,
  SelectUnloadPlaces: true,
  SelectTables: true,
  ConfirmationModal: true,
  LoaderSpinner: true,
  OrgHistoryModal: true,
  CompanyHistoryModal: true,
  DirectoryModeration: true,
}

/** Первая запись ждёт разбора, вторая уже проверена. */
function seed() {
  return [
    { id: 1, name: 'Ромашка', type: 'Арендатор', is_active: true, user_count: 0, moderation_status: 'pending' },
    { id: 2, name: 'Василёк', type: 'Отдел', is_active: true, user_count: 0, moderation_status: 'approved' },
  ]
}

const CASES = [
  {
    kind: 'организации',
    component: OrganizationsManagement,
    useStore: useOrganizationsStore,
    fetchAction: 'fetchOrganizationsWithUsers',
    tid: 'orgs',
  },
  {
    kind: 'компании',
    component: CompaniesManagement,
    useStore: useCompaniesStore,
    fetchAction: 'fetchCompaniesWithUsers',
    tid: 'companies',
  },
]

const PENDING = 'Ромашка'
const APPROVED = 'Василёк'

describe.each(CASES)('$kind: наименование заблокировано до разбора записи', (c) => {
  // Строки отсортированы по имени, поэтому ищем по подписи, а не по индексу.
  const row = (w, name) => w.findAll(`[data-testid="${c.tid}-row"]`)
    .find(r => r.find('.name-col').text().includes(name))
  const nameInput = (w) => w.find(`[data-testid="${c.tid}-detail-name"]`)
  const typeField = (w) => w.findComponent(`[data-testid="${c.tid}-detail-type"]`)
  const lockHint = (w) => w.find(`[data-testid="${c.tid}-name-lock-hint"]`)
  const saveBtn = (w) => w.find(`[data-testid="${c.tid}-save-name"]`)

  let store

  /**
   * @param {{ canModerate?: boolean, items?: Array }} opts
   * @param {string} name подпись строки, которую выбираем
   */
  async function mountAndSelect({ canModerate = true, items = seed() } = {}, name = PENDING) {
    store = c.useStore()
    store.itemsWithUsers = items
    vi.spyOn(store, 'refresh').mockResolvedValue()
    vi.spyOn(store, c.fetchAction).mockResolvedValue()
    const del = useDeletionsStore()
    vi.spyOn(del, 'notify').mockImplementation(() => {})
    usePermissionsStore().mode = canModerate ? 'super' : 'normal'

    const w = mount(c.component, { global: { stubs: STUBS } })
    await flushPromises()
    await row(w, name).trigger('click')
    await flushPromises()
    return w
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    directoryApi.getOrganizationMembers.mockResolvedValue([])
    directoryApi.getCompanyMembers.mockResolvedValue([])
  })

  it('у записи на проверке поле наименования недоступно', async () => {
    const w = await mountAndSelect()
    expect(nameInput(w).attributes('disabled')).toBeDefined()
  })

  it('у проверенной записи поле наименования доступно', async () => {
    const w = await mountAndSelect({}, APPROVED)
    expect(nameInput(w).attributes('disabled')).toBeUndefined()
    expect(lockHint(w).exists()).toBe(false)
  })

  it('тип остаётся доступным даже у записи на проверке', async () => {
    const w = await mountAndSelect()
    expect(typeField(w).props('disabled')).toBe(false)
  })

  it('с правом разбора подсказка отсылает к действию в блоке разбора', async () => {
    const w = await mountAndSelect({ canModerate: true })
    const text = lockHint(w).text()
    expect(text).toContain('Исправить наименование')
    expect(text).toContain('Разбор записи')
  })

  it('без права разбора подсказка говорит, что нужен сотрудник с этим правом', async () => {
    const w = await mountAndSelect({ canModerate: false })
    const text = lockHint(w).text()
    // Блока разбора на экране нет - отсылать к нему нельзя.
    expect(w.find(`[data-testid="${c.tid}-moderation-card"]`).exists()).toBe(false)
    expect(text).toContain('права на разбор у вас нет')
    expect(text).not.toContain('Исправить наименование')
  })

  it('подпись у кнопки сохранения следует за состоянием записи', async () => {
    const pending = await mountAndSelect()
    expect(saveBtn(pending).element.parentElement.textContent).toContain('сохраняется только тип')

    const approved = await mountAndSelect({}, APPROVED)
    expect(saveBtn(approved).element.parentElement.textContent).toContain('Имя и тип сохраняются вместе')
  })

  it('разбор записи снимает блокировку (event-path через resolved)', async () => {
    const w = await mountAndSelect()
    expect(nameInput(w).attributes('disabled')).toBeDefined()

    // Разбор прошёл: список перечитан, запись пришла проверенной.
    store.itemsWithUsers = [{ ...seed()[0], moderation_status: 'approved' }, seed()[1]]
    w.findComponent({ name: 'DirectoryModeration' }).vm.$emit('resolved', { kind: 'organization', id: 1, name: 'Ромашка' })
    await flushPromises()
    await nextTick()

    expect(nameInput(w).attributes('disabled')).toBeUndefined()
    expect(lockHint(w).exists()).toBe(false)
  })
})
