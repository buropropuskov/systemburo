import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const orgApi = vi.hoisted(() => ({
  getOrganizationMembers: vi.fn(),
  bulkUpdateOrganizationType: vi.fn(),
  bulkAssignOrganizationUnloadPlaces: vi.fn(),
  bulkAssignOrganizationTables: vi.fn(),
  bulkAssignOrganizationUsers: vi.fn(),
  bulkArchiveOrganizations: vi.fn(),
  bulkRestoreOrganizations: vi.fn(),
}))
vi.mock('@/api/organizations', () => orgApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import OrganizationsManagement from '../OrganizationsManagement.vue'
import BulkOperationsModal from '../directories/BulkOperationsModal.vue'
import ConfirmationModal from '../ConfirmationModal.vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useDeletionsStore } from '@/stores/deletions'

function seedOrgs() {
  return [
    { id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2 },
    { id: 2, name: 'Бета', type: 'Отдел', is_active: true, user_count: 0 },
    { id: 3, name: 'Гамма', type: null, is_active: true, user_count: 1 },
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
  BulkOperationsModal: true,
  LoaderSpinner: true,
  OrgHistoryModal: true,
}

async function mountCmp() {
  const store = useOrganizationsStore()
  store.itemsWithUsers = seedOrgs()
  vi.spyOn(store, 'refresh').mockResolvedValue()
  vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})

  const w = mount(OrganizationsManagement, { global: { stubs: STUBS } })
  await flushPromises()
  return { w, store, del }
}

const bulkModal = w => w.findComponent(BulkOperationsModal)
const rowChecks = w => w.findAll('[data-testid="orgs-row-check"]')

describe('OrganizationsManagement — применение групповых операций', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    orgApi.getOrganizationMembers.mockResolvedValue([])
  })

  it('type: открывает модалку, зовёт API, notify и сброс выбора при полном успехе', async () => {
    orgApi.bulkUpdateOrganizationType.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    const { w, store, del } = await mountCmp()
    await rowChecks(w)[0].trigger('click')
    await rowChecks(w)[1].trigger('click')
    expect(w.vm.selectedIds).toEqual([1, 2])

    await w.find('[data-testid="orgs-bulk-type"]').trigger('click')
    expect(w.vm.bulkModalVisible).toBe(true)

    bulkModal(w).vm.$emit('apply', { type: 'Подрядчик' })
    await flushPromises()

    expect(orgApi.bulkUpdateOrganizationType).toHaveBeenCalledWith([1, 2], 'Подрядчик')
    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Тип изменён: ', bold: '2' }))
    expect(w.vm.selectedIds).toEqual([])
    expect(w.vm.bulkModalVisible).toBe(false)
    expect(store.refresh).toHaveBeenCalled()
  })

  it('частичный успех: ui.warning с перечнем непрошедших, без notify', async () => {
    orgApi.bulkAssignOrganizationUnloadPlaces.mockResolvedValue({
      success_count: 1,
      error_count: 1,
      errors: [{ id: 2, name: 'Бета', error: 'нет прав' }],
    })
    const { w, del } = await mountCmp()
    await w.find('[data-testid="orgs-select-all"]').trigger('change')
    expect(w.vm.selectedIds).toEqual([1, 2, 3])

    await w.find('[data-testid="orgs-bulk-unload-places"]').trigger('click')
    bulkModal(w).vm.$emit('apply', { unloadPlaceIds: [5], mode: 'add' })
    await flushPromises()

    expect(orgApi.bulkAssignOrganizationUnloadPlaces).toHaveBeenCalledWith([1, 2, 3], [5], 'add')
    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning', bold: '1 из 3', suffix: '. Не удалось: Бета' }))
    expect(w.vm.selectedIds).toEqual([])
  })

  it('архив: открывает ConfirmationModal и зовёт bulkArchive по подтверждению', async () => {
    orgApi.bulkArchiveOrganizations.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    const { w } = await mountCmp()
    await rowChecks(w)[0].trigger('click')
    await rowChecks(w)[1].trigger('click')

    await w.find('[data-testid="orgs-bulk-archive"]').trigger('click')
    expect(w.vm.bulkConfirmVisible).toBe(true)
    expect(w.vm.bulkModalVisible).toBe(false)

    // Вторая ConfirmationModal в шаблоне — групповая (первая — одиночный архив).
    const confirms = w.findAllComponents(ConfirmationModal)
    confirms[confirms.length - 1].vm.$emit('confirm')
    await flushPromises()

    expect(orgApi.bulkArchiveOrganizations).toHaveBeenCalledWith([1, 2])
    expect(w.vm.selectedIds).toEqual([])
    expect(w.vm.bulkConfirmVisible).toBe(false)
  })

  it('ошибка API (envelope success:false -> message): модалка открыта, выбор цел', async () => {
    orgApi.bulkUpdateOrganizationType.mockResolvedValue({ message: 'Недостаточно прав' })
    const { w, del } = await mountCmp()
    await rowChecks(w)[0].trigger('click')
    await w.find('[data-testid="orgs-bulk-type"]').trigger('click')
    bulkModal(w).vm.$emit('apply', { type: 'Отдел' })
    await flushPromises()

    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Недостаточно прав', type: 'error' }))
    // при ошибке разбора результата выбор и модалку сохраняем для повтора
    expect(w.vm.selectedIds).toEqual([1])
    expect(w.vm.bulkModalVisible).toBe(true)
  })

  it('сетевая ошибка (reject): error-notify, модалка открыта, выбор цел', async () => {
    orgApi.bulkAssignOrganizationTables.mockRejectedValue(new Error('network'))
    const { w, del } = await mountCmp()
    await rowChecks(w)[0].trigger('click')
    await w.find('[data-testid="orgs-bulk-tables"]').trigger('click')
    bulkModal(w).vm.$emit('apply', { tableIds: [7], mode: 'replace' })
    await flushPromises()

    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
    expect(w.vm.selectedIds).toEqual([1])
    expect(w.vm.bulkModalVisible).toBe(true)
    expect(w.vm.bulkSubmitting).toBe(false)
  })
})
