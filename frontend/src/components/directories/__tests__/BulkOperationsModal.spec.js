import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const apiMock = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('@/api/client', () => apiMock)

import BulkOperationsModal from '../BulkOperationsModal.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import SelectUnloadPlaces from '@/components/SelectUnloadPlaces.vue'
import SelectTables from '@/components/SelectTables.vue'
import ResponsibleUsersSection from '@/components/ResponsibleUsersSection.vue'

const PLACES = [
  { id: 1, name: 'Склад 1', status: 'active' },
  { id: 2, name: 'Склад 2', status: 'active' },
]
const TABLES = [
  { table: { id: 10, display_name: 'Люди', table_type: 'people' } },
  { table: { id: 11, display_name: 'Машины', table_type: 'cars' } },
]
const USERS = [
  { id: 100, username: 'ivanov', last_name: 'Иванов', first_name: 'Иван', position: 'Инженер' },
  { id: 101, username: 'petrov', last_name: 'Петров', first_name: 'Пётр', position: '' },
]

function okJson(data) {
  return { ok: true, json: vi.fn().mockResolvedValue(data) }
}

const STUBS = { teleport: true }

async function mountShown(operation, { entityType = 'organization', selectedIds = [1, 2, 3] } = {}) {
  const w = mount(BulkOperationsModal, {
    props: { show: false, operation, entityType, selectedIds },
    global: { stubs: STUBS },
  })
  await w.setProps({ show: true })
  await flushPromises()
  return w
}

const applyBtn = w => w.find('[data-testid="bulk-op-apply"]')

