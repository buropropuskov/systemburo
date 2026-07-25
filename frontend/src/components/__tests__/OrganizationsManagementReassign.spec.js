import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const orgApi = vi.hoisted(() => ({
  getOrganizationMembers: vi.fn(),
  reassignOrganizationUsers: vi.fn(),
}))
vi.mock('@/api/organizations', () => orgApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import OrganizationsManagement from '../OrganizationsManagement.vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

function seedOrgs() {
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
  OrgHistoryModal: true,
}

async function mountCmp({ admin = true, members = MEMBERS } = {}) {
  const store = useOrganizationsStore()
  store.itemsWithUsers = seedOrgs()
  vi.spyOn(store, 'refresh').mockResolvedValue()
  vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})
  const perm = usePermissionsStore()
  perm.mode = admin ? 'super' : 'normal'
  orgApi.getOrganizationMembers.mockResolvedValue(members)

  const w = mount(OrganizationsManagement, { global: { stubs: STUBS } })
  await flushPromises()
  // Выбираем первую активную организацию (id=1) - грузятся её участники (блокеры).
  await w.findAll('[data-testid="orgs-row"]')[0].trigger('click')
  await flushPromises()
  return { w, store, del }
}

describe('OrganizationsManagement — блокеры архивации и перенос', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('блок «нельзя архивировать» и кнопка переноса видны админу при наличии блокеров', async () => {
    const { w } = await mountCmp()
    const notice = w.find('[data-testid="orgs-blocking-notice"]')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toContain('нельзя архивировать')
    expect(w.find('[data-testid="orgs-reassign-open"]').exists()).toBe(true)
  })

  it('кнопка переноса скрыта без права page.admin (зеркалит BE requireAdmin), блок остаётся', async () => {
    const { w } = await mountCmp({ admin: false })
    // Блокеров видно (список не гейтится), но переносить нельзя - кнопки нет.
    expect(w.find('[data-testid="orgs-blocking-notice"]').exists()).toBe(true)
    expect(w.find('[data-testid="orgs-reassign-open"]').exists()).toBe(false)
  })

  it('блок скрыт, когда блокеров нет', async () => {
    const { w } = await mountCmp({ members: [] })
    expect(w.find('[data-testid="orgs-blocking-notice"]').exists()).toBe(false)
  })

  it('цели переноса - только активные организации, кроме исходной (архивные исключены)', async () => {
    const { w } = await mountCmp()
    await w.find('[data-testid="orgs-reassign-open"]').trigger('click')
    await nextTick()
    // Источник id=1, архивная id=3 исключены -> остаётся Бета(2).
    expect(w.vm.reassignTargetOptions).toEqual([{ label: 'Бета', value: 2 }])
  })

  it('перенос: вызывает API (source,target), уведомляет, перечитывает блокеров и обновляет user_count списка', async () => {
    orgApi.reassignOrganizationUsers.mockResolvedValue({ reassigned: 2 })
    const { w, store, del } = await mountCmp()
    await w.find('[data-testid="orgs-reassign-open"]').trigger('click')
    await nextTick()

    w.findComponent('[data-testid="orgs-reassign-target"]').vm.$emit('update:modelValue', 2)
    await nextTick()
    // После переноса источник освобождён - следующий loadMembers вернёт пусто.
    orgApi.getOrganizationMembers.mockResolvedValue([])
    store.fetchOrganizationsWithUsers.mockClear()
    await w.find('[data-testid="orgs-reassign-submit"]').trigger('click')
    await flushPromises()

    expect(orgApi.reassignOrganizationUsers).toHaveBeenCalledWith(1, 2)
    expect(del.notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: '2 пользователя', suffix: expect.stringContaining('Бета') }),
    )
    expect(orgApi.getOrganizationMembers).toHaveBeenLastCalledWith(1)
    // user_count в списке меняется у источника и цели -> перечитываем список.
    expect(store.fetchOrganizationsWithUsers).toHaveBeenCalledWith(true)
    expect(w.find('[data-testid="orgs-blocking-notice"]').exists()).toBe(false)
  })

  it('модалка не закрывается, пока перенос летит (гейт от гонки loadMembers)', async () => {
    let resolveReassign
    orgApi.reassignOrganizationUsers.mockReturnValue(new Promise(r => { resolveReassign = r }))
    const { w } = await mountCmp()
    await w.find('[data-testid="orgs-reassign-open"]').trigger('click')
    await nextTick()
    w.findComponent('[data-testid="orgs-reassign-target"]').vm.$emit('update:modelValue', 2)
    await nextTick()
    await w.find('[data-testid="orgs-reassign-submit"]').trigger('click')
    await nextTick()

    expect(w.vm.reassignSubmitting).toBe(true)
    w.vm.closeReassign() // Escape/оверлей/крестик пока летит запрос - no-op
    await nextTick()
    expect(w.vm.reassignVisible).toBe(true)

    resolveReassign({ reassigned: 2 })
    await flushPromises()
    expect(w.vm.reassignVisible).toBe(false)
  })

  it('ошибка переноса показывается через notify type:error (сообщение бэка)', async () => {
    orgApi.reassignOrganizationUsers.mockRejectedValue(new Error('Целевая организация архивирована'))
    const { w, del } = await mountCmp()
    await w.find('[data-testid="orgs-reassign-open"]').trigger('click')
    await nextTick()

    w.findComponent('[data-testid="orgs-reassign-target"]').vm.$emit('update:modelValue', 2)
    await nextTick()
    await w.find('[data-testid="orgs-reassign-submit"]').trigger('click')
    await flushPromises()

    expect(del.notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'error', bold: 'Целевая организация архивирована' }),
    )
  })
})
