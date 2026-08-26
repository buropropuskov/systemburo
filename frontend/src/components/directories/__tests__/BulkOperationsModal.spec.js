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
