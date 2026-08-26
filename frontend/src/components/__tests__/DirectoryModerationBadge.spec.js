import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const orgApi = vi.hoisted(() => ({
  getOrganizationMembers: vi.fn(),
  getCompanyMembers: vi.fn(),
}))
vi.mock('@/api/organizations', () => orgApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import OrganizationsManagement from '../OrganizationsManagement.vue'
import CompaniesManagement from '../CompaniesManagement.vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useCompaniesStore } from '@/stores/companies'
import { useDeletionsStore } from '@/stores/deletions'

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
}

// Строки как их отдаёт /with-users-extended: обычная запись приходит проверенной,
// заведённая подачей заявки - на проверке (#1437).
function seedRows() {
  return [
    { id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2, moderation_status: 'approved' },
    { id: 2, name: 'Бета-Строй', type: 'Подрядчик', is_active: true, user_count: 0, moderation_status: 'pending' },
  ]
}

describe('Бейдж «на проверке» в справочниках организаций и компаний (#1437)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    orgApi.getOrganizationMembers.mockResolvedValue([])
    orgApi.getCompanyMembers.mockResolvedValue([])
    const del = useDeletionsStore()
    vi.spyOn(del, 'notify').mockImplementation(() => {})
  })

  it('организация на проверке помечена в строке и в панели деталей', async () => {
    const store = useOrganizationsStore()
    store.itemsWithUsers = seedRows()
    vi.spyOn(store, 'refresh').mockResolvedValue()
    vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()

    const w = mount(OrganizationsManagement, { global: { stubs: STUBS } })
    await flushPromises()

    const badges = w.findAll('[data-testid="orgs-row-pending"]')
    expect(badges).toHaveLength(1)
    expect(badges[0].text()).toBe('на проверке')

    const rows = w.findAll('[data-testid="orgs-row"]')
    expect(rows[1].find('[data-testid="orgs-row-pending"]').exists()).toBe(true)
    // Внутри .truncate-text бейдж срезало бы многоточием вместе с длинным наименованием -
    // он обязан остаться соседом усечённого имени, а не его частью.
    expect(w.find('.truncate-text [data-testid="orgs-row-pending"]').exists()).toBe(false)

    await rows[1].trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="orgs-details-pending"]').exists()).toBe(true)

    await rows[0].trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="orgs-details-pending"]').exists()).toBe(false)
  })

  // Записи из выборок без moderation_status (поле просто не пришло) - не «на проверке»:
  // бейдж рисуется по реальному значению, а не по отсутствию approved.
  it('строка без статуса разбора бейджа не получает', async () => {
    const store = useOrganizationsStore()
    store.itemsWithUsers = [{ id: 1, name: 'Альфа', type: null, is_active: true, user_count: 0 }]
    vi.spyOn(store, 'refresh').mockResolvedValue()
    vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()

    const w = mount(OrganizationsManagement, { global: { stubs: STUBS } })
    await flushPromises()

    expect(w.find('[data-testid="orgs-row-pending"]').exists()).toBe(false)
  })

  it('компания на проверке помечена так же', async () => {
    const store = useCompaniesStore()
    store.itemsWithUsers = seedRows()
    vi.spyOn(store, 'refresh').mockResolvedValue()
    vi.spyOn(store, 'fetchCompaniesWithUsers').mockResolvedValue()

    const w = mount(CompaniesManagement, { global: { stubs: STUBS } })
    await flushPromises()

    const badges = w.findAll('[data-testid="companies-row-pending"]')
    expect(badges).toHaveLength(1)
    expect(badges[0].text()).toBe('на проверке')

    expect(w.find('.truncate-text [data-testid="companies-row-pending"]').exists()).toBe(false)

    const rows = w.findAll('[data-testid="companies-row"]')
    await rows[1].trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="companies-details-pending"]').exists()).toBe(true)
  })
})
