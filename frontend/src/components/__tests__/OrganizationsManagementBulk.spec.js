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
import { useOrganizationsStore } from '@/stores/organizations'
import { useDeletionsStore } from '@/stores/deletions'

function seedOrgs() {
  return [
    { id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2 },
    { id: 2, name: 'Бета', type: 'Отдел', is_active: true, user_count: 0 },
    { id: 3, name: 'Гамма', type: null, is_active: true, user_count: 1 },
  ]
}

function seedWithArchived() {
  return [
    ...seedOrgs(),
    { id: 4, name: 'Дельта', type: 'Подрядчик', is_active: false, user_count: 0 },
  ]
}

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
}

async function mountCmp(orgs = seedOrgs()) {
  const store = useOrganizationsStore()
  store.itemsWithUsers = orgs
  vi.spyOn(store, 'refresh').mockResolvedValue()
  vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})

  const w = mount(OrganizationsManagement, { global: { stubs: STUBS } })
  await flushPromises()
  return { w, store }
}

const rowChecks = w => w.findAll('[data-testid="orgs-row-check"]')
const bulkBar = w => w.find('[data-testid="orgs-bulk-bar"]')

describe('OrganizationsManagement — групповой выбор и панель', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    orgApi.getOrganizationMembers.mockResolvedValue([])
  })

  it('панель скрыта без выбора и появляется со счётчиком при выборе строки', async () => {
    const { w } = await mountCmp()
    expect(bulkBar(w).exists()).toBe(false)

    await rowChecks(w)[0].trigger('click')
    expect(bulkBar(w).exists()).toBe(true)
    expect(bulkBar(w).find('.bulk-count').text()).toBe('Выбрано: 1')
    expect(w.vm.selectedIds).toEqual([1])
  })

  it('повторный клик по чекбоксу снимает выбор и прячет панель', async () => {
    const { w } = await mountCmp()
    await rowChecks(w)[0].trigger('click')
    await rowChecks(w)[0].trigger('click')
    expect(w.vm.selectedIds).toEqual([])
    expect(bulkBar(w).exists()).toBe(false)
  })

  it('shift-клик выделяет диапазон строк между якорем и целью', async () => {
    const { w } = await mountCmp()
    await rowChecks(w)[0].trigger('click') // якорь: id 1
    await rowChecks(w)[2].trigger('click', { shiftKey: true }) // диапазон 1..3
    expect([...w.vm.selectedIds].sort()).toEqual([1, 2, 3])
    expect(bulkBar(w).find('.bulk-count').text()).toBe('Выбрано: 3')
  })

  it('shift-клик без якоря = обычный выбор одной строки', async () => {
    const { w } = await mountCmp()
    await rowChecks(w)[1].trigger('click', { shiftKey: true })
    expect(w.vm.selectedIds).toEqual([2])
  })

  it('shift-клик с протухшим якорем (нет в списке) = обычный toggle', async () => {
    const { w } = await mountCmp()
    w.vm.lastSelectedId = 999 // якоря нет в sortedOrganizations -> findIndex === -1
    await rowChecks(w)[1].trigger('click', { shiftKey: true })
    expect(w.vm.selectedIds).toEqual([2])
  })

  it('shift-клик по выделенному диапазону снимает его; якорь следует за последним shift-кликом', async () => {
    const { w } = await mountCmp()
    await rowChecks(w)[0].trigger('click') // id 1, якорь=1
    await rowChecks(w)[2].trigger('click', { shiftKey: true }) // 1..3 выбраны, якорь=3
    expect([...w.vm.selectedIds].sort()).toEqual([1, 2, 3])
    await rowChecks(w)[1].trigger('click', { shiftKey: true }) // диапазон 2..3 снят
    expect(w.vm.selectedIds).toEqual([1])
  })

  it('клик по чекбокс-колонке не открывает детали (@click.stop)', async () => {
    const { w } = await mountCmp()
    await w.findAll('[data-testid="orgs-row"]')[0].find('.check-col').trigger('click')
    await flushPromises()
    expect(w.vm.selectedOrganization).toBe(null)
    expect(w.find('[data-testid="orgs-details"]').exists()).toBe(false)
    expect(orgApi.getOrganizationMembers).not.toHaveBeenCalled()
  })

  it('select-all выбирает все видимые строки и снимает повторным кликом', async () => {
    const { w } = await mountCmp()
    await w.find('[data-testid="orgs-select-all"]').trigger('change')
    expect(w.vm.selectedIds).toEqual([1, 2, 3])
    expect(bulkBar(w).find('.bulk-count').text()).toBe('Выбрано: 3')

    await w.find('[data-testid="orgs-select-all"]').trigger('change')
    expect(w.vm.selectedIds).toEqual([])
  })

  it('панель активного режима: операции + В архив, без Восстановить', async () => {
    const { w } = await mountCmp()
    await rowChecks(w)[0].trigger('click')
    expect(w.find('[data-testid="orgs-bulk-type"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-bulk-unload-places"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-bulk-tables"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-bulk-users"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-bulk-archive"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-bulk-restore"]').exists()).toBe(false)
  })

  it('панель архива: только Восстановить', async () => {
    const { w } = await mountCmp(seedWithArchived())
    await w.vm.onArchiveModeChange('archive')
    await nextTick()
    await rowChecks(w)[0].trigger('click')
    expect(w.find('[data-testid="orgs-bulk-restore"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-bulk-type"]').exists()).toBe(false)
    expect(w.find('[data-testid="orgs-bulk-archive"]').exists()).toBe(false)
  })

  it('«Снять выбор» очищает выделение', async () => {
    const { w } = await mountCmp()
    await w.find('[data-testid="orgs-select-all"]').trigger('change')
    await w.find('[data-testid="orgs-bulk-clear"]').trigger('click')
    expect(w.vm.selectedIds).toEqual([])
    expect(bulkBar(w).exists()).toBe(false)
  })

  it('кнопка операции фиксирует намерение в pendingBulkOp', async () => {
    const { w } = await mountCmp()
    await rowChecks(w)[0].trigger('click')
    await w.find('[data-testid="orgs-bulk-archive"]').trigger('click')
    expect(w.vm.pendingBulkOp).toBe('archive')
  })

  it('смена фильтра типа подрезает выбор до видимых строк', async () => {
    const { w } = await mountCmp()
    await w.find('[data-testid="orgs-select-all"]').trigger('change')
    expect(w.vm.selectedIds).toEqual([1, 2, 3])

    w.findComponent('[data-testid="orgs-type-filter"]').vm.$emit('update:modelValue', ['Отдел'])
    await nextTick()
    expect(w.vm.selectedIds).toEqual([2])
    expect(bulkBar(w).find('.bulk-count').text()).toBe('Выбрано: 1')
  })

  it('смена режима на архив сбрасывает выбор активных', async () => {
    const { w } = await mountCmp(seedWithArchived())
    await rowChecks(w)[0].trigger('click')
    expect(w.vm.selectedIds.length).toBe(1)

    await w.vm.onArchiveModeChange('archive')
    await nextTick()
    expect(w.vm.selectedIds).toEqual([])
  })
})
