import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const orgApi = vi.hoisted(() => ({ getOrganizationMembers: vi.fn() }))
vi.mock('@/api/organizations', () => orgApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import OrganizationsManagement from '../OrganizationsManagement.vue'
import CompaniesManagement from '../CompaniesManagement.vue'
import DirectoryModeration from '../directory/DirectoryModeration.vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useCompaniesStore } from '@/stores/companies'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

const MODERATE = 'application.organization.moderate'

const BASE_STUBS = {
  teleport: true,
  RefreshButton: true,
  SearchComponent: true,
  ResponsibleUsersSection: true,
  SelectUnloadPlaces: true,
  SelectTables: true,
  ConfirmationModal: true,
  LoaderSpinner: true,
}

/** Справочники - зеркала друг друга, поэтому кейсы гоняем по обоим одним набором. */
const DIRECTORIES = [
  {
    title: 'организаций',
    component: OrganizationsManagement,
    useStore: useOrganizationsStore,
    fetchAction: 'fetchOrganizationsWithUsers',
    prefix: 'orgs',
    kind: 'organization',
    stubs: { ...BASE_STUBS, OrgHistoryModal: true },
  },
  {
    title: 'компаний',
    component: CompaniesManagement,
    useStore: useCompaniesStore,
    fetchAction: 'fetchCompaniesWithUsers',
    prefix: 'companies',
    kind: 'company',
    stubs: { ...BASE_STUBS, CompanyHistoryModal: true },
  },
]

/** Бета заведена подачей заявки и ждёт разбора, Альфа - проверенная цель привязки. */
function seed() {
  return [
    { id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2, moderation_status: 'approved' },
    { id: 2, name: 'Бета', type: 'Отдел', is_active: true, user_count: 0, moderation_status: 'pending' },
    { id: 3, name: 'Гамма', type: null, is_active: false, user_count: 1, moderation_status: 'approved' },
  ]
}

describe.each(DIRECTORIES)('Справочник $title - разбор записи на проверке (#1875)', (dir) => {
  // Только наименование: бейдж «на проверке» - сосед .truncate-text, и text() по всей
  // колонке приклеил бы его к имени.
  const rowNames = w => w.findAll(`[data-testid="${dir.prefix}-row"] .name-col .truncate-text`).map(c => c.text())
  const panel = w => w.findComponent(DirectoryModeration)
  const details = w => w.find(`[data-testid="${dir.prefix}-details"]`)
  const listMode = w => w.findComponent(`[data-testid="${dir.prefix}-list-mode"]`)

  /**
   * @param {{ allow?: string[], items?: object[], onRefetch?: (store) => void }} options
   */
  async function mountCmp({ allow = [MODERATE], items = seed(), onRefetch } = {}) {
    const store = dir.useStore()
    store.itemsWithUsers = items
    vi.spyOn(store, 'refresh').mockResolvedValue()
    vi.spyOn(store, dir.fetchAction).mockImplementation(async () => {
      onRefetch?.(store)
    })
    vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {})

    const perm = usePermissionsStore()
    perm.mode = 'normal'
    perm.effective = Object.fromEntries(allow.map(key => [key, { value: 'allow', source: 'role' }]))

    const w = mount(dir.component, { global: { stubs: dir.stubs } })
    await flushPromises()
    return { w, store }
  }

  /** Открывает карточку записи «на проверке» (Бета - вторая строка списка). */
  async function selectPending(w) {
    await w.findAll(`[data-testid="${dir.prefix}-row"]`)[1].trigger('click')
    await flushPromises()
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    orgApi.getOrganizationMembers.mockResolvedValue([])
  })

  it('без права разбора плашки в карточке нет', async () => {
    const { w } = await mountCmp({ allow: [] })
    await selectPending(w)

    expect(details(w).exists()).toBe(true)
    expect(w.find(`[data-testid="${dir.prefix}-moderation-card"]`).exists()).toBe(false)
    expect(panel(w).exists()).toBe(false)
  })

  it('с правом показывает плашку разбора для записи на проверке', async () => {
    const { w } = await mountCmp()
    await selectPending(w)

    expect(w.find(`[data-testid="${dir.prefix}-moderation-card"]`).exists()).toBe(true)
    expect(panel(w).props()).toMatchObject({
      kind: dir.kind,
      variant: 'panel',
      entryId: 2,
      entryName: 'Бета',
    })
  })

  it('проверенную запись разбирать не предлагает', async () => {
    const { w } = await mountCmp()
    await w.findAll(`[data-testid="${dir.prefix}-row"]`)[0].trigger('click')
    await flushPromises()

    expect(details(w).exists()).toBe(true)
    expect(panel(w).exists()).toBe(false)
  })

  it('по resolved перечитывает список', async () => {
    const { w, store } = await mountCmp()
    await selectPending(w)

    panel(w).vm.$emit('resolved', { kind: dir.kind, id: 2, name: 'Бета' })
    await flushPromises()

    expect(store[dir.fetchAction]).toHaveBeenCalledWith(true)
  })

  it('подтверждение записи гасит плашку и оставляет карточку открытой', async () => {
    const approved = seed().map(item => (item.id === 2 ? { ...item, moderation_status: 'approved' } : item))
    const { w } = await mountCmp({ onRefetch: store => { store.itemsWithUsers = approved } })
    await selectPending(w)

    panel(w).vm.$emit('resolved', { kind: dir.kind, id: 2, name: 'Бета' })
    await flushPromises()

    expect(details(w).exists()).toBe(true)
    expect(details(w).find('.d-title').text()).toBe('Бета')
    expect(panel(w).exists()).toBe(false)
    expect(rowNames(w)).toEqual(['Альфа', 'Бета'])
  })

  // Привязка физически удаляет разбираемую запись: строка обязана исчезнуть, а карточка -
  // переехать на цель привязки, иначе панель остаётся на мёртвом выборе.
  it('привязка убирает строку из списка и переводит карточку на цель', async () => {
    const afterMerge = seed().filter(item => item.id !== 2)
    const { w } = await mountCmp({ onRefetch: store => { store.itemsWithUsers = afterMerge } })
    await selectPending(w)
    expect(details(w).find('.d-title').text()).toBe('Бета')

    panel(w).vm.$emit('resolved', { kind: dir.kind, id: 1, name: 'Альфа' })
    await flushPromises()

    expect(rowNames(w)).toEqual(['Альфа'])
    expect(details(w).exists()).toBe(true)
    expect(details(w).find('.d-title').text()).toBe('Альфа')
    expect(panel(w).exists()).toBe(false)
  })

  it('привязка к записи вне текущего режима гасит карточку, а не оставляет мёртвый выбор', async () => {
    // Цель привязки в архиве: в режиме активных её строки нет - показывать нечего.
    const afterMerge = seed()
      .filter(item => item.id !== 2)
      .map(item => (item.id === 1 ? { ...item, is_active: false } : item))
    const { w } = await mountCmp({ onRefetch: store => { store.itemsWithUsers = afterMerge } })
    await selectPending(w)

    panel(w).vm.$emit('resolved', { kind: dir.kind, id: 1, name: 'Альфа' })
    await flushPromises()

    expect(rowNames(w)).toEqual([])
    expect(details(w).exists()).toBe(false)
  })

  it('фильтр «На проверке» оставляет только неразобранные записи', async () => {
    const { w } = await mountCmp()
    expect(rowNames(w)).toEqual(['Альфа', 'Бета'])

    listMode(w).vm.$emit('update:modelValue', 'pending')
    await nextTick()

    expect(rowNames(w)).toEqual(['Бета'])
    expect(w.find('.items-count').text()).toBe('На проверке: 1')
  })

  it('«На проверке» - срез активных: архивная запись в него не попадает', async () => {
    const items = seed().map(item => (item.id === 3 ? { ...item, moderation_status: 'pending' } : item))
    const { w } = await mountCmp({ items })

    listMode(w).vm.$emit('update:modelValue', 'pending')
    await nextTick()

    expect(rowNames(w)).toEqual(['Бета'])
  })

  it('разбор в режиме «На проверке» убирает запись из списка и закрывает карточку', async () => {
    const approved = seed().map(item => (item.id === 2 ? { ...item, moderation_status: 'approved' } : item))
    const { w } = await mountCmp({ onRefetch: store => { store.itemsWithUsers = approved } })

    listMode(w).vm.$emit('update:modelValue', 'pending')
    await nextTick()
    await w.findAll(`[data-testid="${dir.prefix}-row"]`)[0].trigger('click')
    await flushPromises()
    expect(panel(w).exists()).toBe(true)

    panel(w).vm.$emit('resolved', { kind: dir.kind, id: 2, name: 'Бета' })
    await flushPromises()

    expect(rowNames(w)).toEqual([])
    expect(details(w).exists()).toBe(false)
  })
})
