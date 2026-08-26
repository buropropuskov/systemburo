import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const dirApi = vi.hoisted(() => ({
  getCompanyMembers: vi.fn(),
  reassignCompanyUsers: vi.fn(),
  getOrganizationMembers: vi.fn(),
  reassignOrganizationUsers: vi.fn(),
}))
vi.mock('@/api/organizations', () => dirApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import CompaniesManagement from '../CompaniesManagement.vue'
import OrganizationsManagement from '../OrganizationsManagement.vue'
import { useCompaniesStore } from '@/stores/companies'
import { useOrganizationsStore } from '@/stores/organizations'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

/**
 * История компании и организации закрыта тем же page.admin.directories, что открывает
 * раздел справочников (#1982). Раньше она требовала page.admin, и кнопку приходилось
 * прятать от админа справочников, чтобы не вела в пустое окно с отказом (#1967);
 * теперь тот же человек историю читает, и гейт стережёт обратное - кнопка не должна
 * исчезнуть у того, кто в разделе работает.
 */

const STUBS = {
  teleport: true,
  RefreshButton: true,
  SearchComponent: true,
  ResponsibleUsersSection: true,
  SelectUnloadPlaces: true,
  SelectTables: true,
  ConfirmationModal: true,
  LoaderSpinner: true,
  CompanyHistoryModal: true,
  OrgHistoryModal: true,
}

const SEED = [{ id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2 }]

// Админ справочников: раздел открыт, page.admin у него нет.
function grantDirectoriesOnly() {
  const perm = usePermissionsStore()
  perm.mode = 'normal'
  perm.effective = { 'page.admin.directories': { value: 'allow' } }
}

// Остаток старого зонтичного права без права раздела: истории он больше не открывает.
function grantLegacyAdminOnly() {
  const perm = usePermissionsStore()
  perm.mode = 'normal'
  perm.effective = { 'page.admin': { value: 'allow' } }
}

async function openCard(component, store, rowTestId) {
  store.itemsWithUsers = SEED
  vi.spyOn(store, 'refresh').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})

  const w = mount(component, { global: { stubs: STUBS } })
  await flushPromises()
  await w.findAll(`[data-testid="${rowTestId}"]`)[0].trigger('click')
  await flushPromises()
  return w
}

describe('CompaniesManagement — вход в историю компании', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    dirApi.getCompanyMembers.mockResolvedValue([])
  })

  it('админ справочников открывает историю компании', async () => {
    grantDirectoriesOnly()
    const store = useCompaniesStore()
    vi.spyOn(store, 'fetchCompaniesWithUsers').mockResolvedValue()
    const w = await openCard(CompaniesManagement, store, 'companies-row')

    const button = w.find('[data-testid="companies-history"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    await flushPromises()
    expect(w.findComponent({ name: 'CompanyHistoryModal' }).exists()).toBe(true)
  })

  it('без права раздела кнопки истории нет, а карточка компании открыта', async () => {
    grantLegacyAdminOnly()
    const store = useCompaniesStore()
    vi.spyOn(store, 'fetchCompaniesWithUsers').mockResolvedValue()
    const w = await openCard(CompaniesManagement, store, 'companies-row')

    // Карточка на месте: соседняя кнопка того же блока гейта не имеет.
    expect(w.find('[data-testid="companies-archive"]').exists()).toBe(true)
    expect(w.find('[data-testid="companies-history"]').exists()).toBe(false)
  })
})

describe('OrganizationsManagement — вход в историю организации', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    dirApi.getOrganizationMembers.mockResolvedValue([])
  })

  it('админ справочников открывает историю организации', async () => {
    grantDirectoriesOnly()
    const store = useOrganizationsStore()
    vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()
    const w = await openCard(OrganizationsManagement, store, 'orgs-row')

    const button = w.find('[data-testid="orgs-history"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    await flushPromises()
    expect(w.findComponent({ name: 'OrgHistoryModal' }).exists()).toBe(true)
  })

  it('без права раздела кнопки истории нет, а карточка организации открыта', async () => {
    grantLegacyAdminOnly()
    const store = useOrganizationsStore()
    vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()
    const w = await openCard(OrganizationsManagement, store, 'orgs-row')

    expect(w.find('[data-testid="orgs-archive"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-history"]').exists()).toBe(false)
  })
})
