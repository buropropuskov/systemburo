import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const compApi = vi.hoisted(() => ({
  getCompanyMembers: vi.fn(),
  reassignCompanyUsers: vi.fn(),
}))
vi.mock('@/api/organizations', () => compApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import CompaniesManagement from '../CompaniesManagement.vue'
import { useCompaniesStore } from '@/stores/companies'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

function seedCompanies() {
  return [
    { id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2 },
    { id: 2, name: 'Бета', type: 'Отдел', is_active: true, user_count: 0 },
    { id: 3, name: 'Гамма', type: null, is_active: false, user_count: 1 }, // архивная - не цель
  ]
}

const MEMBERS = [
  { id: 10, last_name: 'Иванов', first_name: 'Иван', middle_name: null, position: 'Директор' },
  { id: 11, last_name: 'Петров', first_name: 'Пётр', middle_name: null, position: null },
]

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
}

async function mountCmp({ admin = true, members = MEMBERS } = {}) {
  const store = useCompaniesStore()
  store.itemsWithUsers = seedCompanies()
  vi.spyOn(store, 'refresh').mockResolvedValue()
  vi.spyOn(store, 'fetchCompaniesWithUsers').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})
  const perm = usePermissionsStore()
  perm.mode = admin ? 'super' : 'normal'
  compApi.getCompanyMembers.mockResolvedValue(members)

  const w = mount(CompaniesManagement, { global: { stubs: STUBS } })
  await flushPromises()
  await w.findAll('[data-testid="companies-row"]')[0].trigger('click')
  await flushPromises()
  return { w, store, del }
}

describe('CompaniesManagement — блокеры архивации и перенос', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('блок «нельзя архивировать» и кнопка переноса видны админу при наличии блокеров', async () => {
    const { w } = await mountCmp()
    const notice = w.find('[data-testid="companies-blocking-notice"]')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toContain('нельзя архивировать')
    expect(w.find('[data-testid="companies-reassign-open"]').exists()).toBe(true)
  })

  it('кнопка переноса скрыта без права page.admin (зеркалит BE requireAdmin), блок остаётся', async () => {
    const { w } = await mountCmp({ admin: false })
    expect(w.find('[data-testid="companies-blocking-notice"]').exists()).toBe(true)
    expect(w.find('[data-testid="companies-reassign-open"]').exists()).toBe(false)
  })

  it('блок скрыт, когда блокеров нет', async () => {
    const { w } = await mountCmp({ members: [] })
    expect(w.find('[data-testid="companies-blocking-notice"]').exists()).toBe(false)
  })

  it('цели переноса - только активные компании, кроме исходной (архивные исключены)', async () => {
    const { w } = await mountCmp()
    await w.find('[data-testid="companies-reassign-open"]').trigger('click')
    await nextTick()
    expect(w.vm.reassignTargetOptions).toEqual([{ label: 'Бета', value: 2 }])
  })

  it('перенос: вызывает API (source,target), уведомляет, перечитывает блокеров и обновляет user_count списка', async () => {
    compApi.reassignCompanyUsers.mockResolvedValue({ reassigned: 2 })
    const { w, store, del } = await mountCmp()
    await w.find('[data-testid="companies-reassign-open"]').trigger('click')
    await nextTick()

    w.findComponent('[data-testid="companies-reassign-target"]').vm.$emit('update:modelValue', 2)
    await nextTick()
    compApi.getCompanyMembers.mockResolvedValue([])
    store.fetchCompaniesWithUsers.mockClear()
    await w.find('[data-testid="companies-reassign-submit"]').trigger('click')
    await flushPromises()

    expect(compApi.reassignCompanyUsers).toHaveBeenCalledWith(1, 2)
    expect(del.notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: '2 пользователя', suffix: expect.stringContaining('Бета') }),
    )
    expect(compApi.getCompanyMembers).toHaveBeenLastCalledWith(1)
    expect(store.fetchCompaniesWithUsers).toHaveBeenCalledWith(true)
    expect(w.find('[data-testid="companies-blocking-notice"]').exists()).toBe(false)
  })

  it('модалка не закрывается, пока перенос летит (гейт от гонки loadMembers)', async () => {
    let resolveReassign
    compApi.reassignCompanyUsers.mockReturnValue(new Promise(r => { resolveReassign = r }))
    const { w } = await mountCmp()
    await w.find('[data-testid="companies-reassign-open"]').trigger('click')
    await nextTick()
    w.findComponent('[data-testid="companies-reassign-target"]').vm.$emit('update:modelValue', 2)
    await nextTick()
    await w.find('[data-testid="companies-reassign-submit"]').trigger('click')
    await nextTick()

    expect(w.vm.reassignSubmitting).toBe(true)
    w.vm.closeReassign()
    await nextTick()
    expect(w.vm.reassignVisible).toBe(true)

    resolveReassign({ reassigned: 2 })
    await flushPromises()
    expect(w.vm.reassignVisible).toBe(false)
  })

  it('ошибка переноса показывается через notify type:error (сообщение бэка)', async () => {
    compApi.reassignCompanyUsers.mockRejectedValue(new Error('Целевая компания архивирована'))
    const { w, del } = await mountCmp()
    await w.find('[data-testid="companies-reassign-open"]').trigger('click')
    await nextTick()

    w.findComponent('[data-testid="companies-reassign-target"]').vm.$emit('update:modelValue', 2)
    await nextTick()
    await w.find('[data-testid="companies-reassign-submit"]').trigger('click')
    await flushPromises()

    expect(del.notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'error', bold: 'Целевая компания архивирована' }),
    )
  })
})
