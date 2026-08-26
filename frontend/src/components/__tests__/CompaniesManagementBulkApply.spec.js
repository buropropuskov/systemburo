import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const compApi = vi.hoisted(() => ({
  getCompanyMembers: vi.fn(),
  bulkUpdateCompanyType: vi.fn(),
  bulkAssignCompanyUnloadPlaces: vi.fn(),
  bulkAssignCompanyTables: vi.fn(),
  bulkAssignCompanyUsers: vi.fn(),
  bulkArchiveCompanies: vi.fn(),
  bulkRestoreCompanies: vi.fn(),
}))
vi.mock('@/api/organizations', () => compApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import CompaniesManagement from '../CompaniesManagement.vue'
import BulkOperationsModal from '../directories/BulkOperationsModal.vue'
import ConfirmationModal from '../ConfirmationModal.vue'
import { useCompaniesStore } from '@/stores/companies'
import { useDeletionsStore } from '@/stores/deletions'

function seedComps() {
  return [
    { id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2 },
    { id: 2, name: 'Бета', type: 'Отдел', is_active: true, user_count: 0 },
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
  CompanyHistoryModal: true,
}

async function mountCmp() {
  const store = useCompaniesStore()
  store.itemsWithUsers = seedComps()
  vi.spyOn(store, 'refresh').mockResolvedValue()
  vi.spyOn(store, 'fetchCompaniesWithUsers').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})

  const w = mount(CompaniesManagement, { global: { stubs: STUBS } })
  await flushPromises()
  return { w, store, del }
}

const bulkModal = w => w.findComponent(BulkOperationsModal)

describe('CompaniesManagement — применение групповых операций', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    compApi.getCompanyMembers.mockResolvedValue([])
  })

  it('type: зовёт company-API (не org), notify и сброс выбора', async () => {
    compApi.bulkUpdateCompanyType.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    const { w, store, del } = await mountCmp()
    await w.find('[data-testid="companies-select-all"]').trigger('change')
    expect(w.vm.selectedIds).toEqual([1, 2])

    await w.find('[data-testid="companies-bulk-type"]').trigger('click')
    expect(w.vm.bulkModalVisible).toBe(true)
    bulkModal(w).vm.$emit('apply', { type: 'Компания' })
    await flushPromises()

    expect(compApi.bulkUpdateCompanyType).toHaveBeenCalledWith([1, 2], 'Компания')
    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Тип изменён: ', bold: '2' }))
    expect(w.vm.selectedIds).toEqual([])
    expect(store.refresh).toHaveBeenCalled()
  })

  it('частичный успех: ui.warning с перечнем', async () => {
    compApi.bulkAssignCompanyTables.mockResolvedValue({
      success_count: 1,
      error_count: 1,
      errors: [{ id: 2, name: 'Бета', error: 'ошибка' }],
    })
    const { w, del } = await mountCmp()
    await w.find('[data-testid="companies-select-all"]').trigger('change')
    await w.find('[data-testid="companies-bulk-tables"]').trigger('click')
    bulkModal(w).vm.$emit('apply', { tableIds: [7], mode: 'replace' })
    await flushPromises()

    expect(compApi.bulkAssignCompanyTables).toHaveBeenCalledWith([1, 2], [7], 'replace')
    expect(del.notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning', bold: '1 из 2', suffix: '. Не удалось: Бета' }))
  })

  it('архив: подтверждение зовёт bulkArchiveCompanies', async () => {
    compApi.bulkArchiveCompanies.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    const { w } = await mountCmp()
    await w.find('[data-testid="companies-select-all"]').trigger('change')
    await w.find('[data-testid="companies-bulk-archive"]').trigger('click')
    expect(w.vm.bulkConfirmVisible).toBe(true)

    const confirms = w.findAllComponents(ConfirmationModal)
    confirms[confirms.length - 1].vm.$emit('confirm')
    await flushPromises()

    expect(compApi.bulkArchiveCompanies).toHaveBeenCalledWith([1, 2])
    expect(w.vm.selectedIds).toEqual([])
  })
})