describe('BulkOperationsModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiMock.apiRequest.mockImplementation((path) => {
      if (path === '/unload-places') return Promise.resolve(okJson(PLACES))
      if (path === '/system-tables') return Promise.resolve(okJson(TABLES))
      if (path === '/users/all') return Promise.resolve(okJson(USERS))
      return Promise.resolve(okJson([]))
    })
  })

  it('операция type: apply эмитит выбранный тип', async () => {
    const w = await mountShown('type')
    // тип не выбран -> Применить недоступно
    expect(applyBtn(w).attributes('disabled')).toBeDefined()

    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 'Арендатор')
    await flushPromises()
    expect(applyBtn(w).attributes('disabled')).toBeUndefined()

    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ type: 'Арендатор' }])
  })

  it('операция type: «снять тип» эмитит null', async () => {
    const w = await mountShown('type')
    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', '__none__')
    await flushPromises()
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ type: null }])
  })

  it('type: BaseDropdown идёт с teleport (меню не режется модалкой)', async () => {
    const w = await mountShown('type')
    expect(w.findComponent(BaseDropdown).props('teleport')).toBe(true)
  })

  it('операция unload-places: переиспользует SelectUnloadPlaces (selectionMode), apply с mode add', async () => {
    const w = await mountShown('unload-places')
    const places = w.findComponent(SelectUnloadPlaces)
    expect(places.exists()).toBe(true)
    expect(places.props('selectionMode')).toBe(true)

    places.vm.$emit('update:modelValue', [1])
    await flushPromises()
    await w.find('[data-testid="bulk-op-mode-add"]').trigger('click')
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ unloadPlaceIds: [1], mode: 'add' }])
  })

  it('операция tables: переиспользует SelectTables (selectionMode), apply replace по умолчанию', async () => {
    const w = await mountShown('tables')
    const tables = w.findComponent(SelectTables)
    expect(tables.exists()).toBe(true)
    expect(tables.props('selectionMode')).toBe(true)

    tables.vm.$emit('update:modelValue', [10])
    await flushPromises()
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ tableIds: [10], mode: 'replace' }])
  })

  it('операция users: ResponsibleUsersSection, согласование ИНДИВИДУАЛЬНО на каждого', async () => {
    const w = await mountShown('users')
    const resp = w.findComponent(ResponsibleUsersSection)
    expect(resp.exists()).toBe(true)
    expect(resp.props('selectionMode')).toBe(true)
    // ничего не выбрано -> недоступно
    expect(applyBtn(w).attributes('disabled')).toBeDefined()

    resp.vm.$emit('update:modelValue', [
      { username: 'ivanov', required_approval: true },
      { username: 'petrov', required_approval: false },
    ])
    await flushPromises()

    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([
      {
        users: [
          { username: 'ivanov', required_approval: true },
          { username: 'petrov', required_approval: false },
        ],
        mode: 'replace',
      },
    ])
  })

  it('users: префилл союзом уже назначенных с охватом (coverage «в N из M»)', async () => {
    apiMock.apiRequest.mockImplementation((path) => {
      if (path === '/users/all') return Promise.resolve(okJson(USERS))
      if (path === '/organizations/1/users') return Promise.resolve(okJson([
        { username: 'ivanov', required_approval: true },
        { username: 'petrov', required_approval: false },
      ]))
      if (path === '/organizations/2/users') return Promise.resolve(okJson([
        { username: 'ivanov', required_approval: true },
      ]))
      return Promise.resolve(okJson([]))
    })
    const w = await mountShown('users', { selectedIds: [1, 2] })
    const byName = Object.fromEntries(w.vm.responsibleUsers.map(u => [u.username, u]))
    expect(byName.ivanov.coverage).toBe(2)      // назначен в обеих
    expect(byName.ivanov.required_approval).toBe(true)
    expect(byName.petrov.coverage).toBe(1)      // только в одной
    expect(byName.petrov.required_approval).toBe(false)
    // применяется как обычный набор (охват не идёт в payload)
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0][0].users).toEqual([
      { username: 'ivanov', required_approval: true },
      { username: 'petrov', required_approval: false },
    ])
  })

  it('users: смешанное согласование у существующего -> mixedApproval, строже (true)', async () => {
    apiMock.apiRequest.mockImplementation((path) => {
      if (path === '/users/all') return Promise.resolve(okJson(USERS))
      if (path === '/organizations/1/users') return Promise.resolve(okJson([{ username: 'ivanov', required_approval: true }]))
      if (path === '/organizations/2/users') return Promise.resolve(okJson([{ username: 'ivanov', required_approval: false }]))
      return Promise.resolve(okJson([]))
    })
    const w = await mountShown('users', { selectedIds: [1, 2] })
    const iv = w.vm.responsibleUsers.find(u => u.username === 'ivanov')
    expect(iv.mixedApproval).toBe(true)
    expect(iv.required_approval).toBe(true)
  })

  it('users: одно открытие -> ОДИН фетч на сущность (нет двойного reset/префилла)', async () => {
    // родитель ставит operation и show синхронно в одном тике - раньше два watcher'а
    // (show + operation) давали двойной reset() -> 2xN фетчей
    const w = mount(BulkOperationsModal, {
      props: { show: false, operation: '', entityType: 'organization', selectedIds: [1, 2] },
      global: { stubs: STUBS },
    })
    await flushPromises()
    const cnt = () => apiMock.apiRequest.mock.calls.filter(c => c[0] === '/organizations/1/users').length
    const before = cnt()
    await w.setProps({ operation: 'users', show: true })
    await flushPromises()
    expect(cnt() - before).toBe(1)
  })

  it('users: бейдж охвата рендерится в DOM («в N из M» / «во всех»)', async () => {
    apiMock.apiRequest.mockImplementation((path) => {
      if (path === '/users/all') return Promise.resolve(okJson(USERS))
      if (path === '/organizations/1/users') return Promise.resolve(okJson([
        { username: 'ivanov', required_approval: true },
        { username: 'petrov', required_approval: false },
      ]))
      if (path === '/organizations/2/users') return Promise.resolve(okJson([
        { username: 'ivanov', required_approval: true },
      ]))
      return Promise.resolve(okJson([]))
    })
    const w = await mountShown('users', { selectedIds: [1, 2] })
    const badges = w.findAll('.coverage-badge').map(b => b.text())
    expect(badges).toContain('в 1 из 2')                       // petrov - в одной
    expect(badges.some(t => /во всех|в 2 из 2/.test(t))).toBe(true) // ivanov - в обеих
  })

  it('кнопка Отмена эмитит close', async () => {
    const w = await mountShown('type')
    await w.find('[data-testid="bulk-op-cancel"]').trigger('click')
    expect(w.emitted('close')).toBeTruthy()
  })

  it('entity-type company меняет формулировку счётчика', async () => {
    const w = await mountShown('type', { entityType: 'company' })
    expect(w.find('[data-testid="bulk-op-modal"]').text()).toContain('компаниям')
  })
})
