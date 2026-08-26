import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

const clientApi = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('@/api/client', () => clientApi)

const utApi = vi.hoisted(() => ({
  getUserTypeBlockingUsers: vi.fn(),
  reassignUserTypeUsers: vi.fn(),
}))
vi.mock('@/api/user-types', () => utApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import UserTypes from '../UserTypes.vue'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'

const TYPES = [
  { id: 1, name: 'Менеджер', code: 'manager', is_system: false, users_count: 2 },
  { id: 2, name: 'Бюро пропусков', code: 'buropropuskov', is_system: true, users_count: 3 },
  { id: 3, name: 'Охрана', code: 'security', is_system: false, users_count: 0 },
]

// Блокеры = ВСЕ пользователи типа, вкл. архивного (is_active:false, Петров).
const BLOCKERS = [
  { id: 10, username: 'ivanov', last_name: 'Иванов', first_name: 'Иван', middle_name: null, position: 'Менеджер', is_active: true },
  { id: 11, username: 'petrov', last_name: 'Петров', first_name: 'Пётр', middle_name: null, position: null, is_active: false },
]

const STUBS = {
  teleport: true,
  RefreshButton: true,
  SearchComponent: true,
  ConfirmationModal: true,
  UserTypeHistoryModal: true,
}

function seedApiRequest() {
  clientApi.apiRequest.mockImplementation((url) => {
    if (url === '/user-types-management') {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(TYPES.map(t => ({ ...t }))) })
    }
    if (url === '/users/me') {
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ username: 'admin' }) })
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
  })
}

async function mountCmp({ admin = true, selectId = 1 } = {}) {
  const perm = usePermissionsStore()
  perm.mode = admin ? 'super' : 'normal'
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})

  const w = mount(UserTypes, { global: { stubs: STUBS } })
  await flushPromises()
  await w.find(`[data-testid="utype-row-${selectId}"]`).trigger('click')
  await flushPromises()
  return { w, del }
}

describe('UserTypes — блокеры удаления и перенос', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    seedApiRequest()
    // По умолчанию у типа есть блокеры; тип «Охрана» (id=3) - пустой.
    utApi.getUserTypeBlockingUsers.mockImplementation((id) =>
      Promise.resolve(id === 3 ? [] : BLOCKERS),
    )
  })

  it('блок «нельзя удалить» и кнопка переноса видны админу при наличии пользователей', async () => {
    const { w } = await mountCmp()
    const notice = w.find('[data-testid="utype-blocking-notice"]')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toContain('нельзя удалить')
    expect(w.find('[data-testid="utype-reassign-open"]').exists()).toBe(true)
  })

  it('кнопка переноса скрыта без права page.admin (зеркалит BE requireAdmin), блок остаётся', async () => {
    const { w } = await mountCmp({ admin: false })
    expect(w.find('[data-testid="utype-blocking-notice"]').exists()).toBe(true)
    expect(w.find('[data-testid="utype-reassign-open"]').exists()).toBe(false)
  })

  it('у системного типа блок «нельзя удалить» скрыт (систему не удаляют), но список пользователей показан', async () => {
    const { w } = await mountCmp({ selectId: 2 })
    expect(w.find('[data-testid="utype-blocking-notice"]').exists()).toBe(false)
    expect(w.findAll('.type-user').length).toBe(BLOCKERS.length)
  })

  it('архивный пользователь помечен бейджем «архив»', async () => {
    const { w } = await mountCmp()
    const rows = w.findAll('.type-user')
    expect(rows.length).toBe(2)
    const archived = rows.filter(r => r.find('.archived-badge').exists())
    expect(archived.length).toBe(1)
    expect(archived[0].text()).toContain('Петров')
    expect(archived[0].find('.archived-badge').text()).toBe('архив')
  })

  it('цели переноса - все типы, кроме источника; системные НЕ исключены', async () => {
    const { w } = await mountCmp()
    await w.find('[data-testid="utype-reassign-open"]').trigger('click')
    await nextTick()
    // Источник id=1 исключён; системный «Бюро пропусков» (id=2) остаётся целью.
    expect(w.vm.reassignTargetOptions).toEqual([
      { label: 'Бюро пропусков', value: 2 },
      { label: 'Охрана', value: 3 },
    ])
  })

  it('перенос: вызывает API (source, target_type_id), уведомляет, перечитывает блокеров и обновляет список', async () => {
    utApi.reassignUserTypeUsers.mockResolvedValue({ reassigned: 2 })
    const { w, del } = await mountCmp()
    await w.find('[data-testid="utype-reassign-open"]').trigger('click')
    await nextTick()

    w.findComponent('[data-testid="utype-reassign-target"]').vm.$emit('update:modelValue', 2)
    await nextTick()
    // Источник освобождён - следующая загрузка блокеров вернёт пусто.
    utApi.getUserTypeBlockingUsers.mockResolvedValue([])
    clientApi.apiRequest.mockClear()
    seedApiRequest()
    await w.find('[data-testid="utype-reassign-submit"]').trigger('click')
    await flushPromises()

    expect(utApi.reassignUserTypeUsers).toHaveBeenCalledWith(1, 2)
    expect(del.notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: '2 пользователя', suffix: expect.stringContaining('Бюро пропусков') }),
    )
    expect(utApi.getUserTypeBlockingUsers).toHaveBeenLastCalledWith(1)
    // Список типов перечитан (users_count источника упал, у цели вырос).
    expect(clientApi.apiRequest).toHaveBeenCalledWith('/user-types-management', expect.anything())
    expect(w.find('[data-testid="utype-blocking-notice"]').exists()).toBe(false)
  })

  it('окно не закрывается, пока перенос летит (гейт от гонки loadTypeUsers)', async () => {
    let resolveReassign
    utApi.reassignUserTypeUsers.mockReturnValue(new Promise(r => { resolveReassign = r }))
    const { w } = await mountCmp()
    await w.find('[data-testid="utype-reassign-open"]').trigger('click')
    await nextTick()
    w.findComponent('[data-testid="utype-reassign-target"]').vm.$emit('update:modelValue', 3)
    await nextTick()
    await w.find('[data-testid="utype-reassign-submit"]').trigger('click')
    await nextTick()

    expect(w.vm.reassignSubmitting).toBe(true)
    w.vm.closeReassign() // Escape/оверлей/крестик пока летит запрос - no-op
    await nextTick()
    expect(w.vm.reassignVisible).toBe(true)

    resolveReassign({ reassigned: 1 })
    await flushPromises()
    expect(w.vm.reassignVisible).toBe(false)
  })

  it('ошибка переноса показывается через notify type:error (сообщение бэка)', async () => {
    utApi.reassignUserTypeUsers.mockRejectedValue(new Error('Системный тип нельзя освободить'))
    const { w, del } = await mountCmp()
    await w.find('[data-testid="utype-reassign-open"]').trigger('click')
    await nextTick()

    w.findComponent('[data-testid="utype-reassign-target"]').vm.$emit('update:modelValue', 3)
    await nextTick()
    await w.find('[data-testid="utype-reassign-submit"]').trigger('click')
    await flushPromises()

    expect(del.notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'error', bold: 'Системный тип нельзя освободить' }),
    )
  })

  it('блок скрыт, когда у типа нет пользователей', async () => {
    const { w } = await mountCmp({ selectId: 3 })
    expect(w.find('[data-testid="utype-blocking-notice"]').exists()).toBe(false)
    expect(w.find('.type-users__empty').exists()).toBe(true)
  })
})
